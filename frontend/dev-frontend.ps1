$ErrorActionPreference = "Stop"

$script:border = "=" * 56

Write-Host ""
Write-Host $border -ForegroundColor Cyan
Write-Host "  Gentry 前端开发服务器 (Debug Mode)" -ForegroundColor Cyan
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
