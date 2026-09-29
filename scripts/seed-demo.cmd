@echo off
rem ---------------------------------------------------------------
rem  Seed demo data: 2 drawings, operations, programs, tool tables, NC versions.
rem  Requires the backend to be running in another window:
rem      scripts\run-backend.cmd
rem ---------------------------------------------------------------
setlocal
call "%~dp0_run-ps51.cmd"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0seed-demo.ps1" %*
set "RC=%ERRORLEVEL%"
endlocal & exit /b %RC%
