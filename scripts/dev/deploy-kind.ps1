[CmdletBinding()]
param(
    [string] $KindCluster = "open-im-local",
    [string] $KubeContext = "kind-open-im-local",
    [string] $Version = "",
    [string] $PostgresPassword = "",
    [string] $RedisPassword = "",
    [string] $ObjectStoreSecret = "",
    [string] $ImageRegistry = "ghcr.io/openware-io",
    [ValidateSet("auto", "ghcr", "local-build", "local-cache")]
    [string] $ImageSource = "auto",
    [string] $EvidenceDir = "",
    [string] $ImageTagsFile = "",
    [switch] $SkipBuild,
    [switch] $RollbackDrill
)

# Deploys GreenPass to an existing local kind cluster.
#
# Hard rules implemented here (see the 2026-10-30 architect review, M0-1..M0-6):
#   - -Version is required; there is no hidden stale default (M0-1).
#   - -ImageSource auto resolves local images first, then GHCR, then fails with
#     an actionable message instead of attempting a pull that cannot succeed.
#   - The four images are resolved to image IDs and recorded in plan.json;
#     the live deployment is checked for version drift before and after the
#     rollout (M0-2).
#   - Migration Job runs BEFORE the application manifests are applied (M0-4).
#   - GP_VERSION/GP_COMMIT/GP_BUILT_AT are rendered into the manifests instead
#     of being injected with `kubectl set env`, so the rollout happens once.
#   - Every native command is checked; smoke tests assert the served version,
#     commit and the DSH gateway readiness (M0-5).
#   - Failure keeps evidence under $EvidenceDir and restores the previous
#     images + build metadata env vars; migrations are forward-only (M0-6).
#   - -ImageTagsFile pins each of the four images independently
#     (server/web/migrate/dshGateway, optional per-image commit) while the
#     post-deploy checks assert the served version/commit.
#   - -RollbackDrill intentionally fails after the application rollout to
#     exercise the rollback path (local validation only; the previous
#     deployment is restored afterwards).
#   - This script never prunes local images. Opt-in pruning lives in
#     init-kind.ps1 (-PruneLocalImages) and is limited to the four GP repos.

$ErrorActionPreference = "Stop"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$appManifest = Join-Path $repositoryRoot "deploy/k8s/gp-app.yaml"
$migrationManifest = Join-Path $repositoryRoot "deploy/k8s/gp-migration.yaml"
$gatewayManifest = Join-Path $repositoryRoot "deploy/k8s/gp-dsh-gateway.yaml"
$namespace = "gp"
$rolloutTimeout = "300s"
$migrationTimeoutSeconds = 300
$smokeBaseUrl = "http://127.0.0.1:30080"
$smokeHost = "greenpass.localhost"
$buildEnvNames = @("GP_VERSION", "GP_COMMIT", "GP_BUILT_AT")
$deploymentContainers = [ordered]@{
    "gp-server"      = "server"
    "gp-worker"      = "worker"
    "gp-web"         = "web"
    "gp-dsh-gateway" = "dsh-gateway"
}

# ── helpers ────────────────────────────────────────────────────────────────
function Assert-LastExit([string] $Message) {
    if ($LASTEXITCODE -ne 0) { throw "$Message (exit code $LASTEXITCODE)" }
}

function Invoke-Native([string] $Label, [scriptblock] $Action) {
    & $Action
    Assert-LastExit $Label
}

function Write-Utf8NoBom([string] $Path, [string] $Content) {
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Content, $utf8)
}

function New-RandomLocalSecret {
    return [Convert]::ToHexString([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
}

function Get-KubernetesSecretValue([string] $SecretName, [string] $Key) {
    $encoded = & kubectl --context $KubeContext -n $namespace get secret $SecretName -o "jsonpath={.data.$Key}" 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $encoded) { return $null }
    return [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String([string]$encoded))
}

function Render-Manifest([string] $Path, [hashtable] $Tokens) {
    $content = Get-Content $Path -Raw
    # Replacement order matters: specific tokens first, then the boundary-guarded
    # COMMIT/BUILT_AT values so env names such as GP_COMMIT stay intact.
    $content = $content -creplace 'VERSION_NAME', $Tokens.VersionName
    $content = $content -creplace 'IMAGE_REGISTRY', $Tokens.ImageRegistry
    $content = $content -creplace 'IMAGE_PULL_POLICY', $Tokens.PullPolicy
    $content = $content -creplace 'SERVER_VERSION', $Tokens.ServerVersion
    $content = $content -creplace 'WEB_VERSION', $Tokens.WebVersion
    $content = $content -creplace 'MIGRATE_VERSION', $Tokens.MigrateVersion
    $content = $content -creplace 'DSH_VERSION', $Tokens.DshVersion
    $content = $content -creplace '(?<![A-Za-z_])COMMIT(?![A-Za-z_])', $Tokens.Commit
    $content = $content -creplace '(?<![A-Za-z_])BUILT_AT(?![A-Za-z_])', $Tokens.BuiltAt
    return $content
}

function Test-LocalImage([string] $Name) {
    & docker image inspect $Name *> $null
    return ($LASTEXITCODE -eq 0)
}

function Test-GhcrImage([string] $Name) {
    & docker manifest inspect $Name *> $null
    return ($LASTEXITCODE -eq 0)
}

function Get-DeploymentSnapshot([string[]] $Names) {
    $snapshot = @{}
    foreach ($name in $Names) {
        $raw = & kubectl --context $KubeContext -n $namespace get deployment $name -o json 2>$null
        if ($LASTEXITCODE -ne 0 -or -not $raw) { continue }
        $deploy = $raw | ConvertFrom-Json
        $container = $deploy.spec.template.spec.containers[0]
        $envs = @{}
        foreach ($entry in @($container.env)) {
            if ($null -ne $entry -and $entry.name -in $buildEnvNames -and $null -ne $entry.value) {
                $value = $entry.value
                # ConvertFrom-Json turns ISO timestamps into [datetime]; store ISO
                # again so a rollback writes exactly what the live pod had.
                if ($value -is [datetime]) { $value = $value.ToUniversalTime().ToString("o") }
                $envs[[string]$entry.name] = [string]$value
            }
        }
        $annotations = [ordered]@{}
        foreach ($property in $deploy.spec.template.metadata.annotations.PSObject.Properties) {
            $value = $property.Value
            if ($value -is [datetime]) { $value = $value.ToUniversalTime().ToString("o") }
            $annotations[$property.Name] = [string]$value
        }
        $snapshot[$name] = @{
            Image       = [string]$container.image
            Container   = [string]$container.name
            EnvVars     = $envs
            Annotations = $annotations
        }
    }
    return $snapshot
}

function Get-DeploymentImage([string] $Name) {
    $raw = & kubectl --context $KubeContext -n $namespace get deployment $Name -o json 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $raw) { return $null }
    $deploy = $raw | ConvertFrom-Json
    return [string]$deploy.spec.template.spec.containers[0].image
}

