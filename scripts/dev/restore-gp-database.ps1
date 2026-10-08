<#!
.SYNOPSIS
Restores a GreenPass gp SQL backup after explicitly recreating only database gp.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $BackupPath,
    [Parameter(Mandatory)] [string] $ConfirmRestore,
    [string] $KubeContext = "kind-open-im-local"
)

$ErrorActionPreference = "Stop"
$namespace = "gp"
$database = "gp"

if ($ConfirmRestore -cne "RESTORE_GP_DATABASE") {
    throw "Restore refused. Pass -ConfirmRestore RESTORE_GP_DATABASE to recreate only gp/gp."
}
if (-not (Test-Path -LiteralPath $BackupPath -PathType Leaf)) { throw "Backup file does not exist: $BackupPath" }

$pods = @(kubectl --context $KubeContext -n $namespace get pod -l app=gp-postgres -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}') | Where-Object { $_ }
if ($pods.Count -ne 1) { throw "Expected exactly one gp-postgres pod in namespace gp; found $($pods.Count)" }

# This is intentionally limited to the dedicated GP database. It never lists,
# selects, deletes, or writes resources in open-im-local or any other namespace.
& kubectl --context $KubeContext -n $namespace exec $pods[0] -- sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE IF EXISTS gp WITH (FORCE);" && psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE gp OWNER \"$POSTGRES_USER\";"'
if ($LASTEXITCODE -ne 0) { throw "Failed to recreate gp database" }

Get-Content -LiteralPath $BackupPath -Raw | kubectl --context $KubeContext -n $namespace exec -i $pods[0] -- sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d gp'
if ($LASTEXITCODE -ne 0) { throw "Restore into gp database failed; database is left for controlled recovery" }

Write-Output "Restored gp-only SQL backup into gp/gp."
