param (
    [switch]$TestOnly,
    [switch]$BuildOnly
)

$ErrorActionPreference = "Stop"

Write-Host "==> Checking Go formatting..." -ForegroundColor Cyan
go fmt ./...

Write-Host "==> Running static analysis (go vet)..." -ForegroundColor Cyan
go vet ./...

if (-not $BuildOnly) {
    Write-Host "==> Running test suite with coverage..." -ForegroundColor Cyan
    go test -v -cover ./...
}

if (-not $TestOnly) {
    Write-Host "==> Compiling email-agent.exe..." -ForegroundColor Cyan
    go build -v -o email-agent.exe .
    Write-Host "==> Build complete: email-agent.exe" -ForegroundColor Green
}
