@echo off
setlocal
cd /d "%~dp0"
rem One-click build: frontend dist -> backend/web/dist -> embedded Go binary.
rem Keep this file pure ASCII so cmd parses it correctly on any codepage.

echo [1/3] Building frontend...
pushd frontend
if not exist node_modules call npm install
if errorlevel 1 goto :fail
call npm run build
if errorlevel 1 goto :fail
popd

echo [2/3] Copying frontend dist to backend\web\dist ...
if exist backend\web\dist rmdir /s /q backend\web\dist
xcopy /e /i /y frontend\dist backend\web\dist >nul
if errorlevel 1 goto :fail

echo [3/3] Compiling backend (frontend embedded) ...
pushd backend
set CGO_ENABLED=0
go build -ldflags="-s -w" -buildvcs=false -o gentry.exe .
if errorlevel 1 goto :fail
popd

echo.
echo Build OK: backend\gentry.exe  (start it, then visit http://localhost:8080)
exit /b 0

:fail
echo Build FAILED.
exit /b 1
