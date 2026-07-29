# Gentry 前后端分离拆分计划

## 1. 背景与目标

### 1.1 现状分析

当前 Gentry 项目采用前后端混合部署模式：
- 前端 Vue 3 应用通过 `//go:embed frontend/dist` 嵌入 Go 二进制
- Go HTTP 服务器同时提供 API 服务和静态文件服务
- 开发模式下前端通过 Vite 代理 API 请求

### 1.2 拆分目标

1. **独立部署**：前端和后端可独立部署、独立扩展
2. **独立开发**：前端和后端可并行开发，互不影响
3. **标准化 API**：提供清晰的 API 契约，方便第三方适配
4. **多前端支持**：支持 Web、移动端、桌面端等多种前端实现
5. **简化协作**：降低第三方开发者接入门槛

## 2. 架构设计

### 2.1 分离后架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              客户端层                                       │
├─────────────────┬─────────────────┬─────────────────┬───────────────────────┤
│   Web 前端      │   移动端 App     │   桌面端 App    │   第三方应用          │
│   (Vue 3)       │   (Flutter/     │   (Electron/    │   (任意语言)          │
│                 │    React Native)│    Tauri)       │                       │
└────────┬────────┴────────┬────────┴────────┬────────┴───────────┬───────────┘
         │                 │                 │                    │
         └─────────────────┴─────────────────┴────────────────────┘
                                    │
                              HTTP/HTTPS
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API 网关层（可选）                                 │
│                     (Nginx / Traefik / Cloudflare)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                              后端服务层                                      │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Gentry Backend (Go)                          │   │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │   │
│   │  │ Monitor     │  │ Fetcher     │  │ Notify      │  │ Database  │  │   │
│   │  │ Engine      │  │ Service     │  │ Service     │  │ (SQLite)  │  │   │
│   │  └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构规划

```
gentry/                          # 后端仓库 (新)
├── cmd/
│   └── gentry/
│       └── main.go             # 入口文件
├── internal/
│   ├── monitor/                # 监控核心
│   ├── fetcher/                # HTTP 抓取
│   ├── notify/                 # 通知推送
│   ├── database/               # 数据持久化
│   └── web/                    # HTTP 服务（仅 API）
├── pkg/                        # 可导出包（如有需要）
├── api/                        # API 定义
│   ├── openapi.yaml            # OpenAPI 3.0 规范
│   └── types.go                # API 类型定义
├── configs/                    # 配置文件
├── deployments/                # 部署配置
│   ├── docker/
│   └── kubernetes/
├── docs/                       # 后端文档
├── go.mod
├── go.sum
├── Makefile
└── README.md

gentry-web/                      # 前端仓库 (新)
├── src/
│   ├── api/                    # API 客户端
│   ├── components/             # 公共组件
│   ├── views/                  # 页面
│   ├── composables/            # 组合式函数
│   ├── router/                 # 路由
│   └── stores/                 # 状态管理
├── public/                     # 静态资源
├── tests/                      # 测试
├── package.json
├── vite.config.js
├── tsconfig.json               # 可选：迁移到 TypeScript
└── README.md
```

## 3. API 契约设计

### 3.1 API 规范

**基础 URL**: `/api/v1`

**认证方式**: Bearer Token (JWT 或静态 Token)

**响应格式**:
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

**错误格式**:
```json
{
  "code": 40001,
  "message": "错误描述",
  "details": { ... }
}
```

### 3.2 API 端点清单

#### 3.2.1 监控器管理

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/monitors` | 获取监控器列表 | 是 |
| GET | `/api/v1/monitors/:name` | 获取监控器详情 | 是 |
| POST | `/api/v1/monitors` | 创建监控器 | 是 |
| PUT | `/api/v1/monitors/:name` | 更新监控器 | 是 |
| DELETE | `/api/v1/monitors/:name` | 删除监控器 | 是 |
| POST | `/api/v1/monitors/:name/start` | 启动监控器 | 是 |
| POST | `/api/v1/monitors/:name/stop` | 停止监控器 | 是 |
| POST | `/api/v1/monitors/:name/check` | 手动检查 | 是 |
| POST | `/api/v1/monitors/:name/baseline` | 重置基线 | 是 |

#### 3.2.2 监控数据

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/monitors/:name/updates` | 获取更新记录 | 是 |
| GET | `/api/v1/monitors/:name/events` | 获取事件列表 | 是 |
| GET | `/api/v1/monitors/:name/snapshots` | 获取快照列表 | 是 |
| GET | `/api/v1/monitors/:name/config` | 获取监控配置 | 是 |

