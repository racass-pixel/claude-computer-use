@echo off
setlocal
set "EXE=%~dp0cu.exe"
if not exist "%EXE%" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0..\scripts\install.ps1" 1>&2
)
if not exist "%EXE%" (
  echo cu: bin\cu.exe is missing and install failed. Run scripts\build.ps1 with Go installed. 1>&2
  exit /b 1
)
"%EXE%" %*
