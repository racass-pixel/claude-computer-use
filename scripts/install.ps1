$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root "bin"
$exe = Join-Path $bin "cu.exe"
$manifest = Get-Content (Join-Path $root ".claude-plugin\plugin.json") -Raw | ConvertFrom-Json
$version = $manifest.version
$repo = "racass-pixel/claude-computer-use"
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "cu_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/v$version"
New-Item -ItemType Directory -Force $bin | Out-Null
$tmp = Join-Path $env:TEMP "cu-install-$version-$PID"
New-Item -ItemType Directory -Force $tmp | Out-Null
try {
  Write-Host "cu: downloading $base/$asset"
  Invoke-WebRequest -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset) -UseBasicParsing
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt") -UseBasicParsing
  $line = Select-String -Path (Join-Path $tmp "checksums.txt") -Pattern ([regex]::Escape($asset)) | Select-Object -First 1
  if (-not $line) { throw "no checksum for $asset" }
  $expected = ($line.Line -split "\s+")[0].ToLower()
  $actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
  if ($expected -ne $actual) { throw "checksum mismatch for $asset" }
  Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force
  Copy-Item (Join-Path $tmp "cu.exe") $exe -Force
  Write-Host "cu: installed $exe ($version)"
} catch {
  Write-Host "cu: release download failed ($($_.Exception.Message)); trying go build"
  if (Get-Command go -ErrorAction SilentlyContinue) {
    & (Join-Path $PSScriptRoot "build.ps1")
  } else {
    throw "Neither a release download nor Go is available. Install Go from https://go.dev/dl/ and rerun."
  }
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
