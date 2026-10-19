[CmdletBinding()]
param(
    [string] $AdbPath = 'D:\greentools\android-sdk\platform-tools\adb.exe',
    [string] $TaskName = 'GreenPass Local Device Agent'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$agentPath = Join-Path $repositoryRoot 'tools/local-device-agent/server.cjs'
$node = (Get-Command node -ErrorAction Stop).Source

if (-not (Test-Path -LiteralPath $agentPath)) { throw "Device agent is missing: $agentPath" }
if (-not (Test-Path -LiteralPath $AdbPath)) { throw "ADB not found: $AdbPath" }

# schtasks starts this trusted, repository-owned script at interactive logon.
# The agent itself only binds to 127.0.0.1 and cannot expose ADB to kind.
# The agent also knows the default ADB location, so use a direct executable
# action here. It is more reliable than a nested cmd.exe command on Windows.
$command = "`"$node`" `"$agentPath`""
& schtasks.exe /Create /TN $TaskName /SC ONLOGON /RL LIMITED /TR $command /F | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Unable to register the GreenPass local device agent.' }
& schtasks.exe /Run /TN $TaskName | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Unable to start the GreenPass local device agent.' }

Start-Sleep -Seconds 1
$health = Invoke-WebRequest -Uri 'http://127.0.0.1:48173/healthz' -UseBasicParsing -TimeoutSec 5
if ($health.StatusCode -ne 204) { throw "Device agent health check failed: $($health.StatusCode)" }
Write-Host 'GreenPass local device agent is running and will start at next sign-in.'