function Assert-DeploymentState([string] $Name, [string] $ExpectedImage) {
    $raw = & kubectl --context $KubeContext -n $namespace get deployment $Name -o json
    Assert-LastExit "Unable to read deployment $Name"
    $deploy = $raw | ConvertFrom-Json
    $desired = [int]$deploy.spec.replicas
    $updated = [int]$deploy.status.updatedReplicas
    $ready = [int]$deploy.status.readyReplicas
    $unavailable = [int]($deploy.status.unavailableReplicas | ForEach-Object { if ($null -eq $_) { 0 } else { $_ } })
    if ($updated -ne $desired) { throw "Deployment $Name reports $updated/$desired updated replicas after rollout." }
    if ($ready -ne $desired) { throw "Deployment $Name reports $ready/$desired ready replicas after rollout." }
    if ($unavailable -gt 0) { throw "Deployment $Name still reports $unavailable unavailable replicas after rollout." }
    $actualImage = [string]$deploy.spec.template.spec.containers[0].image
    if ($ExpectedImage -and $actualImage -ne $ExpectedImage) {
        throw "Deployment $Name runs image '$actualImage' but expected '$ExpectedImage'."
    }
    if ($Name -eq "gp-server" -or $Name -eq "gp-worker") {
        $container = $deploy.spec.template.spec.containers[0]
        $env = @{}
        foreach ($entry in @($container.env)) {
            if ($null -ne $entry -and $entry.name) { $env[[string]$entry.name] = [string]$entry.value }
        }
        if ($env["GP_VERSION"] -ne $appVersion) {
            throw "Deployment $Name has GP_VERSION='$($env["GP_VERSION"])' but expected '$appVersion'."
        }
        if ($env["GP_COMMIT"] -ne $serverCommit) {
            throw "Deployment $Name has GP_COMMIT='$($env["GP_COMMIT"])' but expected '$serverCommit'."
        }
        if (-not $env["GP_BUILT_AT"] -or $env["GP_BUILT_AT"] -eq "unknown") {
            throw "Deployment $Name is missing rendered GP_BUILT_AT."
        }
    }
    # Pods of a locked image must report a non-empty imageID; this is the runtime
    # counterpart of the imageId recorded in plan.json.
    $component = $deploymentContainers[$Name]
    $podsRaw = & kubectl --context $KubeContext -n $namespace get pods -l "app.kubernetes.io/component=$component" -o json
    Assert-LastExit "Unable to read pods for deployment $Name"
    $pods = ($podsRaw | ConvertFrom-Json).items
    if (-not $pods -or $pods.Count -eq 0) { throw "Deployment $Name has no pods after rollout." }
    $imageIds = @()
    foreach ($pod in $pods) {
        if ($pod.metadata.deletionTimestamp) { continue }
        foreach ($status in @($pod.status.containerStatuses)) {
            if ($status.imageID) { $imageIds += [string]$status.imageID }
        }
    }
    if ($imageIds.Count -eq 0) { throw "Deployment $Name pods report no imageID." }
    if (($imageIds | Select-Object -Unique).Count -ne 1) {
        throw "Deployment $Name pods are running multiple imageIDs: $((($imageIds | Select-Object -Unique) -join ', '))"
    }
}

function Invoke-WebSmoke([string] $Path, [string] $Method = "GET", [string] $Body = "", $Session = $null) {
    $parameters = @{
        Uri                = "$smokeBaseUrl$Path"
        Headers            = @{ Host = $smokeHost }
        UseBasicParsing    = $true
        TimeoutSec         = 20
        Method             = $Method
        MaximumRedirection = 0
    }
    if ($Body) {
        $parameters.ContentType = "application/json"
        $parameters.Body = $Body
    }
    if ($Session) { $parameters.WebSession = $Session }
    try {
        $response = Invoke-WebRequest @parameters
    } catch {
        throw "HTTP smoke failed for $Path : $($_.Exception.Message)"
    }
    if ([int]$response.StatusCode -ne 200) { throw "HTTP smoke failed for ${Path}: status $($response.StatusCode)." }
    return $response
}

