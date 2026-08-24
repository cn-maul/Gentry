# ── 前端构建阶段 ──
FROM node:22-alpine AS frontend

WORKDIR /web

# 先拷贝依赖清单以利用缓存
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# 构建前端产物
COPY frontend/ ./
RUN npm run build

# ── 后端构建阶段 ──
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 先拷贝依赖文件以利用缓存
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# 复制源码（含 web/dist 占位文件）
COPY backend/ ./

# 用真实前端产物覆盖占位文件，随后编译进二进制
COPY --from=frontend /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gentry .

# ── 运行阶段 ──
FROM alpine:3.18

WORKDIR /app

# 复制二进制文件（前端已嵌入）
COPY --from=builder /app/gentry .

# 创建数据目录
RUN mkdir -p /app/data

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["./gentry"]
