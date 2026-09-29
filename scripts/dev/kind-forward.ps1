# GP0-03 dev port-forward helper (Windows PowerShell)
# Maps gp-namespace services to host ports that avoid im-saas occupancy (ENGINEERING-SPEC 11).
# Usage:
#   .\kind-forward.ps1 start      # start all port-forwards (background)
#   .\kind-forward.ps1 stop       # stop recorded port-forwards
#   .\kind-forward.ps1 cleanup    # kill leftover kubectl port-forward processes
param([ValidateSet("start","stop","cleanup")][string]$Action = "start")

$namespace = "gp"
$pidFile = Join-Path $PSScriptRoot ".gp-portforward.pids"
$kubectl = "kubectl"

$mappings = @(
  @{ Name = "gp-postgres-postgresql"; Ports = "5433:5432" },
  @{ Name = "gp-redis-master";    Ports = "6380:6379" },
  @{ Name = "gp-temporal-frontend"; Ports = "7233:7233" },
  @{ Name = "gp-temporal-web";    Ports = "8082:8080" },
  @{ Name = "gp-seaweedfs";    Ports = "9100:9000" }
)

function Write-Forward {
  if ($Action -eq "start") {
    foreach ($m in $mappings) {
      $args = "port-forward -n $namespace svc/$($m.Name) $($m.Ports) --address 127.0.0.1"
      $p = Start-Process -FilePath $kubectl -ArgumentList $args -WindowStyle Hidden -PassThru
      Add-Content -Path $pidFile -Value $p.Id -Encoding ascii
      Write-Output "started port-forward svc/$($m.Name) -> $($m.Ports) (PID $($p.Id))"
    }
  } elseif ($Action -eq "stop") {
    if (Test-Path $pidFile) {
      Get-Content $pidFile | ForEach-Object {
        if ($_ -match '^\d+$') { Stop-Process -Id ([int]$_) -Force -ErrorAction SilentlyContinue }
      }
      Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
      Write-Output "stopped recorded port-forwards"
    } else {
      Write-Output "no recorded pids"
    }
  } elseif ($Action -eq "cleanup") {
    Get-CimInstance Win32_Process -Filter "Name='kubectl.exe'" | Where-Object {
      $_.CommandLine -match "port-forward"
    } | ForEach-Object {
      Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
      Write-Output "killed kubectl port-forward PID $($_.ProcessId)"
    }
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
  }
}

Write-Forward
