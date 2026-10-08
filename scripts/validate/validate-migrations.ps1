# GreenPass migration validator (Windows native, equivalent to validate-migrations.sh)
# Checks: file name format / up-down pairing / table prefix registry (ENGINEERING-SPEC 8)
param([string]$MigDir = "migrations")
$ErrorActionPreference = "Stop"
$prefixes = @("tgt_","cas_","ctr_","rpt_","tnt_","iam_","mdl_","repo_","run_","res_","env_","gen_","gate_","cost_","aud_","evd_","ts_","svc_","cicd_")
$err = 0
$up = @{}; $down = @{}
Get-ChildItem (Join-Path (Get-Location) $MigDir) -Filter *.sql -ErrorAction SilentlyContinue | ForEach-Object {
  $base = $_.Name
  if ($base -notmatch '^[0-9]{6}_.+\.(up|down)\.sql$') {
    Write-Output ("migcheck: bad file name: {0} (expect 000NNN_name.up.sql / .down.sql)" -f $base); $script:err++
  } else {
    $num = $base.Substring(0,6)
    if ($base -match '\.up\.sql$') { $up[$num] = $true } else { $down[$num] = $true }
  }
}
foreach($num in $up.Keys){ if(-not $down.ContainsKey($num)){ Write-Output ("migcheck: {0} missing .down.sql" -f $num); $script:err++ } }
foreach($num in $down.Keys){ if(-not $up.ContainsKey($num)){ Write-Output ("migcheck: {0} missing .up.sql" -f $num); $script:err++ } }
Get-ChildItem (Join-Path (Get-Location) $MigDir) -Filter *.up.sql -ErrorAction SilentlyContinue | ForEach-Object {
  $content = Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
  [regex]::Matches($content, '(?im)^\s*CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_\.]+)') | ForEach-Object {
    $tbl = $_.Groups[2].Value
    $b = ($tbl -split '\.')[-1]
    $matched = $false
    foreach($p in $prefixes){ if($b -like "$p*"){ $matched = $true; break } }
    if(-not $matched){ Write-Output ("migcheck: {0} table {1} prefix not registered (ENGINEERING-SPEC 8)" -f $_.Groups[0].Value, $b); $script:err++ }
  }
}
if($err -eq 0){ Write-Output "migcheck: OK" } else { exit 1 }
