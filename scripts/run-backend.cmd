@echo off
rem ---------------------------------------------------------------
rem  Start the backend server.
rem  Wrapped in .cmd because the default Windows execution policy is
rem  Restricted, which refuses to run .ps1 files directly.
rem  Bypass only affects this process; it does not change system settings.
rem ---------------------------------------------------------------
setlocal
call "%~dp0_run-ps51.cmd"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0run-backend.ps1" %*
set "RC=%ERRORLEVEL%"
endlocal & exit /b %RC%