function Invoke-SmokeTests([string] $ExpectedVersion, [string] $ExpectedCommit) {
    $null = Invoke-WebSmoke "/"
    $null = Invoke-WebSmoke "/api/healthz"
    $null = Invoke-WebSmoke "/api/readyz"
    $versionResponse = Invoke-WebSmoke "/api/version"
    $versionInfo = $versionResponse.Content | ConvertFrom-Json
    if ($versionInfo.version -ne $ExpectedVersion) {
        throw "Served version '$($versionInfo.version)' does not match expected '$ExpectedVersion'."
    }
    if ($versionInfo.commit -ne $ExpectedCommit) {
        throw "Served commit '$($versionInfo.commit)' does not match expected '$ExpectedCommit'."
    }
    if (-not $versionInfo.built_at -or $versionInfo.built_at -eq "unknown") {
        throw "Served build info has no built_at timestamp."
    }
    $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $null = Invoke-WebSmoke "/api/auth/login" -Method "Post" -Body '{"username":"zhangli","password":"GreenPass@123"}' -Session $session
    $meResponse = Invoke-WebSmoke "/api/auth/me" -Session $session
    if (-not $meResponse.Content) { throw "Authenticated smoke test returned an empty /api/auth/me response." }

    # The DSH gateway is ClusterIP-only; probe it from inside its own pod.
    $gatewayScript = "fetch('http://127.0.0.1:8090/readyz').then(r=>{if(!r.ok){console.error('gateway readyz status '+r.status);process.exit(1)};return r.text()}).then(t=>console.log(t)).catch(e=>{console.error('gateway readyz error '+e);process.exit(1)})"
    $gatewayOutput = & kubectl --context $KubeContext -n $namespace exec deploy/gp-dsh-gateway -c dsh-gateway -- node -e $gatewayScript 2>&1 | Out-String
    Assert-LastExit "DSH gateway readiness smoke failed: $($gatewayOutput.Trim())"
    return [ordered]@{
        web      = "200"
        healthz  = "200"
        readyz   = "200"
        version  = $versionInfo
        login    = "200"
        me       = "200"
        gateway  = $gatewayOutput.Trim()
    }
}

function Get-MigrationVersion {
    try {
        $pgUser = Get-KubernetesSecretValue "gp-postgres-auth" "username"
        $pgPass = Get-KubernetesSecretValue "gp-postgres-auth" "password"
        if (-not $pgUser -or -not $pgPass) { return "unknown" }
        $raw = & kubectl --context $KubeContext -n $namespace exec deploy/gp-postgres -c postgres -- env "PGPASSWORD=$pgPass" psql -U $pgUser -d gp -tAc "select version from gp.schema_migrations order by version desc limit 1" 2>$null | Out-String
        if ($LASTEXITCODE -ne 0) { return "unknown" }
        $value = $raw.Trim()
        if (-not $value) { return "unknown" }
        return $value
    } catch {
        return "unknown"
    }
}

function Collect-FailureEvidence([string] $Reason) {
    $failureDir = Join-Path $EvidenceDir "failure"
    New-Item -ItemType Directory -Force -Path $failureDir | Out-Null
    Write-Utf8NoBom (Join-Path $failureDir "reason.txt") $Reason
    try {
        & kubectl --context $KubeContext -n $namespace get deploy,pods -o wide 2>&1 | Out-File (Join-Path $failureDir "deployments-pods.txt") -Encoding utf8
    } catch { }
    try {
        & kubectl --context $KubeContext -n $namespace get events --sort-by=.lastTimestamp 2>&1 | Out-File (Join-Path $failureDir "events.txt") -Encoding utf8
    } catch { }
    foreach ($name in $deploymentContainers.Keys) {
        try {
            & kubectl --context $KubeContext -n $namespace describe deployment $name 2>&1 | Out-File (Join-Path $failureDir "describe-$name.txt") -Encoding utf8
        } catch { }
        try {
            & kubectl --context $KubeContext -n $namespace logs "deployment/$name" --all-containers=true --tail=200 2>&1 | Out-File (Join-Path $failureDir "logs-$name.txt") -Encoding utf8
        } catch { }
    }
    try {
        & kubectl --context $KubeContext -n $namespace get jobs -l app.kubernetes.io/component=migration -o yaml 2>&1 | Out-File (Join-Path $failureDir "migration-jobs.yaml") -Encoding utf8
    } catch { }
    try {
        & kubectl --context $KubeContext -n $namespace logs "job/$migrationJobName" --tail=200 2>&1 | Out-File (Join-Path $failureDir "migration-job-logs.txt") -Encoding utf8
    } catch { }
}

function Restore-PreviousDeployment([hashtable] $Snapshot) {
    $restoreErrors = @()
    foreach ($name in $Snapshot.Keys) {
        $entry = $Snapshot[$name]
        if (-not $entry.Image) { continue }
        Write-Warning "Rolling back $name to previous image $($entry.Image)"
        & kubectl --context $KubeContext -n $namespace set image "deployment/$name" "$($entry.Container)=$($entry.Image)" | Out-Null
        if ($LASTEXITCODE -ne 0) { $restoreErrors += "set image $name"; continue }
        $envArguments = @()
        foreach ($envName in $buildEnvNames) {
            if ($entry.EnvVars.ContainsKey($envName)) {
                $envArguments += "$envName=$($entry.EnvVars[$envName])"
            } else {
                $envArguments += "$envName-"
            }
        }
        & kubectl --context $KubeContext -n $namespace set env "deployment/$name" @envArguments | Out-Null
        if ($LASTEXITCODE -ne 0) { $restoreErrors += "set env $name" }
    }
    foreach ($name in $Snapshot.Keys) {
        & kubectl --context $KubeContext -n $namespace rollout status "deployment/$name" --timeout=$rolloutTimeout | Out-Null
        if ($LASTEXITCODE -ne 0) { $restoreErrors += "rollout $name" }
    }
    if ($restoreErrors.Count -gt 0) {
        Write-Warning "Rollback finished with issues: $($restoreErrors -join ', '). Check evidence directory $EvidenceDir."
    } else {
        Write-Warning "Previous GreenPass deployment restored (images + build metadata env)."
    }
    Write-Warning "Database migrations are forward-only; the schema was NOT rolled back."
}

