[CmdletBinding(SupportsShouldProcess)]
param()

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$relativeTargets = @(
    'node_modules',
    'dist',
    'bin',
    '.outputs',
    'design',
    'dev.log',
    'dev.err.log',
    'schedreplay.exe',
    'replay-payload.json',
    'scripts\dev\.gp-portforward.pids'
)

foreach ($relativeTarget in $relativeTargets) {
    $target = [System.IO.Path]::GetFullPath((Join-Path $repositoryRoot $relativeTarget))
    if (-not $target.StartsWith($repositoryRoot + [System.IO.Path]::DirectorySeparatorChar)) {
        throw "拒绝清理仓库外路径：$target"
    }
    if ((Test-Path -LiteralPath $target) -and $PSCmdlet.ShouldProcess($target, '删除本地可重建产物')) {
        Remove-Item -LiteralPath $target -Recurse -Force
    }
}

Write-Host 'GreenPass 本地可重建产物已清理。'
