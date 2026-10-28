$ErrorActionPreference = "Stop"

Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "   PayFlow -- Full System End-to-End Test Suite" -ForegroundColor Cyan
Write-Host "================================================================================" -ForegroundColor Cyan

$RootDir = Split-Path -Parent $PSScriptRoot

Write-Host "[1/4] Running Go Payment Service Tests..." -ForegroundColor Yellow
Push-Location "$RootDir\services\payment"
try {
    go test ./...
} finally {
    Pop-Location
}

Write-Host "[2/4] Running E2E Invariants Verification..." -ForegroundColor Yellow
Push-Location "$RootDir\services\payment"
try {
    go run ./cmd/e2e
} finally {
    Pop-Location
}

Write-Host "[3/4] Running API Gateway Tests..." -ForegroundColor Yellow
Push-Location "$RootDir\services\api-gateway"
try {
    npm test
} finally {
    Pop-Location
}

Write-Host "[4/4] Building React Web Dashboard..." -ForegroundColor Yellow
Push-Location "$RootDir\apps\web"
try {
    npm run build
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "All verification suites passed successfully!" -ForegroundColor Green
