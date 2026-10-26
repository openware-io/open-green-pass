param(
    [string] $AgentPath = "",
    [string] $Port = "48173"
)

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if (-not $AgentPath) { $AgentPath = Join-Path $repositoryRoot "tools/local-device-agent/server.cjs" }
if (-not (Test-Path -LiteralPath $AgentPath)) { throw "Device agent is missing: $AgentPath" }

try {
    $health = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/healthz" -UseBasicParsing -TimeoutSec 2
    if ($health.StatusCode -eq 204) { exit 0 }
} catch {
    # Agent is not listening; start it below.
}

$node = (Get-Command node -ErrorAction Stop).Source
Start-Process -FilePath $node -ArgumentList @("`"$AgentPath`"") -WindowStyle Hidden
