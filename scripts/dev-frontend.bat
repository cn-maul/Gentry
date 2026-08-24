@echo off
chcp 65001 >nul
cd /d "%~dp0..\frontend"

echo.
echo ========================================================
echo   Gentry Frontend Dev Server (Debug Mode)
echo ========================================================
echo.
echo ---- Architecture ----
echo.
echo   Frontend: React 19 + TypeScript + Tailwind CSS 4 + Vite
echo   Production builds embed frontend/dist into the Go binary
echo   Dev mode: Vite serves 5173 and proxies /api to 8080
echo.
echo ========================================================
echo.

echo [INFO] Starting frontend dev server (Debug Mode)
echo [INFO] URL: http://localhost:5173
echo [INFO] Proxy: /api -^> http://localhost:8080
echo [INFO] Sourcemap + HMR enabled
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

echo [INFO] Running npm run dev (Press Ctrl+C to stop)...
echo ========================================================
echo.

call npm run dev

pause