# ── parameter validation (M0-1) ────────────────────────────────────────────
if (-not $Version) {
    throw "Version is required. Example: pwsh -NoProfile -File scripts/dev/init-kind.ps1 -Version 0.1.213-SNAPSHOT. See README.md section '本地 kind 初始化与发布'."
}
if ($Version -notmatch '^\d+\.\d+\.\d+-SNAPSHOT$') {
    throw "Development releases must use X.Y.Z-SNAPSHOT without a v prefix (got '$Version')."
}
if ($ImageRegistry -notmatch '^[a-z0-9][a-z0-9.-]*(?::[0-9]+)?(?:/[a-z0-9._-]+)*$') {
    throw "ImageRegistry must be a registry/repository prefix without a trailing slash."
}
if ($SkipBuild -and $ImageSource -eq "ghcr") {
    # Backwards-compatible alias for callers that already have images locally.
    $ImageSource = "local-cache"
}

foreach ($command in @("docker", "kind", "kubectl", "git")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Required command not found: $command" }
}

$commit = (& git rev-parse HEAD | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or -not $commit) { throw "Unable to resolve the current git commit." }
$startedAt = (Get-Date).ToString("o")

# ── per-image tag/commit lock (M0-2, optional) ─────────────────────────────
$imageTags = @{
    server           = $Version
    web              = $Version
    migrate          = $Version
    dshGateway       = $Version
    serverCommit     = $commit
    webCommit        = ""
    migrateCommit    = ""
    dshGatewayCommit = ""
}
$imageTagsFileInfo = $null
if ($ImageTagsFile) {
    $tagsPath = $ImageTagsFile
    if (-not [System.IO.Path]::IsPathRooted($tagsPath)) { $tagsPath = Join-Path $repositoryRoot $tagsPath }
    if (-not (Test-Path $tagsPath)) { throw "ImageTagsFile not found: $tagsPath" }
    try { $tagData = Get-Content $tagsPath -Raw | ConvertFrom-Json } catch { throw "ImageTagsFile is not valid JSON: $($_.Exception.Message)" }
    $imageTagsFileInfo = [ordered]@{
        path   = $tagsPath
        sha256 = (Get-FileHash $tagsPath -Algorithm SHA256).Hash
    }
    foreach ($key in @("server", "web", "migrate", "dshGateway")) {
        $property = $tagData.PSObject.Properties[$key]
        if ($null -eq $property -or $null -eq $property.Value) { continue }
        $value = $property.Value
        if ($value -is [string]) {
            $imageTags[$key] = $value
        } else {
            if ($value.tag) { $imageTags[$key] = [string]$value.tag }
            if ($value.commit) { $imageTags["$($key)Commit"] = [string]$value.commit }
        }
    }
    foreach ($commitKey in @("serverCommit", "webCommit", "migrateCommit", "dshGatewayCommit")) {
        $property = $tagData.PSObject.Properties[$commitKey]
        if ($null -ne $property -and $null -ne $property.Value) { $imageTags[$commitKey] = [string]$property.Value }
    }
    foreach ($key in @("server", "web", "migrate", "dshGateway")) {
        $tag = [string]$imageTags[$key]
        if ($tag -notmatch '^\d+\.\d+\.\d+-SNAPSHOT$') { throw "ImageTagsFile.$key must be X.Y.Z-SNAPSHOT (got '$tag')." }
    }
}
$appVersion = [string]$imageTags.server
$serverCommit = [string]$imageTags.serverCommit
if (-not $serverCommit) { $serverCommit = $commit }
$webVersion = [string]$imageTags.web
$migrateVersion = [string]$imageTags.migrate
$dshVersion = [string]$imageTags.dshGateway

if (-not $EvidenceDir) {
    $EvidenceDir = Join-Path $repositoryRoot (Join-Path ".outputs/deploy" ("{0}-{1}" -f $Version, (Get-Date -Format "yyyyMMdd-HHmmss")))
} elseif (-not [System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidenceDir = Join-Path $repositoryRoot $EvidenceDir
}
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $EvidenceDir "rendered") | Out-Null
$planPath = Join-Path $EvidenceDir "plan.json"
$summaryPath = Join-Path $EvidenceDir "summary.json"
Write-Host "Evidence directory: $EvidenceDir"

# Keep the full deploy output (warnings, crictl checks, rollback steps) in the
# evidence directory so the run is auditable even when the console is lost.
$transcriptStarted = $false
try {
    Start-Transcript -Path (Join-Path $EvidenceDir "deploy.log") -Force | Out-Null
    $transcriptStarted = $true
} catch {
    Write-Warning "Unable to start the deploy transcript: $($_.Exception.Message)"
}

$images = @(
    @{ Name = "$ImageRegistry/open-green-pass:$($imageTags.server)"; File = "Dockerfile"; Component = "server/worker"; Commit = $serverCommit },
    @{ Name = "$ImageRegistry/open-green-pass-web:$webVersion"; File = "Dockerfile.web"; Component = "web"; Commit = [string]$imageTags.webCommit },
    @{ Name = "$ImageRegistry/open-green-pass-migrate:$migrateVersion"; File = "Dockerfile.migrate"; Component = "migrate"; Commit = [string]$imageTags.migrateCommit },
    @{ Name = "$ImageRegistry/open-green-pass-dsh-gateway:$dshVersion"; File = "Dockerfile.dsh-gateway"; Component = "dsh-gateway"; Commit = [string]$imageTags.dshGatewayCommit }
)
$expectedImages = [ordered]@{
    "gp-server"      = "$ImageRegistry/open-green-pass:$($imageTags.server)"
    "gp-worker"      = "$ImageRegistry/open-green-pass:$($imageTags.server)"
    "gp-web"         = "$ImageRegistry/open-green-pass-web:$webVersion"
    "gp-dsh-gateway" = "$ImageRegistry/open-green-pass-dsh-gateway:$dshVersion"
}

