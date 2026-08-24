@echo off
chcp 65001 >nul
cd /d "%~dp0"

set "PORT=%1"
if "%PORT%"=="" set "PORT=8080"

echo.
echo ========================================================
echo   Gentry Backend Dev Server
echo ========================================================
echo.

set GIN_MODE=debug
set PORT=%PORT%

echo [INFO] Starting backend API server
echo [INFO] Listening on http://localhost:%PORT%/api/v1
echo.

go run -buildvcs=false .

pause