#### 3.2.3 智能扫描

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| POST | `/api/v1/monitors/preview` | 预览扫描结果 | 是 |
| POST | `/api/v1/monitors/smart-create` | 智能创建监控器 | 是 |
| POST | `/api/v1/monitors/validate` | 验证监控配置 | 是 |

#### 3.2.4 推送管理

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/settings/notification-accounts` | 获取推送账户列表 | 是 |
| POST | `/api/v1/settings/notification-accounts` | 创建推送账户 | 是 |
| PUT | `/api/v1/settings/notification-accounts/:id` | 更新推送账户 | 是 |
| DELETE | `/api/v1/settings/notification-accounts/:id` | 删除推送账户 | 是 |
| GET | `/api/v1/settings/notification-providers` | 获取推送服务商列表 | 是 |

#### 3.2.5 扫描规则

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/settings/scan-rules` | 获取扫描规则列表 | 是 |
| POST | `/api/v1/settings/scan-rules` | 创建扫描规则 | 是 |
| PUT | `/api/v1/settings/scan-rules/:id` | 更新扫描规则 | 是 |
| DELETE | `/api/v1/settings/scan-rules/:id` | 删除扫描规则 | 是 |
| GET | `/api/v1/settings/scan-rules/export` | 导出规则 | 是 |
| POST | `/api/v1/settings/scan-rules/import` | 导入规则 | 是 |
| POST | `/api/v1/settings/scan-rules/:id/test` | 测试规则 | 是 |

#### 3.2.6 系统设置

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/settings/notifications` | 获取通知设置 | 是 |
| PUT | `/api/v1/settings/notifications` | 更新通知设置 | 是 |
| GET | `/api/v1/health` | 健康检查 | 是 |
| GET | `/api/v1/stats` | 获取统计信息 | 是 |
| GET | `/api/v1/groups` | 获取分组列表 | 是 |

#### 3.2.7 公开接口

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/version` | 获取版本信息 | 否 |
| GET | `/api/update/check` | 检查更新 | 否 |

### 3.3 API 文档

使用 **OpenAPI 3.0** 规范编写 API 文档，提供：
- 完整的端点定义
- 请求/响应示例
- 认证方式说明
- 错误码定义

## 4. 实施计划

### 4.1 阶段一：API 标准化（1-2 周）

**目标**：完善 API 文档，统一响应格式

- [ ] 编写 OpenAPI 3.0 规范文档
- [ ] 统一所有 API 响应格式
- [ ] 定义完整的错误码体系
- [ ] 添加 API 版本控制

**交付物**：
- `api/openapi.yaml` - OpenAPI 规范文件
- `api/types.go` - API 类型定义
- `docs/api.md` - API 使用文档

### 4.2 阶段二：后端解耦（2-3 周）

**目标**：移除前端嵌入逻辑，后端仅提供 API 服务

- [ ] 移除 `//go:embed frontend/dist` 嵌入逻辑
- [ ] 移除静态文件服务
- [ ] 重构 CORS 配置，支持多来源
- [ ] 添加 API 请求日志中间件
- [ ] 添加速率限制（可选）

**交付物**：
- 纯 API 后端服务
- 部署文档更新

### 4.3 阶段三：前端独立（2-3 周）

**目标**：前端项目独立，可独立开发和部署

- [ ] 创建独立的前端仓库
- [ ] 配置 Vite 代理和 API 基础 URL
- [ ] 添加环境变量配置（开发/生产）
- [ ] 实现 API 客户端封装
- [ ] 添加请求拦截器（认证、错误处理）

**交付物**：
- 独立前端项目
- 前端开发文档

### 4.4 阶段四：多前端支持（持续）

**目标**：提供 SDK 和示例，支持第三方开发

- [ ] 提供 JavaScript/TypeScript SDK
- [ ] 提供各语言 SDK 示例（Python、Go、Java）
- [ ] 编写第三方接入指南
- [ ] 提供 Postman/Insomnia 集合

**交付物**：
- SDK 和示例代码
- 第三方接入文档

## 5. 技术方案

### 5.1 后端改动

#### 5.1.1 移除前端嵌入

```go
// main.go 改动前
//go:embed frontend/dist
var frontendDist embed.FS

// main.go 改动后
// 移除 embed 和静态文件服务
```

#### 5.1.2 CORS 配置优化

```go
// 支持多来源配置
func setupCORS() cors.Config {
    return cors.Config{
        AllowOrigins:     getAllowedOrigins(), // 从配置读取
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }
}
```

