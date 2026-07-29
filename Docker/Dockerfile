# ================================================================
# Dockerfile — Gentry 多阶段构建
#
# Go 版本注意: go.mod 指定 go 1.26，使用 golang:1.26-alpine 镜像
# 如需其他版本:
#   docker build --build-arg GO_VERSION=1.25-alpine .
# ================================================================

# ---- 构建参数 ----
ARG GO_VERSION=1.26-alpine
ARG NODE_VERSION=22-alpine
ARG ALPINE_VERSION=3.21

# ==============================
# Stage 1: 构建前端
# ==============================
FROM node:${NODE_VERSION} AS frontend-builder

WORKDIR /frontend

# 利用 Docker 缓存层：先复制依赖锁文件 → pnpm install
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN npm install -g pnpm --silent && pnpm install --frozen-lockfile

# 复制源码并构建
COPY frontend/ ./
RUN pnpm run build

# ==============================
# Stage 2: 编译 Go 后端
# ==============================
FROM golang:${GO_VERSION} AS backend-builder

WORKDIR /app

# 基础工具
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 嵌入 Stage 1 的前端产物
COPY --from=frontend-builder /frontend/dist ./frontend/dist

# CGO_ENABLED=0: 纯静态编译，无需 glibc/musl
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gentry .

# ==============================
# Stage 3: 最小运行镜像
# ==============================
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates tzdata wget

# 以非 root 用户运行
RUN adduser -D -h /app gentry

# 提前创建数据目录并授权（重要：VOLUME 会继承这些权限）
RUN mkdir -p /app/data && chown gentry:gentry /app/data

# 设置工作目录为数据卷路径（SQLite 自动建在此处）
WORKDIR /app/data

# 只复制编译好的二进制
COPY --from=backend-builder --chown=gentry:gentry /app/gentry /app/gentry

EXPOSE 8889

# SQLite 数据库持久化（-v gentry-data:/app/data）
VOLUME /app/data

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:8889/api/health || exit 1

USER gentry

ENV GIN_MODE=release \
    PORT=8889

ENTRYPOINT ["/app/gentry"]
