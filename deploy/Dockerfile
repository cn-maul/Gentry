# ================================================================
# Dockerfile — Gentry 部署版（使用预编译二进制）
# ================================================================

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget

# 以非 root 用户运行
RUN adduser -D -h /app gentry

# 提前创建数据目录并授权
RUN mkdir -p /app/data && chown gentry:gentry /app/data

# 设置工作目录
WORKDIR /app/data

# 复制预编译的二进制
COPY gentry-linux-amd64 /app/gentry
RUN chmod +x /app/gentry

EXPOSE 8889

# SQLite 数据库持久化
VOLUME /app/data

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:8889/api/health || exit 1

USER gentry

ENV GIN_MODE=release \
    PORT=8889

ENTRYPOINT ["/app/gentry"]