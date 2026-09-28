@echo off
rem ---------------------------------------------------------------
rem  Production build -> frontend\dist
rem  Type-checks first, so it fails loudly on TS errors.
rem ---------------------------------------------------------------
setlocal
cd /d "%~dp0"
call npm.cmd run build
endlocal
