$ErrorActionPreference = "Stop"
if (Get-Command go -ErrorAction SilentlyContinue) {
  & (Join-Path $PSScriptRoot "build.ps1")
} else {
  Write-Error "Go is not installed and no release download is configured yet."
}
