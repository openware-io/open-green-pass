param(
    [string]$BaseUrl = "http://greenpass.localhost:30080",
    [string]$Context = "kind-open-im-local",
    [string]$Namespace = "gp",
    [string]$Username = $(if ($env:GP_ARBITER_USERNAME) { $env:GP_ARBITER_USERNAME } else { "zhangli" }),
    [string]$Password = $env:GP_ARBITER_PASSWORD,
    [string]$ProjectId = "2876690473259008",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("gp-arbiter-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
$cookie = Join-Path $tmp "cookies.txt"
$checks = [System.Collections.Generic.List[object]]::new()

function Add-Check([string]$Id, [bool]$Ok, [string]$Detail) {
    $checks.Add([pscustomobject]@{ id = $Id; ok = $Ok; detail = $Detail })
    $tag = if ($Ok) { "PASS" } else { "FAIL" }
    Write-Host "[$tag] $Id - $Detail"
}

function Invoke-Kubectl([string[]]$Arguments) {
    $output = & kubectl --context $Context -n $Namespace @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw ($output -join "`n") }
    return (($output | Out-String).Trim())
}

function Invoke-Psql([string]$Sql) {
    $output = & kubectl --context $Context -n $Namespace exec deploy/gp-postgres -- psql -U gp -d gp -tAc $Sql 2>&1
    if ($LASTEXITCODE -ne 0) { throw ($output -join "`n") }
    return (($output | Out-String).Trim())
}

function Invoke-Curl([string[]]$Arguments) {
    $bodyFile = Join-Path $tmp ("body-" + [guid]::NewGuid().ToString("N") + ".json")
    $status = & curl.exe -s -o $bodyFile -w "%{http_code}" @Arguments
    $body = if (Test-Path $bodyFile) { Get-Content $bodyFile -Raw } else { "" }
    return [pscustomobject]@{ status = [int]$status; body = $body }
}

try {
    if (-not $Password) {
        throw "GP_ARBITER_PASSWORD is required (or pass -Password)."
    }

    $deployments = @("gp-server", "gp-worker", "gp-web", "gp-dsh-gateway")
    foreach ($name in $deployments) {
        $json = Invoke-Kubectl @("get", "deployment", $name, "-o", "json")
        $d = $json | ConvertFrom-Json
        $desired = [int]$d.spec.replicas
        $ready = [int]$d.status.readyReplicas
        Add-Check "deploy/$name" ($ready -ge $desired -and $desired -gt 0) "ready=$ready desired=$desired"
    }

    $migration = Invoke-Psql "select version || '|' || dirty from schema_migrations;"
    Add-Check "schema/version" ($migration -eq "48|f" -or $migration -eq "48|false") "schema_migrations=$migration"

    $requiredTables = Invoke-Psql "select count(*) from information_schema.tables where table_schema='gp' and table_name in ('gen_run','gen_run_unit','cost_price');"
    Add-Check "schema/run-tables" ($requiredTables -eq "3") "gen_run/gen_run_unit/cost_price count=$requiredTables"

    $requiredPriceColumns = Invoke-Psql "select count(*) from information_schema.columns where table_schema='gp' and table_name='cost_price' and column_name in ('model_key','input_per_1k','output_per_1k','cache_read_per_1k','effective_from');"
    Add-Check "schema/price-columns" ($requiredPriceColumns -eq "5") "cost_price columns=$requiredPriceColumns/5"

    $requiredCostColumns = Invoke-Psql "select count(*) from information_schema.columns where table_schema='gp' and table_name='cost_line_item' and column_name in ('cache_read_tokens','input_unit_price','output_unit_price','cache_read_unit_price','price_missing');"
    Add-Check "schema/cost-columns" ($requiredCostColumns -eq "5") "cost_line_item columns=$requiredCostColumns/5"

    $ttl = Invoke-Psql "select count(*) from pg_proc where proname='purge_generation_artifacts';"
    Add-Check "schema/ttl-function" ($ttl -eq "1") "purge_generation_artifacts=$ttl"

    $loginBody = @{ username = $Username; password = $Password } | ConvertTo-Json -Compress
    $loginFile = Join-Path $tmp "login.json"
    [System.IO.File]::WriteAllText($loginFile, $loginBody, [System.Text.UTF8Encoding]::new($false))
    $login = Invoke-Curl @("-c", $cookie, "-H", "Content-Type: application/json", "--data-binary", "@$loginFile", "$BaseUrl/api/auth/login")
    Add-Check "api/login" ($login.status -eq 200) "HTTP $($login.status)"
    if ($login.status -ne 200) { throw "login failed: $($login.body)" }

    $runs = Invoke-Curl @("-b", $cookie, "-H", "x-gp-team-id: 1001", "$BaseUrl/api/generation/runs?project_id=$ProjectId&limit=5")
    Add-Check "api/generation-runs" ($runs.status -eq 200 -and $runs.body.TrimStart().StartsWith("[")) "HTTP $($runs.status)"
    $runsJson = if ($runs.status -eq 200) { $runs.body | ConvertFrom-Json } else { @() }
    Add-Check "api/run-history" (@($runsJson).Count -gt 0) "runs=$(@($runsJson).Count)"
    if (@($runsJson).Count -gt 0) {
        $latest = @($runsJson)[0]
        $latestRun = Invoke-Curl @("-b", $cookie, "-H", "x-gp-team-id: 1001", "$BaseUrl/api/generation/runs/$($latest.id)")
        Add-Check "api/run-detail" ($latestRun.status -eq 200 -and ($latestRun.body | ConvertFrom-Json).units) "HTTP $($latestRun.status)"
    }

    $prices = Invoke-Curl @("-b", $cookie, "-H", "x-gp-team-id: 1001", "$BaseUrl/api/cost/prices")
    Add-Check "api/cost-prices" ($prices.status -eq 200 -and $prices.body.TrimStart().StartsWith("[")) "HTTP $($prices.status)"
    $items = Invoke-Curl @("-b", $cookie, "-H", "x-gp-team-id: 1001", "$BaseUrl/api/cost/items?limit=5")
    Add-Check "api/cost-items" ($items.status -eq 200) "HTTP $($items.status)"

    $cancelledClean = Invoke-Psql "select count(*) from gen_run r where r.state='cancelled' and exists (select 1 from gen_run_unit u where u.run_id=r.id and u.batch_id is not null and not exists (select 1 from gen_seed s where s.batch_id=u.batch_id) and not exists (select 1 from gen_source_snapshot s where s.batch_id=u.batch_id));"
    Add-Check "evidence/cancel-cleanup" ([int]$cancelledClean -gt 0) "cancelled runs with cleaned units=$cancelledClean"

    $succeeded = Invoke-Psql "select count(*) from gen_run where state='succeeded' and completed_units>0;"
    Add-Check "evidence/run-succeeded" ([int]$succeeded -gt 0) "succeeded runs=$succeeded"

    $priced = Invoke-Psql "select count(*) from cost_line_item where biz_point='case_generate' and price_missing=false and amount>0 and model like 'dsh/%';"
    Add-Check "evidence/priced-generation" ([int]$priced -gt 0) "priced generation lines=$priced"

    $actualModel = Invoke-Psql "select count(*) from cost_line_item where biz_point='case_generate' and model like 'dsh/deepseek-%';"
    Add-Check "evidence/actual-model" ([int]$actualModel -gt 0) "actual DSH model lines=$actualModel"

    $runCost = Invoke-Psql "select count(*) from gen_run where cost_amount>0 and tokens_in>0 and tokens_out>0;"
    Add-Check "evidence/run-cost" ([int]$runCost -gt 0) "runs with cost/tokens=$runCost"

    $html = Invoke-Curl @("$BaseUrl/")
    Add-Check "ui/shell" ($html.status -eq 200) "HTTP $($html.status)"
    $jsPath = ([regex]::Match($html.body, 'src="([^"]+\.js)"')).Groups[1].Value
    if ($jsPath) {
        $js = Invoke-Curl @("$BaseUrl$jsPath")
        $uiText = $js.body
        Add-Check "ui/run-card" ($uiText.Contains("最近生成任务") -and $uiText.Contains("新建生成任务")) "run card strings"
        Add-Check "ui/run-review" ($uiText.Contains("全部通过并入库")) "run review action"
        Add-Check "ui/price-panel" ($uiText.Contains("模型价目表")) "model price panel"
    } else {
        Add-Check "ui/bundle" $false "JS bundle not found"
    }

    if (-not $SkipTests) {
        Push-Location $repoRoot
        try {
            & go test ./... *> (Join-Path $tmp "go-test.log")
            Add-Check "test/go" ($LASTEXITCODE -eq 0) "go test ./..."
            & npm run typecheck *> (Join-Path $tmp "typecheck.log")
            Add-Check "test/typecheck" ($LASTEXITCODE -eq 0) "npm run typecheck"
        } finally {
            Pop-Location
        }
    }
} catch {
    Add-Check "arbiter/runtime" $false $_.Exception.Message
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$failed = @($checks | Where-Object { -not $_.ok })
$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz"
$lines = @(
    "# Generation Arbiter Report",
    "",
    "- Time: $timestamp",
    "- Base URL: $BaseUrl",
    "- Checks: $($checks.Count)",
    "- Failed: $($failed.Count)",
    ""
) + ($checks | ForEach-Object { "- [" + $(if ($_.ok) { "x" } else { " " }) + "] $($_.id): $($_.detail)" })
$report = $lines -join "`n"
$reportDir = Join-Path $repoRoot "docs/status"
New-Item -ItemType Directory -Path $reportDir -Force | Out-Null
$reportPath = Join-Path $reportDir "ARBITER-COMPLETION-2026-10-23.md"
[System.IO.File]::WriteAllText($reportPath, $report + "`n", [System.Text.UTF8Encoding]::new($false))
Write-Host ""
Write-Host "Arbiter report: $reportPath"
if ($failed.Count -gt 0) {
    Write-Error "$($failed.Count) completion checks failed."
    exit 1
}
Write-Host "All completion checks passed."
exit 0