#### 5.1.3 环境变量扩展

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8080 | HTTP 服务端口 |
| `GIN_MODE` | release | Gin 运行模式 |
| `ALLOWED_ORIGINS` | * | 允许的 CORS 来源 |
| `API_TOKEN` | 空 | API 认证令牌 |
| `DB_PATH` | gentry.db | 数据库路径 |

### 5.2 前端改动

#### 5.2.1 API 客户端封装

```typescript
// src/api/client.ts
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

class ApiClient {
  private baseUrl: string
  private token: string | null

  constructor() {
    this.baseUrl = API_BASE_URL
    this.token = localStorage.getItem('api_token')
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
        ...options.headers,
      },
    })
    return response.json()
  }
}

export const apiClient = new ApiClient()
```

#### 5.2.2 环境配置

```env
# .env.development
VITE_API_BASE_URL=http://localhost:8080/api/v1

# .env.production
VITE_API_BASE_URL=/api/v1
```

### 5.3 认证方案

#### 5.3.1 Token 认证（当前）

```
Authorization: Bearer <token>
```

#### 5.3.2 未来扩展：JWT（可选）

```json
{
  "sub": "user_id",
  "exp": 1234567890,
  "iat": 1234567890
}
```

## 6. 部署方案

### 6.1 后端部署

#### Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o gentry .

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/gentry .
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./gentry"]
```

#### Docker Compose

```yaml
version: '3.8'
services:
  gentry:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - API_TOKEN=your-secret-token
      - ALLOWED_ORIGINS=https://your-domain.com
    restart: unless-stopped
```

### 6.2 前端部署

#### 静态文件部署

```dockerfile
# Dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

#### Nginx 配置

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    root /usr/share/nginx/html;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    location /api/ {
        proxy_pass http://gentry-backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 6.3 一体化部署（可选）

使用 Nginx 统一代理前后端：

```nginx
server {
    listen 80;
    server_name gentry.example.com;
    
    # 前端
    location / {
        root /var/www/gentry-web;
        try_files $uri $uri/ /index.html;
    }
    
    # API
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 7. 开发工作流

### 7.1 后端开发

```bash
# 克隆后端仓库
git clone https://github.com/cn-maul/gentry.git
cd gentry

# 运行开发服务器
make dev

# 运行测试
go test ./...

# 构建
make build
```

### 7.2 前端开发

```bash
# 克隆前端仓库
git clone https://github.com/cn-maul/gentry-web.git
cd gentry-web

# 安装依赖
npm install

# 运行开发服务器（连接本地后端）
npm run dev

# 构建
npm run build
```

### 7.3 联调开发

```bash
# 终端 1：启动后端
cd gentry
make dev

# 终端 2：启动前端
cd gentry-web
npm run dev

# 前端通过 Vite 代理将 /api 请求转发到后端
```

## 8. 迁移指南

### 8.1 从旧版本迁移

1. **备份数据**：复制 `gentry.db` 到安全位置
2. **部署新版本**：按照新部署方案部署
3. **恢复数据**：将 `gentry.db` 放到新数据目录
4. **验证功能**：检查所有监控器正常运行

### 8.2 第三方接入指南

1. 阅读 API 文档 (`docs/api.md`)
2. 获取 API Token
3. 使用 SDK 或直接调用 API
4. 参考示例代码

## 9. 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| API 变更导致兼容性问题 | 高 | 使用版本控制 (v1, v2)，保持向后兼容 |
| 前端独立后部署复杂度增加 | 中 | 提供 Docker Compose 一键部署方案 |
| 第三方开发者学习成本 | 中 | 提供完善的文档和 SDK |
| 分离后测试复杂度增加 | 中 | 添加契约测试，确保 API 一致性 |

## 10. 时间规划

| 阶段 | 时间 | 主要任务 |
|------|------|----------|
| 阶段一 | 第 1-2 周 | API 标准化，编写 OpenAPI 文档 |
| 阶段二 | 第 3-5 周 | 后端解耦，移除前端嵌入逻辑 |
| 阶段三 | 第 6-8 周 | 前端独立，创建独立仓库 |
| 阶段四 | 持续 | SDK 开发，文档完善 |

## 11. 验收标准

- [ ] API 文档完整，覆盖所有端点
- [ ] 后端可独立部署运行
- [ ] 前端可独立开发和构建
- [ ] 前后端可独立部署并正常协作
- [ ] 提供至少一种语言的 SDK 示例
- [ ] 第三方开发者可仅通过 API 文档接入

---

*文档版本：v1.0*
*创建日期：2026-07-29*
