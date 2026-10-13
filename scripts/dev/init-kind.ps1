[CmdletBinding()]
param(
    [string] $KindCluster = "",
    [string] $KubeContext = "",
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
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$deployScript = Join-Path $PSScriptRoot "deploy-kind.ps1"

foreach ($command in @("docker", "kind", "kubectl")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Required command not found: $command" }
}

Push-Location $root
try {
    $clusters = @(& kind get clusters 2>$null)
    if ($LASTEXITCODE -ne 0) { throw "Unable to query local kind clusters." }
    if (-not $KindCluster) {
        if ($clusters.Count -eq 0) {
            $KindCluster = "open-im-local"
        } elseif ($clusters.Count -eq 1) {
            $KindCluster = $clusters[0]
            Write-Host "Reusing the only existing kind cluster: $KindCluster"
        } else {
            $clustersWithGp = @()
            $clustersWithIm = @()
            foreach ($candidate in $clusters) {
                $candidateContext = "kind-$candidate"
                if ((& kubectl config get-contexts $candidateContext --no-headers 2>$null).Count -eq 0) { continue }
                if (& kubectl --context $candidateContext get namespace gp --ignore-not-found -o name 2>$null) { $clustersWithGp += $candidate }
                if (& kubectl --context $candidateContext get namespace open-im-local --ignore-not-found -o name 2>$null) { $clustersWithIm += $candidate }
            }
            if ($clustersWithGp.Count -eq 1) {
                $KindCluster = $clustersWithGp[0]
                Write-Host "Reusing the existing kind cluster that already hosts GreenPass: $KindCluster"
            } elseif ($clustersWithGp.Count -eq 0 -and $clustersWithIm.Count -eq 1) {
                $KindCluster = $clustersWithIm[0]
                Write-Host "Reusing the only existing kind cluster that hosts the IM test target: $KindCluster"
            } else {
                throw "Multiple kind clusters exist ($($clusters -join ', ')); specify -KindCluster because no single GP/IM target cluster can be identified safely."
            }
        }
    }
    if (-not $KubeContext) { $KubeContext = "kind-$KindCluster" }
    $createdCluster = $false
    if ($clusters -notcontains $KindCluster) {
        $config = Join-Path $root "deploy/kind/cluster.yaml"
        if (-not (Test-Path $config)) { throw "Kind cluster config is missing: $config" }
        Write-Host "Creating kind cluster $KindCluster from $config"
        & kind create cluster --name $KindCluster --config $config --wait 180s
        if ($LASTEXITCODE -ne 0) { throw "Kind cluster creation failed." }
        $createdCluster = $true
    }

    if ((& kubectl config get-contexts $KubeContext --no-headers 2>$null).Count -eq 0) {
        throw "Expected Kubernetes context is missing: $KubeContext"
    }
    & kubectl --context $KubeContext wait --for=condition=Ready node --all --timeout=180s
    if ($LASTEXITCODE -ne 0) { throw "Kind nodes did not become Ready." }

    $ingressNamespace = "ingress-nginx"
    $ingress = & kubectl --context $KubeContext -n $ingressNamespace get deployment ingress-nginx-controller -o name 2>$null
    if (-not $ingress) {
        if (-not $createdCluster) {
            throw "ingress-nginx is not installed in existing cluster $KubeContext. Install the approved local ingress controller before initializing GP."
        }
        Write-Host "Installing ingress-nginx into newly created cluster $KindCluster"
        & kubectl --context $KubeContext label node "$KindCluster-control-plane" ingress-ready=true --overwrite
        if ($LASTEXITCODE -ne 0) { throw "Unable to label the new kind control-plane for ingress-nginx." }
        & kubectl --context $KubeContext apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/kind/deploy.yaml
        if ($LASTEXITCODE -ne 0) { throw "ingress-nginx installation failed." }
    }
    & kubectl --context $KubeContext -n $ingressNamespace rollout status deployment/ingress-nginx-controller --timeout=300s
    if ($LASTEXITCODE -ne 0) { throw "ingress-nginx did not become Ready." }

    $kindNodes = @(& kind get nodes --name $KindCluster)
    if ($LASTEXITCODE -ne 0 -or $kindNodes.Count -eq 0) { throw "Unable to discover nodes in kind cluster $KindCluster." }
    $controlPlaneNode = $kindNodes | Where-Object { $_ -match '-control-plane$' } | Select-Object -First 1
    if (-not $controlPlaneNode) { throw "Kind cluster $KindCluster has no control-plane node." }
    $portMapping = & docker port $controlPlaneNode "30080/tcp" 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $portMapping -or -not ($portMapping | Where-Object { $_ -match ':30080$' })) {
        throw "Kind cluster $KindCluster does not expose container port 30080 on host port 30080; recreate it with deploy/kind/cluster.yaml or use a compatible existing cluster."
    }
    $ingressService = & kubectl --context $KubeContext -n $ingressNamespace get service ingress-nginx-controller -o json | ConvertFrom-Json
    $ingressHttpPort = $ingressService.spec.ports | Where-Object { $_.name -eq "http" } | Select-Object -First 1
    if (-not $ingressHttpPort -or [int]$ingressHttpPort.nodePort -ne 30080) {
        throw "ingress-nginx HTTP NodePort must be 30080 for the GreenPass local URL."
    }

    $deployParameters = @{
        KindCluster = $KindCluster
        KubeContext = $KubeContext
        Version = $Version
        PostgresPassword = $PostgresPassword
        RedisPassword = $RedisPassword
        ObjectStoreSecret = $ObjectStoreSecret
        ImageRegistry = $ImageRegistry
        ImageSource = $ImageSource
    }
    if ($SkipBuild) { $deployParameters.SkipBuild = $true }
    & $deployScript @deployParameters
    if ($LASTEXITCODE -ne 0) { throw "GreenPass kind deployment failed." }
} finally {
    Pop-Location
}
