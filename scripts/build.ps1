$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$manifest = Get-Content (Join-Path $root ".claude-plugin\plugin.json") -Raw | ConvertFrom-Json
$version = $manifest.version
$env:CGO_ENABLED = "0"
$out = Join-Path $root "bin\cu.exe"
& go build -trimpath -ldflags "-s -w -X main.version=$version" -o $out "$root/cmd/cu"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "built $out ($version)"
