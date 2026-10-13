[CmdletBinding()]
param(
    [string] $KindCluster = "open-im-local",
    [string] $KubeContext = "kind-open-im-local",
    [string] $Version = "0.1.25-SNAPSHOT",
    [string] $PostgresPassword = "",
    [string] $RedisPassword = "",
    [string] $ObjectStoreSecret = "",
    [string] $ImageRegistry = "ghcr.io/openware-io",
    [ValidateSet("ghcr", "local-build", "local-cache")]
    [string] $ImageSource = "ghcr",
    [switch] $SkipBuild
)

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$appManifest = Join-Path $repositoryRoot "deploy/k8s/gp-app.yaml"
$migrationManifest = Join-Path $repositoryRoot "deploy/k8s/gp-migration.yaml"
$namespace = "gp"

if ($SkipBuild -and $ImageSource -eq "ghcr") {
    # Backwards-compatible alias for callers that already have images locally.
    $ImageSource = "local-cache"
}
if ($ImageRegistry -notmatch '^[a-z0-9][a-z0-9.-]*(?::[0-9]+)?(?:/[a-z0-9._-]+)*$') {
    throw "ImageRegistry must be a registry/repository prefix without a trailing slash."
}

if ($Version -notmatch '^\d+\.\d+\.\d+-SNAPSHOT$') {
    throw "Development releases must use X.Y.Z-SNAPSHOT without a v prefix."
}

foreach ($command in @("docker", "kind", "kubectl", "git")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Required command not found: $command" }
}
if ((& kubectl --context $KubeContext config current-context) -ne $KubeContext) { throw "Kubernetes context is not available: $KubeContext" }
if (-not ((& kind get clusters) -contains $KindCluster)) { throw "Kind cluster is not available: $KindCluster" }

$imSnapshot = @{}
if (& kubectl --context $KubeContext get namespace open-im-local --ignore-not-found -o name 2>$null) {
    $imBefore = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
    foreach ($deployment in $imBefore.items) {
        $imSnapshot[$deployment.metadata.name] = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
    }
}

$images = @(
    @{ Name = "$ImageRegistry/open-green-pass:$Version"; File = "Dockerfile" },
    @{ Name = "$ImageRegistry/open-green-pass-web:$Version"; File = "Dockerfile.web" },
    @{ Name = "$ImageRegistry/open-green-pass-migrate:$Version"; File = "Dockerfile.migrate" }
)

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

