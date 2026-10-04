@echo off
setlocal
call "%~dp0_run-ps51.cmd"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0publish-release.ps1" %*
set "RC=%ERRORLEVEL%"
endlocal & exit /b %RC%
