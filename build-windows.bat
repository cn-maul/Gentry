@echo off
chcp 65001 >nul 2>&1
cd /d "%~dp0"

echo ===== Gentry Build =====

echo [1/3] Installing frontend dependencies...
cd frontend
call pnpm install
if errorlevel 1 goto :err
cd ..

echo [2/3] Building frontend...
cd frontend
call pnpm run build
if errorlevel 1 goto :err
cd ..

echo [3/3] Compiling backend...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -buildvcs=false -o "%~dp0gentry.exe" .
if errorlevel 1 goto :err

echo ===== Done =====
echo Output: gentry.exe
echo Run: gentry.exe
pause
exit /b 0

:err
echo Build failed.
pause
exit /b 1