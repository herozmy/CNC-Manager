@echo off
rem ---------------------------------------------------------------
rem  Serve the built dist at http://127.0.0.1:5173
rem  Use this to verify a production build locally before deploying.
rem  Requires the backend on http://127.0.0.1:8080
rem ---------------------------------------------------------------
setlocal
cd /d "%~dp0"
call npm.cmd run preview
endlocal
