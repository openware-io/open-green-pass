param(
    [string] $ChromePath = "C:\Program Files\Google\Chrome\Application\chrome.exe",
    [int] $Port = 48200
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$agent = Join-Path $root "tools/browser-runner/server.cjs"
if (-not (Test-Path -LiteralPath $agent)) { throw "Browser runner is missing: $agent" }
if (-not (Test-Path -LiteralPath $ChromePath)) { throw "Chrome is missing: $ChromePath" }
if (-not (Test-Path -LiteralPath (Join-Path $root "tools/browser-runner/node_modules/playwright-core"))) {
    throw "Run npm install in tools/browser-runner first"
}

try {
    $health = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/healthz" -UseBasicParsing -TimeoutSec 2
    if ($health.StatusCode -eq 204) { Write-Host "Browser runner already running on port $Port"; exit 0 }
} catch { }

$env:GP_CHROME_PATH = $ChromePath
$env:GP_BROWSER_RUNNER_PORT = [string]$Port
$node = (Get-Command node -ErrorAction Stop).Source
Start-Process -FilePath $node -ArgumentList @("`"$agent`"") -WindowStyle Hidden
Start-Sleep -Seconds 2
$health = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/healthz" -UseBasicParsing -TimeoutSec 5
if ($health.StatusCode -ne 204) { throw "Browser runner health check failed: $($health.StatusCode)" }
Write-Host "Browser runner listening on http://127.0.0.1:$Port"
