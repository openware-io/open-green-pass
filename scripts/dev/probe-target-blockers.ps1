# Probes the target-side blockers that prevent M3 strict completion.
# Read-only except for one disposable client-release draft attempt and one media upload.
# Usage: pwsh -NoProfile -File scripts/dev/probe-target-blockers.ps1
param(
    [string]$KubeContext = "kind-open-im-local",
    [string]$GreenPassNamespace = "gp",
    [string]$BaseUrl = "http://127.0.0.1:30080",
    [string]$HostHeader = "open-im.localhost"
)

$ErrorActionPreference = "Stop"

function Get-AdminToken {
    $encoded = & kubectl --context $KubeContext -n $GreenPassNamespace get secret gp-im-token -o jsonpath='{.data.token}'
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($encoded)) {
        throw "gp-im-token secret is not available"
    }
    return [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded.Trim()))
}

function Invoke-Target {
    param(
        [string]$Method,
        [string]$Path,
        [string]$Body = "",
        [string]$ContentType = "application/json",
        [string]$IdempotencyKey = ""
    )
    $arguments = @("--noproxy", "*", "-s", "-w", "`n%{http_code}", "-X", $Method,
        "-H", "Host: $HostHeader", "-H", "Authorization: Bearer $script:token")
    if ($ContentType) { $arguments += @("-H", "Content-Type: $ContentType") }
    if ($IdempotencyKey) { $arguments += @("-H", "Idempotency-Key: $IdempotencyKey") }
    if ($Body -ne "") {
        $temp = Join-Path ([IO.Path]::GetTempPath()) ("gp-probe-" + [guid]::NewGuid().ToString() + ".json")
        [IO.File]::WriteAllText($temp, $Body, [Text.UTF8Encoding]::new($false))
        $arguments += @("--data-binary", "@$temp")
    }
    $arguments += "$BaseUrl$Path"
    $raw = & curl.exe @arguments
    $lines = @($raw)
    $status = [int]$lines[-1]
    $content = ($lines[0..($lines.Count - 2)] -join "`n")
    if ($Body -ne "") { Remove-Item -LiteralPath $temp -Force -ErrorAction SilentlyContinue }
    return [pscustomobject]@{ Status = $status; Body = $content }
}

$script:token = Get-AdminToken

$report = [ordered]@{}

# Blocker 1: openPlatform create route is not exposed.
$report.openPlatform = Invoke-Target -Method POST -Path "/api/v1/admin/open-platform/applications" `
    -Body '{"name":"gp-e2e-probe"}' -IdempotencyKey ([guid]::NewGuid().ToString())

# Blocker 2: media validation rejects a valid 1x1 PNG.
Add-Type -AssemblyName System.Drawing
$bitmap = [Drawing.Bitmap]::new(100, 100)
$graphics = [Drawing.Graphics]::FromImage($bitmap)
$graphics.Clear([Drawing.Color]::FromArgb(255, 20, 120, 220))
$pngPath = Join-Path ([IO.Path]::GetTempPath()) ("gp-probe-" + [guid]::NewGuid().ToString() + ".png")
$bitmap.Save($pngPath, [Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()
$png = [IO.File]::ReadAllBytes($pngPath)
$sha = [BitConverter]::ToString([Security.Cryptography.SHA256]::Create().ComputeHash($png)).Replace("-", "").ToLowerInvariant()
$mediaBody = @{
    scope = "avatar"; mediaKind = "image"; fileName = "gp-probe.png";
    contentType = "image/png"; size = $png.Length; sha256 = $sha
} | ConvertTo-Json -Compress
$session = Invoke-Target -Method POST -Path "/api/v1/media/upload-sessions" -Body $mediaBody `
    -IdempotencyKey ([guid]::NewGuid().ToString())
$report.mediaSession = $session
if ($session.Status -eq 200) {
    $sessionValue = $session.Body | ConvertFrom-Json
    $upload = & curl.exe --noproxy "*" -s -o NUL -w "%{http_code}" -X PUT `
        -H "Content-Type: image/png" --data-binary "@$pngPath" $sessionValue.uploadUrl
    $completeBody = @{ size = $png.Length; sha256 = $sha } | ConvertTo-Json -Compress
    $complete = Invoke-Target -Method POST -Path "/api/v1/media/upload-sessions/$($sessionValue.uploadSessionId)/complete" `
        -Body $completeBody -IdempotencyKey ([guid]::NewGuid().ToString())
    $media = Invoke-Target -Method GET -Path "/api/v1/media/$($sessionValue.objectId)"
    $access = Invoke-Target -Method GET -Path "/api/v1/media/$($sessionValue.objectId)/access"
    $report.mediaUpload = [ordered]@{ uploadStatus = [int]$upload; complete = $complete; media = $media; access = $access }
}

# Blocker 3: client release draft creation returns 500.
$releaseBody = @{
    platform = "android"; channel = "stable"; releaseVersion = "9.9.10"; buildNumber = 1000;
    releaseNotes = "gp blocker probe"; storeUrl = "https://play.google.com/store/apps/details?id=com.gp.probe";
    mandatory = $false; reason = "gp blocker probe";
    compatibility = @{ protocolVersion = "v1"; minimumServerCapabilityVersion = 1 };
    artifacts = @(@{ architecture = "universal"; packageType = "google-play" })
} | ConvertTo-Json -Depth 6 -Compress
$report.clientRelease = Invoke-Target -Method POST -Path "/api/v1/admin/client-releases" -Body $releaseBody `
    -IdempotencyKey ([guid]::NewGuid().ToString())

Remove-Item -LiteralPath $pngPath -Force -ErrorAction SilentlyContinue
$report | ConvertTo-Json -Depth 8
