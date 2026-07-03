@echo off
REM Install the brain CLI from GitHub Releases (Windows). No Go toolchain needed.
REM
REM   install.cmd
REM
REM Env:
REM   BRAIN_VERSION  release tag to install (default: latest)
REM   BRAIN_BINDIR   where to put the binary (default: %USERPROFILE%\bin)
setlocal enabledelayedexpansion

set "REPO=muthuishere/brain"
if "%BRAIN_VERSION%"=="" set "BRAIN_VERSION=latest"
if "%BRAIN_BINDIR%"=="" set "BRAIN_BINDIR=%USERPROFILE%\bin"

set "ASSET=brain_windows_amd64.zip"
if "%BRAIN_VERSION%"=="latest" (
  set "URL=https://github.com/%REPO%/releases/latest/download/%ASSET%"
) else (
  set "URL=https://github.com/%REPO%/releases/download/%BRAIN_VERSION%/%ASSET%"
)

echo downloading %ASSET% (%BRAIN_VERSION%)...
if not exist "%BRAIN_BINDIR%" mkdir "%BRAIN_BINDIR%"
set "TMP_ZIP=%TEMP%\brain_%RANDOM%.zip"

powershell -NoProfile -Command "Invoke-WebRequest -Uri '%URL%' -OutFile '%TMP_ZIP%'"
if errorlevel 1 exit /b 1
powershell -NoProfile -Command "Expand-Archive -Force -Path '%TMP_ZIP%' -DestinationPath '%BRAIN_BINDIR%'"
if errorlevel 1 exit /b 1
del "%TMP_ZIP%" >nul 2>&1

echo installed %BRAIN_BINDIR%\brain.exe
"%BRAIN_BINDIR%\brain.exe" install-skills
if errorlevel 1 exit /b 1

echo done. ensure %BRAIN_BINDIR% is on your PATH, then try:  brain --repo C:\tmp\mybrain init
endlocal
