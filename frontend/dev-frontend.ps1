$ErrorActionPreference = "Stop"

$script:border = "=" * 56

Write-Host ""
Write-Host $border -ForegroundColor Cyan
Write-Host "  Gentry 前端开发服务器 (Debug Mode)" -ForegroundColor Cyan
Write-Host $border -ForegroundColor Cyan
Write-Host ""

Write-Host "━━ 架构说明 ━━" -ForegroundColor Green
Write-Host ""
Write-Host "  前端：React 19 + TypeScript + Tailwind CSS 4 + Vite" -ForegroundColor Yellow
Write-Host "  生产构建会把 frontend/dist 嵌入 Go 二进制（单服务部署）" -ForegroundColor Yellow
Write-Host "  开发模式下由 Vite 提供 5173 端口并代理 /api 到后端 8080" -ForegroundColor Yellow
Write-Host ""

Write-Host "[启动] 前端开发服务器 (Debug Mode)" -ForegroundColor Magenta
Write-Host "[地址] http://localhost:5173" -ForegroundColor Magenta
Write-Host "[代理] /api -> http://localhost:8080" -ForegroundColor Magenta
Write-Host "[调试] 启用 sourcemap，热重载" -ForegroundColor Magenta
Write-Host ""

# 检查 node_modules
if (-not (Test-Path -LiteralPath "node_modules")) {
  Write-Host "[安装] 检测到 node_modules 缺失，运行 npm install..." -ForegroundColor Cyan
  npm install
  if ($LASTEXITCODE -ne 0) {
    Write-Host "[错误] npm install 失败" -ForegroundColor Red
    exit 1
  }
  Write-Host "[完成] 依赖安装完毕" -ForegroundColor Green
  Write-Host ""
}

Write-Host "[启动] 运行 vite dev (按 Ctrl+C 停止)..." -ForegroundColor Cyan
Write-Host $border -ForegroundColor DarkGray
Write-Host ""

# 启动前端
npm run dev
