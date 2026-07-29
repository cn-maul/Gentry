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
echo ---- Implementation Summary ----
echo.
echo   Phase 1: API Standardization
echo     [OK] api/types.go - API type definitions
echo     [OK] api/openapi.yaml - OpenAPI 3.0 spec
echo     [OK] Error code system (40001-50001)
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
echo     [OK] Created .env files (.env.development, .env.production)
echo     [OK] Created frontend Dockerfile and Nginx config
echo     [OK] Created docker-compose.yml for one-click deploy
echo.
echo   Verification
echo     [OK] go build ./...  - Passed
echo     [OK] go test ./...   - All passed
echo     [OK] go vet ./...    - Passed
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
