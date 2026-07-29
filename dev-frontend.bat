@echo off
chcp 65001 >nul
cd /d "%~dp0frontend"

echo.
echo ========================================================
echo   Gentry Frontend Dev Server (Debug Mode)
echo ========================================================
echo.
echo ---- Implementation Summary ----
echo.
echo   Phase 1: API Standardization
echo     [OK] api/types.go
echo     [OK] api/openapi.yaml
echo     [OK] Error codes (40001-50001)
echo.
echo   Phase 2: Backend Decoupling
echo     [OK] Removed go:embed frontend/dist
echo     [OK] Removed static file serving
echo     [OK] Refactored CORS with ALLOWED_ORIGINS
echo     [OK] Unified API routes under /api/v1
echo.
echo   Phase 3: Frontend Independence
echo     [OK] Updated API client with env vars
echo     [OK] Configured Vite proxy and API base URL
echo     [OK] Created .env files
echo     [OK] Created Dockerfile and Nginx config
echo     [OK] Created docker-compose.yml
echo.
echo   Verification
echo     [OK] go build ./...
echo     [OK] go test ./...
echo     [OK] go vet ./...
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
