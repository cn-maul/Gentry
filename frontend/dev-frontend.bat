@echo off
chcp 65001 >nul
cd /d "%~dp0"

echo.
echo ========================================================
echo   Gentry Frontend Dev Server
echo ========================================================
echo.

echo [INFO] Starting frontend dev server
echo [INFO] URL: http://localhost:5173
echo [INFO] Proxy: /api -> http://localhost:8080
echo.

if not exist "node_modules\" (
    echo [INFO] Installing dependencies...
    call npm install
    if errorlevel 1 (
        echo [ERROR] npm install failed
        pause
        exit /b 1
    )
    echo [OK] Dependencies installed
    echo.
)

call npm run dev

pause
