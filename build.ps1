# Gentry Windows 一键构建：前端产物嵌入 Go 二进制
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "[1/3] 构建前端..." -ForegroundColor Cyan
Push-Location "$root\frontend"
try {
    if (-not (Test-Path node_modules)) { npm install }
    if ($LASTEXITCODE -ne 0) { throw "npm install 失败" }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "npm run build 失败" }
} finally {
    Pop-Location
}

Write-Host "[2/3] 拷贝前端产物到 backend\web\dist..." -ForegroundColor Cyan
$distTarget = "$root\backend\web\dist"
if (Test-Path $distTarget) { Remove-Item -Recurse -Force $distTarget }
Copy-Item -Recurse "$root\frontend\dist" $distTarget

Write-Host "[3/3] 编译后端（嵌入前端）..." -ForegroundColor Cyan
Push-Location "$root\backend"
try {
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -buildvcs=false -o gentry.exe .
    if ($LASTEXITCODE -ne 0) { throw "go build 失败" }
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "✅ 构建完成: backend\gentry.exe （启动后访问 http://localhost:8080）" -ForegroundColor Green
