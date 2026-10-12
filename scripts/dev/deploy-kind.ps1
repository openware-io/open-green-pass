[CmdletBinding()]
param(
    [string] $KindCluster = "open-im-local",
    [string] $KubeContext = "kind-open-im-local",
    [string] $Version = "0.1.25-SNAPSHOT",
    [string] $PostgresPassword = "gp-local-postgres",
    [string] $RedisPassword = "gp-local-redis",
    [string] $ObjectStoreSecret = "gp-local-object-store"
)

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$chart = Join-Path $repositoryRoot "deploy/helm/gp"
$values = Join-Path $chart "values-kind.yaml"
$namespace = "gp"
$runnerNamespace = "gp-runner"
$release = "gp"

if ($Version -notmatch '^\d+\.\d+\.\d+-SNAPSHOT$') {
    throw "Development releases must use X.Y.Z-SNAPSHOT without a v prefix."
}

foreach ($command in @("docker", "kind", "kubectl", "helm", "git")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Required command not found: $command" }
}
if ((& kubectl --context $KubeContext config current-context) -ne $KubeContext) { throw "Kubernetes context is not available: $KubeContext" }
if (-not ((& kind get clusters) -contains $KindCluster)) { throw "Kind cluster is not available: $KindCluster" }

$imBefore = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
$imSnapshot = @{}
foreach ($deployment in $imBefore.items) {
    $imSnapshot[$deployment.metadata.name] = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
}

$images = @(
    @{ Name = "ghcr.io/openware-io/open-green-pass:$Version"; File = "Dockerfile" },
    @{ Name = "ghcr.io/openware-io/open-green-pass-web:$Version"; File = "Dockerfile.web" },
    @{ Name = "ghcr.io/openware-io/open-green-pass-migrate:$Version"; File = "Dockerfile.migrate" }
)

Push-Location $repositoryRoot
try {
    foreach ($image in $images) {
        & docker build --pull=false --file $image.File --tag $image.Name .
        if ($LASTEXITCODE -ne 0) { throw "Image build failed: $($image.Name)" }
        & kind load docker-image --name $KindCluster --nodes "$KindCluster-control-plane,$KindCluster-worker" $image.Name
        if ($LASTEXITCODE -ne 0) { throw "Image load failed: $($image.Name)" }
    }

    & kubectl --context $KubeContext apply -f deploy/k8s/namespaces.yaml | Out-Null
    & helm template $release $chart --namespace $namespace -f $values --set-string "migration.image.tag=$Version" --set-string "server.image.tag=$Version" --set-string "worker.image.tag=$Version" --set-string "web.image.tag=$Version" | & kubectl --context $KubeContext apply --dry-run=server -f - | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Server-side manifest validation failed." }
    & kubectl --context $KubeContext -n $namespace create secret generic gp-postgres-auth --from-literal=username=gp --from-literal=password=$PostgresPassword --dry-run=client -o yaml | & kubectl --context $KubeContext apply -f - | Out-Null
    & kubectl --context $KubeContext -n $namespace create secret generic gp-redis-auth --from-literal=password=$RedisPassword --dry-run=client -o yaml | & kubectl --context $KubeContext apply -f - | Out-Null
    $dsn = "postgres://gp:$PostgresPassword@gp-postgres.gp.svc.cluster.local:5432/gp?sslmode=disable"
    & kubectl --context $KubeContext -n $namespace create secret generic gp-app-config --from-literal=GP_DB_DSN=$dsn --from-literal=GP_REDIS_ADDR=gp-redis.gp.svc.cluster.local:6379 --from-literal=GP_REDIS_PASSWORD=$RedisPassword --from-literal=GP_MINIO_ENDPOINT=gp-seaweedfs.gp.svc.cluster.local:9000 --from-literal=GP_MINIO_KEY=gpminio --from-literal=GP_MINIO_SECRET=$ObjectStoreSecret --dry-run=client -o yaml | & kubectl --context $KubeContext apply -f - | Out-Null

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

    & helm lint $chart -f $values --set-string "migration.image.tag=$Version" --set-string "server.image.tag=$Version" --set-string "worker.image.tag=$Version" --set-string "web.image.tag=$Version"
    if ($LASTEXITCODE -ne 0) { throw "Helm lint failed." }
    & helm template $release $chart -n $namespace -f $values --set-string "migration.image.tag=$Version" --set-string "server.image.tag=$Version" --set-string "worker.image.tag=$Version" --set-string "web.image.tag=$Version" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Helm template failed." }

    & helm upgrade --install $release $chart --namespace $namespace -f $values --set-string "migration.image.tag=$Version" --set-string "server.image.tag=$Version" --set-string "worker.image.tag=$Version" --set-string "web.image.tag=$Version" --atomic --wait --timeout 10m
    if ($LASTEXITCODE -ne 0) { throw "Helm deployment failed and was rolled back." }

    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-server --timeout=300s
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-worker --timeout=300s
    & kubectl --context $KubeContext -n $namespace rollout status deployment/gp-web --timeout=300s

    $imAfter = & kubectl --context $KubeContext -n open-im-local get deployment -o json | ConvertFrom-Json
    foreach ($deployment in $imAfter.items) {
        $value = "$($deployment.metadata.uid)|$($deployment.metadata.generation)|$($deployment.spec.template.metadata.annotations.'kubectl.kubernetes.io/restartedAt')"
        if (-not $imSnapshot.ContainsKey($deployment.metadata.name) -or $imSnapshot[$deployment.metadata.name] -ne $value) {
            throw "IM workload changed during GreenPass deployment: $($deployment.metadata.name)"
        }
    }

    $revision = (& helm status $release -n $namespace -o json | ConvertFrom-Json).version
    $commit = & git rev-parse HEAD
    Write-Host "GreenPass kind deployment succeeded: version=$Version commit=$commit helmRevision=$revision"
} finally {
    Pop-Location
}
