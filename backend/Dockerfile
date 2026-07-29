# 构建阶段
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .

# 构建
RUN CGO_ENABLED=0 go build -o gentry .

# 运行阶段
FROM alpine:3.18

WORKDIR /app

# 复制二进制文件
COPY --from=builder /app/gentry .

# 创建数据目录
RUN mkdir -p /app/data

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["./gentry"]
