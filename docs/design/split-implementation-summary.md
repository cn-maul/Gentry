# 前后端分离实施总结

## 实施概述

本次实施完成了 Gentry 项目的前后端分离拆分，使前端和后端可以独立开发、独立部署。

## 已完成的变更

### 阶段一：API 标准化

#### 新增文件
- `api/types.go` - API 类型定义，包含所有请求/响应结构体
- `api/openapi.yaml` - OpenAPI 3.0 规范文档

#### 错误码体系
| 代码 | 说明 |
|------|------|
| 0 | 成功 |
| 40001 | 请求参数错误 |
| 40101 | 未授权 |
| 40301 | 禁止访问 |
| 40401 | 资源不存在 |
| 40901 | 资源冲突 |
| 50001 | 服务器内部错误 |

### 阶段二：后端解耦

#### 修改文件
- `main.go` - 移除 `//go:embed frontend/dist` 和前端嵌入逻辑
- `web/types.go` - 移除 `frontendFS` 字段，简化 WebServer 结构
- `web/routes.go` - 移除静态文件服务，重构 CORS 配置

#### 主要变更
1. **移除前端嵌入**：删除 `//go:embed frontend/dist` 和相关代码
2. **移除静态文件服务**：删除 `StaticFS` 和 `NoRoute` 处理
3. **CORS 配置优化**：支持通过 `ALLOWED_ORIGINS` 环境变量配置多来源
4. **API 路由统一**：所有 API 统一在 `/api/v1` 路径下

#### 环境变量
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8080 | HTTP 服务端口 |
| `GIN_MODE` | release | Gin 运行模式 |
| `ALLOWED_ORIGINS` | * | 允许的 CORS 来源（逗号分隔） |
| `API_TOKEN` | 空 | API 认证令牌 |
| `DB_PATH` | gentry.db | 数据库路径 |

### 阶段三：前端独立

#### 修改文件
- `frontend/src/api/monitors.js` - 统一 API 客户端，支持环境变量配置
- `frontend/vite.config.js` - 支持环境变量配置 API 目标

#### 新增文件
- `frontend/.env.development` - 开发环境配置
- `frontend/.env.production` - 生产环境配置
- `frontend/.env.example` - 配置示例
- `frontend/Dockerfile` - 前端 Docker 构建文件
- `frontend/nginx.conf` - Nginx 配置文件
- `frontend/.dockerignore` - Docker 忽略文件

#### 部署相关
- `Dockerfile` - 后端 Docker 构建文件
- `docker-compose.yml` - 一键部署配置
- `.dockerignore` - Docker 忽略文件

## 目录结构

```
Gentry/
├── api/                        # API 定义
│   ├── types.go                # API 类型
│   └── openapi.yaml            # OpenAPI 规范
├── cmd/
│   └── gentry/
│       └── main.go             # 入口文件
├── database/                   # 数据库层
├── fetcher/                    # HTTP 抓取
├── frontend/                   # 前端项目（独立）
│   ├── src/                    # 源码
│   ├── Dockerfile              # 前端 Docker 构建
│   ├── nginx.conf              # Nginx 配置
│   ├── vite.config.js          # Vite 配置
│   └── .env.*                  # 环境配置
├── monitor/                    # 监控核心
├── notify/                     # 通知推送
├── web/                        # HTTP 服务（仅 API）
├── Dockerfile                  # 后端 Docker 构建
├── docker-compose.yml          # 一键部署
└── docs/                       # 文档
```

## 部署方式

### 方式一：Docker Compose 一键部署

```bash
# 克隆仓库
git clone https://github.com/cn-maul/Gentry.git
cd Gentry

# 启动服务
docker-compose up -d

# 访问前端 http://localhost
# API 地址 http://localhost:8080/api/v1
```

### 方式二：独立部署后端

```bash
# 构建
go build -o gentry .

# 运行
export API_TOKEN=your-secret-token
export ALLOWED_ORIGINS=https://your-domain.com
./gentry
```

### 方式三：独立部署前端

```bash
cd frontend

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build

# 部署 dist 目录到任何静态文件服务器
```

## 开发工作流

### 后端开发

```bash
# 运行后端
go run main.go

# 运行测试
go test ./...
```

### 前端开发

```bash
cd frontend

# 安装依赖
npm install

# 启动开发服务器（自动代理 API 到后端）
npm run dev

# 构建
npm run build
```

### 联调开发

```bash
# 终端 1：启动后端
go run main.go

# 终端 2：启动前端开发服务器
cd frontend
npm run dev

# 前端通过 Vite 代理将 /api 请求转发到后端
```

## API 文档

启动服务后，可通过以下方式获取 API 文档：

1. **OpenAPI 规范**：`api/openapi.yaml`
2. **Swagger UI**：可配合 swagger-ui 使用

## 迁移指南

### 从旧版本迁移

1. **备份数据**：复制 `gentry.db` 到安全位置
2. **部署新版本**：按照新部署方案部署
3. **恢复数据**：将 `gentry.db` 放到新数据目录
4. **验证功能**：检查所有监控器正常运行

### API 兼容性

旧版本使用 `/api` 路径，新版本统一使用 `/api/v1`。如需兼容旧客户端，可添加路由别名：

```go
// 在 routes.go 中添加兼容路由
s.engine.GET("/api/health", s.healthCheck)
```

## 后续工作

- [ ] 添加 API 速率限制
- [ ] 实现 JWT 认证
- [ ] 提供 JavaScript/TypeScript SDK
- [ ] 提供 Python SDK
- [ ] 完善单元测试
- [ ] 添加集成测试

---

*实施日期：2026-07-29*
*版本：v1.1.2*