Push-Location $repositoryRoot
try {
    foreach ($manifestPath in @($appManifest, $migrationManifest)) {
        if (-not (Test-Path $manifestPath)) { throw "Required Kubernetes manifest is missing: $manifestPath" }
    }
    foreach ($image in $images) {
        if ($ImageSource -eq "ghcr") {
            & docker pull $image.Name
            if ($LASTEXITCODE -ne 0) { throw "GHCR image pull failed: $($image.Name). Authenticate with docker login if the repository is private." }
        } elseif ($ImageSource -eq "local-build") {
            & docker build --pull=false --file $image.File --tag $image.Name .
            if ($LASTEXITCODE -ne 0) { throw "Image build failed: $($image.Name)" }
        } else {
            Write-Host "Using existing local image: $($image.Name)"
        }
        & docker image inspect $image.Name | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Required image is missing: $($image.Name)" }
        $kindNodes = @(& kind get nodes --name $KindCluster)
        if ($LASTEXITCODE -ne 0 -or $kindNodes.Count -eq 0) { throw "Unable to discover nodes in kind cluster $KindCluster." }
        & kind load docker-image --name $KindCluster --nodes ($kindNodes -join ",") $image.Name
        if ($LASTEXITCODE -ne 0) { throw "Image load failed: $($image.Name)" }
    }

    & kubectl --context $KubeContext apply -f deploy/k8s/namespaces.yaml | Out-Null
    $renderedApp = Join-Path ([System.IO.Path]::GetTempPath()) "gp-app-$Version.yaml"
    $renderedMigration = Join-Path ([System.IO.Path]::GetTempPath()) "gp-migration-$Version.yaml"
    $versionName = $Version.ToLowerInvariant().Replace('.', '-').Replace('_', '-')
    $pullPolicy = if ($ImageSource -eq "local-cache" -or $ImageSource -eq "local-build") { "IfNotPresent" } else { "IfNotPresent" }
    Write-Utf8NoBom $renderedApp ((Get-Content $appManifest -Raw).Replace("VERSION", $Version).Replace("IMAGE_REGISTRY", $ImageRegistry).Replace("IMAGE_PULL_POLICY", $pullPolicy))
    Write-Utf8NoBom $renderedMigration ((Get-Content $migrationManifest -Raw).Replace("VERSION_NAME", $versionName).Replace("VERSION", $Version).Replace("IMAGE_REGISTRY", $ImageRegistry).Replace("IMAGE_PULL_POLICY", $pullPolicy))
    # Existing kind installations may have been created by the removed Helm chart.
    # Force ownership only for the GP manifest so the script becomes the sole
    # manager without touching resources outside the GP namespace.
    & kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts --dry-run=server -f $renderedApp | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Server-side GP manifest validation failed." }
    & kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts --dry-run=server -f $renderedMigration | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Server-side migration manifest validation failed." }
    $pgUser = Get-KubernetesSecretValue "gp-postgres-auth" "username"
    $pgPassword = Get-KubernetesSecretValue "gp-postgres-auth" "password"
    if (-not $pgUser -and -not $pgPassword) {
        if (-not $PostgresPassword) { $PostgresPassword = New-RandomLocalSecret }
        & kubectl --context $KubeContext -n $namespace create secret generic gp-postgres-auth --from-literal=username=gp --from-literal=password=$PostgresPassword | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Unable to create GP PostgreSQL credentials." }
        $pgUser = "gp"
        $pgPassword = $PostgresPassword
    } elseif (-not $pgUser -or -not $pgPassword) {
        throw "Existing Secret gp-postgres-auth is missing username or password; refusing to replace credentials."
    }

    $redisPasswordActual = Get-KubernetesSecretValue "gp-redis-auth" "password"
    if (-not $redisPasswordActual) {
        if (-not $RedisPassword) { $RedisPassword = New-RandomLocalSecret }
        & kubectl --context $KubeContext -n $namespace create secret generic gp-redis-auth --from-literal=password=$RedisPassword | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Unable to create GP Redis credentials." }
        $redisPasswordActual = $RedisPassword
    }

    $existingAppSecret = & kubectl --context $KubeContext -n $namespace get secret gp-app-config -o name 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $existingAppSecret) {
        if (-not $ObjectStoreSecret) { $ObjectStoreSecret = New-RandomLocalSecret }
        $escapedUser = [uri]::EscapeDataString($pgUser)
        $escapedPassword = [uri]::EscapeDataString($pgPassword)
        $dsn = "postgres://${escapedUser}:${escapedPassword}@gp-postgres.gp.svc.cluster.local:5432/gp?sslmode=disable"
        & kubectl --context $KubeContext -n $namespace create secret generic gp-app-config --from-literal=GP_DB_DSN=$dsn --from-literal=GP_REDIS_ADDR=gp-redis.gp.svc.cluster.local:6379 --from-literal=GP_REDIS_PASSWORD=$redisPasswordActual --from-literal=GP_MINIO_ENDPOINT=gp-seaweedfs.gp.svc.cluster.local:9000 --from-literal=GP_MINIO_KEY=gpminio --from-literal=GP_MINIO_SECRET=$ObjectStoreSecret | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Unable to create GP application configuration Secret." }
    }

    foreach ($manifest in @("gp-postgres.yaml", "gp-redis.yaml", "seaweedfs.yaml")) {
        & kubectl --context $KubeContext apply -f (Join-Path "deploy/k8s" $manifest) | Out-Null
    }
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-postgres --timeout=300s
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-redis --timeout=300s
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-seaweedfs --timeout=300s
    & kubectl --context $KubeContext apply -f deploy/k8s/gp-temporal.yaml | Out-Null
    & kubectl --context $KubeContext apply -f deploy/k8s/gp-runner-rbac.yaml | Out-Null
    & kubectl --context $KubeContext apply -f deploy/k8s/gp-runner-policy.yaml | Out-Null
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-temporal --timeout=600s

    $previousImages = @{}
    foreach ($deploymentName in @("gp-server", "gp-worker", "gp-web")) {
        $image = & kubectl --context $KubeContext -n $namespace get deployment $deploymentName -o jsonpath="{.spec.template.spec.containers[0].image}" 2>$null
        if ($LASTEXITCODE -eq 0 -and $image) { $previousImages[$deploymentName] = [string]$image }
    }
    try {
        & kubectl --context $KubeContext apply --server-side --field-manager=greenpass-deploy --force-conflicts -f $renderedApp | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "GP Kubernetes manifest apply failed." }
        & kubectl --context $KubeContext delete job -n $namespace -l app.kubernetes.io/component=migration --ignore-not-found | Out-Null
        & kubectl --context $KubeContext apply -f $renderedMigration | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Migration Job creation failed." }
        & kubectl --context $KubeContext -n $namespace wait --for=condition=complete "job/gp-migrate-$versionName" --timeout=300s
        if ($LASTEXITCODE -ne 0) { throw "Migration Job failed." }

        & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-server --timeout=300s
        & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-worker --timeout=300s
        & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-web --timeout=300s

        $baseUrl = "http://127.0.0.1:30080"
        $headers = @{ Host = "greenpass.localhost" }
        foreach ($path in @("/", "/api/healthz", "/api/readyz")) {
            $response = Invoke-WebRequest -Uri "$baseUrl$path" -Headers $headers -UseBasicParsing -TimeoutSec 15
            if ([int]$response.StatusCode -ne 200) { throw "HTTP smoke test failed for ${path}: $($response.StatusCode)" }
        }
        $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
        $loginBody = '{"username":"zhangli","password":"GreenPass@123"}'
        $loginResponse = Invoke-WebRequest -Uri "$baseUrl/api/auth/login" -Headers $headers -ContentType "application/json" -Method Post -Body $loginBody -WebSession $session -UseBasicParsing -TimeoutSec 15
        if ([int]$loginResponse.StatusCode -ne 200) { throw "Login smoke test failed: $($loginResponse.StatusCode)" }
        $meResponse = Invoke-WebRequest -Uri "$baseUrl/api/auth/me" -Headers $headers -WebSession $session -UseBasicParsing -TimeoutSec 15
        if ([int]$meResponse.StatusCode -ne 200) { throw "Authenticated session smoke test failed: $($meResponse.StatusCode)" }

        if ($imSnapshot.Count -gt 0) {
            $imAfter = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
            if ($imAfter.items.Count -ne $imSnapshot.Count) { throw "IM deployment count changed during GreenPass deployment." }
            foreach ($deployment in $imAfter.items) {
                $value = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
                if (-not $imSnapshot.ContainsKey($deployment.metadata.name) -or $imSnapshot[$deployment.metadata.name] -ne $value) {
                    throw "IM workload changed during GreenPass deployment: $($deployment.metadata.name)"
                }
            }
        } else {
            Write-Host "No open-im-local namespace found; IM workload isolation check not applicable."
        }
    } catch {
        if ($previousImages.Count -gt 0) {
            Write-Warning "Post-deployment validation failed; restoring the previous GP deployment images."
            foreach ($entry in $previousImages.GetEnumerator()) {
                $containerName = switch ($entry.Key) { "gp-server" { "server" } "gp-worker" { "worker" } "gp-web" { "web" } }
                & kubectl --context $KubeContext -n $namespace set image "deployment/$($entry.Key)" "$containerName=$($entry.Value)" | Out-Null
            }
            foreach ($deploymentName in $previousImages.Keys) {
                & kubectl --context $KubeContext -n $namespace rollout status "deployment/$deploymentName" --timeout=300s | Out-Null
            }
        }
        throw
    }

    $commit = & git rev-parse HEAD
    Write-Host "GreenPass kind initialization succeeded: version=$Version commit=$commit deployment=kubectl http=ready login=ready"
} finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $renderedApp, $renderedMigration
    Pop-Location
}