Push-Location $repositoryRoot
$applied = $false
$migrationVersion = "unknown"
try {
    if (@(& kubectl config get-contexts $KubeContext --no-headers 2>$null).Count -eq 0) { throw "Kubernetes context is not available: $KubeContext" }
    if (-not ((& kind get clusters) -contains $KindCluster)) { throw "Kind cluster is not available: $KindCluster" }
    foreach ($manifestPath in @($appManifest, $migrationManifest, $gatewayManifest)) {
        if (-not (Test-Path $manifestPath)) { throw "Required Kubernetes manifest is missing: $manifestPath" }
    }

    # Read-only snapshot of the IM test target. The deployment later proves that
    # GP did not touch any open-im-local workload. A present-but-unreadable
    # namespace must never silently degrade to "check not applicable" (N3).
    $imNamespaceRef = & kubectl --context $KubeContext get namespace open-im-local --ignore-not-found -o name 2>$null
    Assert-LastExit "Unable to query the open-im-local namespace."
    $imNamespacePresent = [bool]$imNamespaceRef
    $imSnapshot = @{}
    if ($imNamespacePresent) {
        $imBefore = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
        Assert-LastExit "Unable to snapshot open-im-local deployments."
        foreach ($deployment in $imBefore.items) {
            $imSnapshot[$deployment.metadata.name] = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
        }
        if ($imSnapshot.Count -eq 0) { throw "open-im-local namespace exists but reports no deployments; refusing to skip the IM isolation check." }
        Write-Host "Captured open-im-local snapshot ($($imSnapshot.Count) deployments)."
    }

    # ── resolve image source (M0-1) ────────────────────────────────────────
    $resolvedImageSource = $ImageSource
    if ($ImageSource -eq "auto") {
        $missingLocal = @($images | Where-Object { -not (Test-LocalImage $_.Name) })
        if ($missingLocal.Count -eq 0) {
            $resolvedImageSource = "local-cache"
            Write-Host "ImageSource auto: all 4 images found locally -> local-cache"
        } else {
            $missingGhcr = @($images | Where-Object { -not (Test-GhcrImage $_.Name) })
            if ($missingGhcr.Count -eq 0) {
                $resolvedImageSource = "ghcr"
                Write-Host "ImageSource auto: all 4 images found in GHCR -> ghcr"
            } else {
                $missingDetail = @()
                foreach ($image in $images) {
                    if (-not (Test-LocalImage $image.Name)) { $missingDetail += "  local: $($image.Name) (missing)" }
                    if (-not (Test-GhcrImage $image.Name)) { $missingDetail += "  ghcr:  $($image.Name) (manifest unknown)" }
                }
                throw "No usable image source for version $Version.`n$($missingDetail -join "`n")`nBuild locally: add -ImageSource local-build, or publish the images first."
            }
        }
    }

    # ── N1 guard: auto-selected local images may be stale ─────────────────
    if ($resolvedImageSource -eq "local-cache" -and $ImageSource -eq "auto") {
        $unverifiable = @()
        foreach ($image in $images) {
            $labelsJson = (& docker image inspect --format '{{json .Config.Labels}}' $image.Name 2>$null | Out-String).Trim()
            Assert-LastExit "Unable to read labels for $($image.Name)."
            $labels = if ($labelsJson -and $labelsJson -ne "null") { $labelsJson | ConvertFrom-Json } else { $null }
            $labelCommit = if ($labels -and $labels.'gp.build.commit') { [string]$labels.'gp.build.commit' } else { "" }
            $image.BuildCommit = $labelCommit
            if (-not $labelCommit) {
                $unverifiable += $image.Name
            } elseif ($image.Commit -and $labelCommit -ne $image.Commit) {
                throw "Local image $($image.Name) was built from commit '$labelCommit' but this deployment expects '$($image.Commit)'. Rebuild with -ImageSource local-build, or pin the matching commit for this image in -ImageTagsFile (serverCommit/webCommit/migrateCommit/dshGatewayCommit)."
            } elseif (-not $image.Commit) {
                Write-Host "Recorded local image $($image.Name) build commit: $labelCommit"
            }
        }
        if ($unverifiable.Count -gt 0) {
            Write-Warning "Auto selected local-cache; these images carry no gp.build.commit label, so staleness cannot be verified: $($unverifiable -join ', '). Use -ImageSource local-build after changing source."
        }
        $dirtyWorktree = (& git status --porcelain | Out-String).Trim()
        if ($dirtyWorktree) {
            Write-Warning "Worktree is dirty; local-cache images may not reflect the current source. Use -ImageSource local-build for a reproducible deployment."
        }
    }

    # ── snapshot live state before any changes (M0-2, M0-6) ───────────────
    $previousDeployments = Get-DeploymentSnapshot -Names $deploymentContainers.Keys
    $drift = @()
    foreach ($name in $deploymentContainers.Keys) {
        $actual = Get-DeploymentImage $name
        if ($actual -and $actual -ne $expectedImages[$name]) {
            $drift += "$name : $actual -> $($expectedImages[$name])"
        }
    }
    if ($drift.Count -gt 0) {
        Write-Warning "Version drift detected before deployment:"
        foreach ($line in $drift) { Write-Warning "  $line" }
    }

    # ── acquire, verify and load images ────────────────────────────────────
    $kindNodes = @(& kind get nodes --name $KindCluster)
    if ($LASTEXITCODE -ne 0 -or $kindNodes.Count -eq 0) { throw "Unable to discover nodes in kind cluster $KindCluster." }
    if ($resolvedImageSource -eq "local-build") {
        foreach ($image in $images) {
            if ($image.Commit -and $image.Commit -ne $commit) {
                throw "Cannot build $($image.Name) from commit '$($image.Commit)' because the worktree is at '$commit'. Check out that commit or use -ImageSource local-cache."
            }
        }
    }
    foreach ($image in $images) {
        if ($resolvedImageSource -eq "ghcr") {
            Invoke-Native "GHCR image pull failed for $($image.Name) (run 'docker login ghcr.io' if the repository is private)" { docker pull $image.Name }
        } elseif ($resolvedImageSource -eq "local-build") {
            if ($image.File -eq "Dockerfile.dsh-gateway") {
                $oldCGO = $env:CGO_ENABLED; $oldGOOS = $env:GOOS; $oldGOARCH = $env:GOARCH
                $env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
                try {
                    Invoke-Native "DSH gateway Go binary build failed." { go build -trimpath -ldflags "-s -w" -o dsh-gateway-linux ./cmd/dsh-gateway }
                } finally {
                    $env:CGO_ENABLED = $oldCGO; $env:GOOS = $oldGOOS; $env:GOARCH = $oldGOARCH
                }
            }
            $buildCommit = if ($image.Commit) { $image.Commit } else { $commit }
            Invoke-Native "Image build failed for $($image.Name)" {
                docker build --pull=false --label "gp.build.commit=$buildCommit" --label "gp.build.version=$($image.Name.Split(':')[-1])" --label "gp.build.source=local-build" --file $image.File --tag $image.Name .
            }
        } else {
            Write-Host "Using existing local image: $($image.Name)"
        }
        Invoke-Native "Required image is missing: $($image.Name)" { docker image inspect $image.Name | Out-Null }
        $image.ImageId = ((& docker image inspect --format '{{.Id}}' $image.Name) | Out-String).Trim()
        Assert-LastExit "Unable to inspect image ID for $($image.Name)"
        if (-not $image.ImageId) { throw "Docker returned an empty image ID for $($image.Name)." }
        $image.Digest = (((& docker image inspect --format '{{range .RepoDigests}}{{.}}{{end}}' $image.Name 2>$null) | Out-String).Trim())
        $imageLabelsJson = (& docker image inspect --format '{{json .Config.Labels}}' $image.Name 2>$null | Out-String).Trim()
        $image.BuildLabels = if ($imageLabelsJson -and $imageLabelsJson -ne "null") { $imageLabelsJson | ConvertFrom-Json } else { $null }
        Invoke-Native "Image load failed for $($image.Name)" { kind load docker-image --name $KindCluster --nodes ($kindNodes -join ",") $image.Name }
        foreach ($node in $kindNodes) {
            & docker exec $node crictl inspecti $image.Name *> $null
            Assert-LastExit "Image $($image.Name) is not present in kind node $node after load."
        }
    }

    # ── dry-run manifests and write deployment plan ────────────────────────
    $builtAt = [DateTime]::UtcNow.ToString("o")
    $versionName = $Version.ToLowerInvariant().Replace('.', '-').Replace('_', '-')
    $migrationJobName = "gp-migrate-$versionName"
    $tokens = @{
        VersionName    = $versionName
        ImageRegistry  = $ImageRegistry
        PullPolicy     = "IfNotPresent"
        ServerVersion  = $imageTags.server
        WebVersion     = $webVersion
        MigrateVersion = $migrateVersion
        DshVersion     = $dshVersion
        Commit         = $serverCommit
        BuiltAt        = $builtAt
    }
    $renderedApp = Join-Path $EvidenceDir "rendered/gp-app.yaml"
    $renderedMigration = Join-Path $EvidenceDir "rendered/gp-migration.yaml"
    $renderedGateway = Join-Path $EvidenceDir "rendered/gp-dsh-gateway.yaml"
    Write-Utf8NoBom $renderedApp (Render-Manifest -Path $appManifest -Tokens $tokens)
    Write-Utf8NoBom $renderedMigration (Render-Manifest -Path $migrationManifest -Tokens $tokens)
    Write-Utf8NoBom $renderedGateway (Render-Manifest -Path $gatewayManifest -Tokens $tokens)

    Invoke-Native "Server-side GP application manifest validation failed." { kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts --dry-run=server -f $renderedApp | Out-Null }
    Invoke-Native "Server-side migration manifest validation failed." { kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts --dry-run=server -f $renderedMigration | Out-Null }
    Invoke-Native "Server-side DSH gateway manifest validation failed." { kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts --dry-run=server -f $renderedGateway | Out-Null }

    $plan = [ordered]@{
        generatedAt        = (Get-Date).ToString("o")
        version            = $Version
        appVersion         = $appVersion
        commit             = $commit
        serverCommit       = $serverCommit
        requestedImageSource = $ImageSource
        imageSource        = $resolvedImageSource
        imageTags          = [ordered]@{ server = $imageTags.server; web = $webVersion; migrate = $migrateVersion; dshGateway = $dshVersion; serverCommit = $serverCommit }
        imageTagsFile      = $imageTagsFileInfo
        rollbackDrill      = [bool]$RollbackDrill
        imageRegistry      = $ImageRegistry
        kindCluster        = $KindCluster
        kubeContext        = $KubeContext
        namespace          = $namespace
        migrationJob       = $migrationJobName
        evidenceDir        = $EvidenceDir
        images             = @($images | ForEach-Object { [ordered]@{ name = $_.Name; component = $_.Component; imageId = $_.ImageId; digest = $_.Digest; commit = $_.Commit; labels = $_.BuildLabels } })
        previousDeployments = $previousDeployments
        versionDrift       = @($drift)
    }
    Write-Utf8NoBom $planPath (($plan | ConvertTo-Json -Depth 10) + "`n")

    # Namespaces are applied only after the image source is resolved, so a
    # preflight failure (for example an unknown image) does not mutate the cluster.
    Invoke-Native "Applying deploy/k8s/namespaces.yaml failed." { kubectl --context $KubeContext apply -f deploy/k8s/namespaces.yaml | Out-Null }

    # ── secrets (idempotent; never overwrite existing credentials) ─────────
    $pgUser = Get-KubernetesSecretValue "gp-postgres-auth" "username"
    $pgPassword = Get-KubernetesSecretValue "gp-postgres-auth" "password"
    if (-not $pgUser -and -not $pgPassword) {
        if (-not $PostgresPassword) { $PostgresPassword = New-RandomLocalSecret }
        Invoke-Native "Unable to create GP PostgreSQL credentials." { kubectl --context $KubeContext -n $namespace create secret generic gp-postgres-auth --from-literal=username=gp --from-literal=password=$PostgresPassword | Out-Null }
        $pgUser = "gp"
        $pgPassword = $PostgresPassword
    } elseif (-not $pgUser -or -not $pgPassword) {
        throw "Existing Secret gp-postgres-auth is missing username or password; refusing to replace credentials."
    }

    $redisPasswordActual = Get-KubernetesSecretValue "gp-redis-auth" "password"
    if (-not $redisPasswordActual) {
        if (-not $RedisPassword) { $RedisPassword = New-RandomLocalSecret }
        Invoke-Native "Unable to create GP Redis credentials." { kubectl --context $KubeContext -n $namespace create secret generic gp-redis-auth --from-literal=password=$RedisPassword | Out-Null }
        $redisPasswordActual = $RedisPassword
    }

    $existingAppSecret = & kubectl --context $KubeContext -n $namespace get secret gp-app-config -o name 2>$null
    if (-not $existingAppSecret) {
        if (-not $ObjectStoreSecret) { $ObjectStoreSecret = New-RandomLocalSecret }
        $escapedUser = [uri]::EscapeDataString($pgUser)
        $escapedPassword = [uri]::EscapeDataString($pgPassword)
        $dsn = "postgres://${escapedUser}:${escapedPassword}@gp-postgres.gp.svc.cluster.local:5432/gp?sslmode=disable"
        Invoke-Native "Unable to create the GP application configuration Secret." {
            kubectl --context $KubeContext -n $namespace create secret generic gp-app-config --from-literal=GP_DB_DSN=$dsn --from-literal=GP_REDIS_ADDR=gp-redis.gp.svc.cluster.local:6379 --from-literal=GP_REDIS_PASSWORD=$redisPasswordActual --from-literal=GP_MINIO_ENDPOINT=gp-seaweedfs.gp.svc.cluster.local:9000 --from-literal=GP_MINIO_KEY=gpminio --from-literal=GP_MINIO_SECRET=$ObjectStoreSecret | Out-Null
        }
    }

    $existingS3Auth = & kubectl --context $KubeContext -n $namespace get secret gp-s3-auth -o name 2>$null
    if (-not $existingS3Auth) {
        $minioKey = Get-KubernetesSecretValue "gp-app-config" "GP_MINIO_KEY"
        $minioSecret = Get-KubernetesSecretValue "gp-app-config" "GP_MINIO_SECRET"
        if (-not $minioKey -or -not $minioSecret) { throw "GP object storage credentials are missing." }
        $s3Config = @{
            signingKey = [guid]::NewGuid().ToString("N")
            identities = @(@{
                    name        = "gpminio"
                    credentials = @(@{ accessKey = $minioKey; secretKey = $minioSecret })
                    actions     = @("Admin", "Read", "Write")
                })
        } | ConvertTo-Json -Depth 6 -Compress
        $s3ConfigFile = Join-Path $EvidenceDir "s3-auth.json"
        Write-Utf8NoBom $s3ConfigFile $s3Config
        Invoke-Native "Unable to create SeaweedFS S3 auth Secret." {
            kubectl --context $KubeContext -n $namespace create secret generic gp-s3-auth --from-file=s3.json=$s3ConfigFile --dry-run=client -o yaml | kubectl --context $KubeContext -n $namespace apply -f - | Out-Null
        }
        Remove-Item -LiteralPath $s3ConfigFile -Force -ErrorAction SilentlyContinue
    }

    $dshToken = Get-KubernetesSecretValue "gp-dsh-auth" "token"
    if (-not $dshToken) {
        $dshToken = New-RandomLocalSecret
        Invoke-Native "Unable to create the DSH gateway internal token Secret." { kubectl --context $KubeContext -n $namespace create secret generic gp-dsh-auth --from-literal=token=$dshToken | Out-Null }
    }

    # ── base dependencies ──────────────────────────────────────────────────
    foreach ($manifest in @("gp-postgres.yaml", "gp-redis.yaml", "seaweedfs.yaml")) {
        Invoke-Native "Applying $manifest failed." { kubectl --context $KubeContext -n $namespace apply -f (Join-Path "deploy/k8s" $manifest) | Out-Null }
    }
    foreach ($name in @("gp-postgres", "gp-redis", "gp-seaweedfs")) {
        Invoke-Native "Base dependency $name did not become ready." { kubectl --context $KubeContext -n $namespace rollout status "deployment/$name" --timeout=$rolloutTimeout | Out-Null }
    }
    Invoke-Native "Applying gp-temporal.yaml failed." { kubectl --context $KubeContext -n $namespace apply -f deploy/k8s/gp-temporal.yaml | Out-Null }
    Invoke-Native "Applying gp-runner-rbac.yaml failed." { kubectl --context $KubeContext apply -f deploy/k8s/gp-runner-rbac.yaml | Out-Null }
    Invoke-Native "Applying gp-runner-policy.yaml failed." { kubectl --context $KubeContext apply -f deploy/k8s/gp-runner-policy.yaml | Out-Null }
    Invoke-Native "Temporal did not become ready." { kubectl --context $KubeContext -n $namespace rollout status deployment/gp-temporal --timeout="600s" | Out-Null }

    # ── migration BEFORE application (M0-4) ────────────────────────────────
    Invoke-Native "Unable to remove previous migration Jobs." { kubectl --context $KubeContext -n $namespace delete job -l app.kubernetes.io/component=migration --ignore-not-found | Out-Null }
    Invoke-Native "Migration Job creation failed." { kubectl --context $KubeContext apply -f $renderedMigration | Out-Null }
    & kubectl --context $KubeContext -n $namespace wait --for=condition=complete "job/$migrationJobName" --timeout="$($migrationTimeoutSeconds)s" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        $migrationLogs = (& kubectl --context $KubeContext -n $namespace logs "job/$migrationJobName" --tail=100 2>&1 | Out-String).Trim()
        throw "Migration Job $migrationJobName failed before application rollout: $migrationLogs"
    }
    $migrationVersion = Get-MigrationVersion
    try {
        & kubectl --context $KubeContext -n $namespace logs "job/$migrationJobName" --tail=200 2>&1 | Out-File (Join-Path $EvidenceDir "migration-job.log") -Encoding utf8
    } catch {
        Write-Warning "Unable to capture migration Job logs: $($_.Exception.Message)"
    }
    Write-Host "Migration complete (schema version: $migrationVersion)."

    # ── application rollout (single rollout; GP_* rendered in manifest) ────
    Invoke-Native "GP Kubernetes manifest apply failed." { kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts -f $renderedApp | Out-Null }
    $applied = $true
    foreach ($name in @("gp-server", "gp-worker", "gp-web")) {
        Invoke-Native "Deployment $name did not complete its rollout." { kubectl --context $KubeContext -n $namespace rollout status "deployment/$name" --timeout=$rolloutTimeout | Out-Null }
    }
    Invoke-Native "DSH gateway manifest apply failed." { kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts -f $renderedGateway | Out-Null }
    Invoke-Native "Deployment gp-dsh-gateway did not complete its rollout." { kubectl --context $KubeContext -n $namespace rollout status "deployment/gp-dsh-gateway" --timeout=$rolloutTimeout | Out-Null }

    # ── post-deploy assertions (M0-2, M0-5) ────────────────────────────────
    foreach ($name in $deploymentContainers.Keys) {
        Assert-DeploymentState -Name $name -ExpectedImage $expectedImages[$name]
    }
    if ($RollbackDrill) {
        throw "Rollback drill requested: simulating a post-apply validation failure to exercise the rollback path."
    }
    $smoke = Invoke-SmokeTests -ExpectedVersion $appVersion -ExpectedCommit $serverCommit
    Write-Host "Smoke tests passed (version=$appVersion commit=$serverCommit gateway=ready)."

    # ── IM isolation check ─────────────────────────────────────────────────
    $imChecked = $false
    if ($imNamespacePresent) {
        $imAfter = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
        Assert-LastExit "Unable to re-read open-im-local deployments."
        if ($imAfter.items.Count -ne $imSnapshot.Count) { throw "IM deployment count changed during GreenPass deployment." }
        foreach ($deployment in $imAfter.items) {
            $value = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
            if (-not $imSnapshot.ContainsKey($deployment.metadata.name) -or $imSnapshot[$deployment.metadata.name] -ne $value) {
                throw "IM workload changed during GreenPass deployment: $($deployment.metadata.name)"
            }
        }
        $imChecked = $true
        Write-Host "open-im-local deployments unchanged ($($imSnapshot.Count) deployments)."
    } else {
        Write-Host "open-im-local namespace not found; IM workload isolation check not applicable."
    }

    # ── evidence summary ───────────────────────────────────────────────────
    $finishedAt = (Get-Date).ToString("o")
    $duration = ([DateTime]::Parse($finishedAt) - [DateTime]::Parse($startedAt)).TotalSeconds
    $summary = [ordered]@{
        version          = $Version
        appVersion       = $appVersion
        commit           = $commit
        serverCommit     = $serverCommit
        requestedImageSource = $ImageSource
        imageSource      = $resolvedImageSource
        imageTags        = [ordered]@{ server = $imageTags.server; web = $webVersion; migrate = $migrateVersion; dshGateway = $dshVersion }
        imageTagsFile    = $imageTagsFileInfo
        rollbackDrill    = [bool]$RollbackDrill
        evidenceDir      = $EvidenceDir
        startedAt        = $startedAt
        finishedAt       = $finishedAt
        durationSeconds  = [Math]::Round($duration, 1)
        migrationVersion = $migrationVersion
        images           = @($images | ForEach-Object { [ordered]@{ name = $_.Name; imageId = $_.ImageId; digest = $_.Digest; commit = $_.Commit; labels = $_.BuildLabels } })
        deployments      = [ordered]@{}
        smoke            = $smoke
        imIsolation      = [ordered]@{ checked = $imChecked; namespacePresent = $imNamespacePresent; deployments = $imSnapshot.Count }
    }
    foreach ($name in $deploymentContainers.Keys) {
        $raw = & kubectl --context $KubeContext -n $namespace get deployment $name -o json
        Assert-LastExit "Unable to read deployment $name for the evidence summary."
        $deploy = $raw | ConvertFrom-Json
        $summary.deployments[$name] = [ordered]@{
            image     = [string]$deploy.spec.template.spec.containers[0].image
            replicas  = [int]$deploy.spec.replicas
            ready     = [int]$deploy.status.readyReplicas
            updated   = [int]$deploy.status.updatedReplicas
        }
    }
    Write-Utf8NoBom $summaryPath (($summary | ConvertTo-Json -Depth 10) + "`n")

    Write-Host "GreenPass kind initialization succeeded: version=$Version commit=$commit deployment=kubectl http=ready login=ready"
    Write-Host "Evidence: $summaryPath"
} catch {
    $reason = $_.Exception.Message
    if ($EvidenceDir -and (Test-Path $EvidenceDir)) {
        try { Collect-FailureEvidence -Reason $reason } catch { Write-Warning "Unable to collect all failure evidence: $($_.Exception.Message)" }
    }
    if ($applied) {
        Write-Warning "Post-deployment validation failed; restoring the previous GP deployment."
        if ($migrationVersion -and $migrationVersion -ne "unknown") {
            Write-Warning "Database migrations were already applied up to schema version $migrationVersion; the schema is forward-only and will NOT be rolled back."
        }
        try { Restore-PreviousDeployment -Snapshot $previousDeployments } catch { Write-Warning "Rollback failed: $($_.Exception.Message)" }
    } else {
        Write-Warning "Application manifests were not applied; previous GP deployment is untouched."
    }
    throw
} finally {
    if ($transcriptStarted) {
        try { Stop-Transcript | Out-Null } catch { }
    }
    Pop-Location
}
