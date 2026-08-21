$ErrorActionPreference = 'Stop'
$projectRoot = Resolve-Path "$PSScriptRoot\.."
Push-Location $projectRoot
try {
    go run ./cmd/server
} finally {
    Pop-Location
}
