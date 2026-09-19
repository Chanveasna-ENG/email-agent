param (
    [switch]$TestOnly,
    [switch]$BuildOnly,
    [switch]$Linux
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
    if ($Linux) {
        Write-Host "==> Cross-compiling static Linux binary (bin/email-agent)..." -ForegroundColor Cyan
        $env:CGO_ENABLED = "0"
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
        go build -ldflags="-s -w" -v -o bin/email-agent ./cmd/email-agent
        Remove-Item Env:\CGO_ENABLED
        Remove-Item Env:\GOOS
        Remove-Item Env:\GOARCH
        Write-Host "==> Linux build complete: bin/email-agent" -ForegroundColor Green
    } else {
        Write-Host "==> Compiling email-agent.exe..." -ForegroundColor Cyan
        go build -v -o email-agent.exe ./cmd/email-agent
        Write-Host "==> Windows build complete: email-agent.exe" -ForegroundColor Green
    }
}

