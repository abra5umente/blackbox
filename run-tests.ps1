$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $env:GOCACHE) {
    $env:GOCACHE = Join-Path $scriptDir ".gocache"
}
if (-not $env:BLACKBOX_MIGRATIONS_DIR) {
    $env:BLACKBOX_MIGRATIONS_DIR = Join-Path $scriptDir "migrations"
}

if (-not (Test-Path $env:GOCACHE)) {
    New-Item -ItemType Directory -Path $env:GOCACHE | Out-Null
}

Push-Location $scriptDir
try {
    Write-Host "==> go test ./..."
    go test ./...
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        Write-Host "Tests failed with exit code $exitCode" -ForegroundColor Red
        exit $exitCode
    }
    Write-Host "All tests passed!" -ForegroundColor Green
    exit 0
}
finally {
    Pop-Location
}
