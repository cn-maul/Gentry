.PHONY: all build run dev clean release test

all: build

# 编译后端
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -buildvcs=false -o gentry .
	@echo "✅ 构建完成！运行: ./gentry"

# 本地运行（需先 build）
run:
	./gentry

# 开发模式（启动后端）
dev:
	CGO_ENABLED=0 go run -buildvcs=false .

# 运行测试
test:
	go test ./...

# 跨平台发布
release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -buildvcs=false -o gentry-linux-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -buildvcs=false -o gentry-windows-amd64.exe .
	@echo "✅ 发布包构建完成"

# 清理构建产物
clean:
	rm -f gentry gentry-linux-amd64 gentry-windows-amd64.exe gentry.bak
	rm -f gentry.db gentry.db-shm gentry.db-wal
