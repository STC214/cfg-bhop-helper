$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$helperDir = $PSScriptRoot
$distDir = Join-Path $repoRoot "dist"
$outExe = Join-Path $distDir "cfg-bhop-helper.exe"

New-Item -ItemType Directory -Force -Path $distDir | Out-Null

Push-Location $helperDir
try {
    go build -trimpath -ldflags "-H=windowsgui -s -w" -o $outExe .
}
finally {
    Pop-Location
}

Write-Host "Portable exe created:"
Write-Host $outExe
