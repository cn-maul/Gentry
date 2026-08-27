@echo off
chcp 65001 >nul
cd /d "%~dp0..\backend"

set "PORT=%1"
if "%PORT%"=="" set "PORT=8080"

echo.
echo ========================================================
echo   Gentry Backend Dev Server (Debug Mode)
echo ========================================================
echo.

set GIN_MODE=debug
set PORT=%PORT%

echo [INFO] Starting backend API server (Debug Mode)
echo [INFO] Listening on http://localhost:%PORT%/api/v1
echo [INFO] GIN_MODE=debug
if not "%ALLOWED_ORIGINS%"=="" (
    echo [INFO] CORS origins: %ALLOWED_ORIGINS%
) else (
    echo [INFO] CORS origins: * (default)
)
echo.

echo [INFO] Running go vet ...
go vet ./...
if errorlevel 1 (
    echo [WARN] go vet found potential issues, please check
) else (
    echo [OK] go vet passed
)
echo.

echo [INFO] Starting go run . (Press Ctrl+C to stop)...
echo ========================================================
echo.

go run -buildvcs=false .

pause
