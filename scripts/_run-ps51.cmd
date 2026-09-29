@echo off
rem ---------------------------------------------------------------
rem  Shared preamble for the .cmd wrappers that invoke Windows
rem  PowerShell 5.1.
rem
rem  Why this exists:
rem    When one of these wrappers is started from a PowerShell 7 shell,
rem    PSModulePath is inherited with the PowerShell 7 module folders
rem    listed first. Windows PowerShell 5.1 then tries to autoload the
rem    *Core edition* copies of its own modules, fails, and reports it
rem    as absurd errors like "Get-FileHash is not recognized" -- a
rem    cmdlet that has shipped with 5.1 since v4.
rem
rem    Pointing PSModulePath back at the 5.1 folders fixes the whole
rem    class of problems, not just that one cmdlet, and the wrappers
rem    then behave the same however they were launched.
rem
rem  Usage (no setlocal in here on purpose -- the variables must land
rem  in the caller's scope):
rem      call "%~dp0_run-ps51.cmd"
rem ---------------------------------------------------------------

set "PSModulePath=%SystemRoot%\system32\WindowsPowerShell\v1.0\Modules"
if exist "%ProgramFiles%\WindowsPowerShell\Modules" set "PSModulePath=%PSModulePath%;%ProgramFiles%\WindowsPowerShell\Modules"
if exist "%USERPROFILE%\Documents\WindowsPowerShell\Modules" set "PSModulePath=%PSModulePath%;%USERPROFILE%\Documents\WindowsPowerShell\Modules"
exit /b 0
