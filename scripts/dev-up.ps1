Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

Write-Host "Starting MySQL and Redis..."
docker compose up -d

Write-Host "Waiting for auction-mysql to become healthy..."
do {
    Start-Sleep -Seconds 2
    $health = docker inspect --format='{{.State.Health.Status}}' auction-mysql 2>$null
    if (-not $health) {
        $health = "starting"
    }
    Write-Host "mysql: $health"
} while ($health -ne "healthy")

Write-Host "Starting backend on http://localhost:8080"
Start-Process powershell -WindowStyle Hidden -ArgumentList "-NoExit", "-Command", "cd `"$repoRoot\server-go`"; go run ."

Write-Host "Starting mobile H5 on http://localhost:5173"
Start-Process powershell -WindowStyle Hidden -ArgumentList "-NoExit", "-Command", "cd `"$repoRoot\mobile-h5`"; npm.cmd install; npm.cmd run dev -- --host 0.0.0.0 --port 5173"

Write-Host "Starting admin web on http://localhost:5174"
Start-Process powershell -WindowStyle Hidden -ArgumentList "-NoExit", "-Command", "cd `"$repoRoot\admin-web`"; npm.cmd install; npm.cmd run dev -- --host 0.0.0.0 --port 5174"

Write-Host "Services are launching:"
Write-Host "- Backend:   http://localhost:8080/health"
Write-Host "- Mobile H5: http://localhost:5173"
Write-Host "- Admin Web: http://localhost:5174"
