param(
  [string]$Port = "8080"
)

$ErrorActionPreference = "Stop"

$script:border = "=" * 56

Write-Host ""
Write-Host $border -ForegroundColor Cyan
Write-Host "  Gentry 后端开发服务器 (Debug Mode)" -ForegroundColor Cyan
Write-Host $border -ForegroundColor Cyan
Write-Host ""

Write-Host "━━ 架构说明 ━━" -ForegroundColor Green
Write-Host ""
Write-Host "  单服务模式：前端构建产物嵌入二进制（web/dist）" -ForegroundColor Yellow
Write-Host "  生产构建：make build（自动构建前端并嵌入）" -ForegroundColor Yellow
Write-Host "  开发模式：前端另起 Vite（frontend/ 下 npm run dev，代理 /api）" -ForegroundColor Yellow
Write-Host ""

Write-Host $border -ForegroundColor Cyan
Write-Host ""

# 设置 Gin 为 Debug 模式
$env:GIN_MODE = "debug"
$env:PORT = $Port

Write-Host "[启动] 后端 API 服务 (Debug Mode)" -ForegroundColor Magenta
Write-Host "[端口] http://localhost:$Port/api/v1" -ForegroundColor Magenta
Write-Host "[模式] GIN_MODE=debug" -ForegroundColor Magenta
if ($env:ALLOWED_ORIGINS) {
  Write-Host "[CORS] 允许来源: $env:ALLOWED_ORIGINS" -ForegroundColor Magenta
} else {
  Write-Host "[CORS] 允许来源: * (默认)" -ForegroundColor Magenta
}
Write-Host "[调试] 输出详细请求日志" -ForegroundColor Magenta
Write-Host ""

# 编译检查
Write-Host "[检查] 执行 go vet ..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) {
  Write-Host "[警告] go vet 发现潜在问题，请检查" -ForegroundColor Yellow
} else {
  Write-Host "[通过] go vet 检查通过" -ForegroundColor Green
}
Write-Host ""

Write-Host "[启动] 运行 go run . (按 Ctrl+C 停止)..." -ForegroundColor Cyan
Write-Host $border -ForegroundColor DarkGray
Write-Host ""

# 启动后端
go run -buildvcs=false .
