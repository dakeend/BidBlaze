# ============================================================
# deploy.ps1  —  Deploy auction-system to 114.55.252.115
# Run from:  E:\code\ai_zijie\auction-system
# Usage:     .\deploy.ps1
# ============================================================
$ErrorActionPreference = "Stop"

$SERVER  = "114.55.252.115"
$USER    = "root"
$REMOTE  = "/opt/auction-system"

# ---- Step 1: Build frontends --------------------------------
Write-Host "`n=== [1/4] Building admin-web ===" -ForegroundColor Cyan
Push-Location admin-web
npm.cmd ci --prefer-offline 2>$null; if (-not $?) { npm.cmd install }
npm.cmd run build
Pop-Location

Write-Host "`n=== [1/4] Building mobile-h5 ===" -ForegroundColor Cyan
Push-Location mobile-h5
npm.cmd ci --prefer-offline 2>$null; if (-not $?) { npm.cmd install }
npm.cmd run build
Pop-Location

# ---- Step 2: Pack files ------------------------------------
Write-Host "`n=== [2/4] Packing files ===" -ForegroundColor Cyan
$PACK = "auction-deploy.tar.gz"

# Remove old pack if exists
Remove-Item $PACK -ErrorAction SilentlyContinue

# tar is built into Windows 10 1803+
# Exclude: uploads (large), node_modules, .data, video, *.winbak*
tar -czf $PACK `
    --exclude="server-go/uploads" `
    --exclude="*/node_modules" `
    --exclude=".data" `
    --exclude="video" `
    --exclude="*.winbak*" `
    docker-compose.prod.yml `
    .env.prod `
    nginx `
    remote-setup.sh `
    docs/schema-v2.sql `
    server-go `
    admin-web/dist `
    mobile-h5/dist

$sizeMB = [math]::Round((Get-Item $PACK).Length / 1MB, 1)
Write-Host "Archive: $PACK ($sizeMB MB)" -ForegroundColor Green

# ---- Step 3: Upload to server ------------------------------
Write-Host "`n=== [3/4] Uploading to server (enter password when prompted) ===" -ForegroundColor Cyan
ssh -o StrictHostKeyChecking=accept-new "${USER}@${SERVER}" "mkdir -p ${REMOTE}"
scp $PACK "${USER}@${SERVER}:${REMOTE}/"

# Extract on server
ssh "${USER}@${SERVER}" "cd ${REMOTE} && tar -xzf auction-deploy.tar.gz && rm -f auction-deploy.tar.gz"

Write-Host "Files uploaded and extracted." -ForegroundColor Green

# ---- Step 4: Run setup script on server --------------------
Write-Host "`n=== [4/4] Running setup on server (enter password when prompted) ===" -ForegroundColor Cyan
ssh "${USER}@${SERVER}" "chmod +x ${REMOTE}/remote-setup.sh && bash ${REMOTE}/remote-setup.sh"

# Cleanup local pack
Remove-Item $PACK -ErrorAction SilentlyContinue

Write-Host "`n=== Deployment finished ===" -ForegroundColor Green
Write-Host "  Mobile H5:  http://${SERVER}" -ForegroundColor White
Write-Host "  Admin Web:  http://${SERVER}:8081" -ForegroundColor White
Write-Host "  API health: http://${SERVER}/api/health" -ForegroundColor White
