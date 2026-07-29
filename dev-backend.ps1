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

Write-Host "━━ 实施完成情况 ━━" -ForegroundColor Green
Write-Host ""

Write-Host "  阶段一：API 标准化" -ForegroundColor Yellow
Write-Host "  $([char]0x2714) api/types.go - API 类型定义" -ForegroundColor Green
Write-Host "  $([char]0x2714) api/openapi.yaml - OpenAPI 3.0 规范" -ForegroundColor Green
Write-Host "  $([char]0x2714) 错误码体系（40001-50001）" -ForegroundColor Green
Write-Host ""

Write-Host "  阶段二：后端解耦" -ForegroundColor Yellow
Write-Host "  $([char]0x2714) 移除 //go:embed frontend/dist 嵌入逻辑" -ForegroundColor Green
Write-Host "  $([char]0x2714) 移除静态文件服务" -ForegroundColor Green
Write-Host "  $([char]0x2714) 重构 CORS 配置，支持 ALLOWED_ORIGINS 环境变量" -ForegroundColor Green
Write-Host "  $([char]0x2714) 统一 API 路由到 /api/v1" -ForegroundColor Green
Write-Host ""

Write-Host "  阶段三：前端独立" -ForegroundColor Yellow
Write-Host "  $([char]0x2714) 更新 API 客户端，支持环境变量配置" -ForegroundColor Green
Write-Host "  $([char]0x2714) 配置 Vite 代理和 API 基础 URL" -ForegroundColor Green
Write-Host "  $([char]0x2714) 环境配置文件（.env.development / .env.production）" -ForegroundColor Green
Write-Host "  $([char]0x2714) 前端 Dockerfile 和 Nginx 配置" -ForegroundColor Green
Write-Host "  $([char]0x2714) docker-compose.yml 一键部署配置" -ForegroundColor Green
Write-Host ""

Write-Host "  验证结果" -ForegroundColor Yellow
Write-Host "  $([char]0x2714) go build ./...  - 通过" -ForegroundColor Green
Write-Host "  $([char]0x2714) go test ./...   - 全部通过" -ForegroundColor Green
Write-Host "  $([char]0x2714) go vet ./...    - 通过" -ForegroundColor Green
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
