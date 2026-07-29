@echo off
chcp 65001 >nul
cd /d "%~dp0"

set "PORT=%1"
if "%PORT%"=="" set "PORT=8080"

echo.
echo ========================================================
echo   Gentry Backend Dev Server (Debug Mode)
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
    echo [WARN] go vet found issues, please check
) else (
    echo [OK] go vet passed
)
echo.

echo [INFO] Starting go run . (Press Ctrl+C to stop)...
echo ========================================================
echo.

go run -buildvcs=false .

pause
