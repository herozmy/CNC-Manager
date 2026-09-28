@echo off
rem ---------------------------------------------------------------
rem  Start the frontend dev server (hot reload).
rem  Requires the backend on http://127.0.0.1:8080
rem  (run scripts\run-backend.cmd in another window first).
rem
rem  Open http://127.0.0.1:5173 after it starts.
rem ---------------------------------------------------------------
setlocal
cd /d "%~dp0"
call npm.cmd run dev
endlocal
