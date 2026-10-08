<#!
.SYNOPSIS
Creates a plain-SQL backup of only the GreenPass gp database in the gp namespace.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $OutputPath,
    [string] $KubeContext = "kind-open-im-local"
)

$ErrorActionPreference = "Stop"
$namespace = "gp"
$database = "gp"

if ([string]::IsNullOrWhiteSpace($OutputPath)) { throw "OutputPath is required" }
$resolvedParent = Split-Path -Parent ([IO.Path]::GetFullPath($OutputPath))
if (-not (Test-Path -LiteralPath $resolvedParent -PathType Container)) { throw "Backup directory does not exist: $resolvedParent" }
if (Test-Path -LiteralPath $OutputPath) { throw "Refusing to overwrite existing backup: $OutputPath" }

$pods = @(kubectl --context $KubeContext -n $namespace get pod -l app=gp-postgres -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}') | Where-Object { $_ }
if ($pods.Count -ne 1) { throw "Expected exactly one gp-postgres pod in namespace gp; found $($pods.Count)" }

# pg_dump reads credentials from the PostgreSQL container environment. No secret
# is copied to the host or written into this artifact.
& kubectl --context $KubeContext -n $namespace exec $pods[0] -- sh -c 'pg_dump -U "$POSTGRES_USER" --format=plain --no-owner --no-privileges --dbname gp' |
    Out-File -LiteralPath $OutputPath -Encoding utf8NoBOM
if ($LASTEXITCODE -ne 0) {
    Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
    throw "gp database backup failed"
}

Write-Output "Created gp-only SQL backup: $([IO.Path]::GetFullPath($OutputPath))"
