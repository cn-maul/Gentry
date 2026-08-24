.PHONY: all build build-go embed-frontend run dev clean release test

all: build

# 完整构建：先构建前端并嵌入，再编译后端
build: embed-frontend
	CGO_ENABLED=0 go build -ldflags="-s -w" -buildvcs=false -o gentry .
	@echo "✅ 构建完成！运行: ./gentry"

# 仅构建后端（不嵌入真实前端，访问 / 显示占位页）
build-go:
	CGO_ENABLED=0 go build -ldflags="-s -w" -buildvcs=false -o gentry .
	@echo "✅ 后端构建完成（未嵌入前端产物）"

# 构建前端并把产物拷贝到 web/dist（供 go:embed 嵌入）
embed-frontend:
	npm --prefix ../frontend install
	npm --prefix ../frontend run build
	rm -rf web/dist
	cp -r ../frontend/dist web/dist
	@echo "✅ 前端产物已拷贝到 web/dist"

# 本地运行（需先 build）
run:
	./gentry

# 开发模式（启动后端，前端开发用 frontend/ 下 npm run dev）
dev:
	CGO_ENABLED=0 go run -buildvcs=false .

# 运行测试
test:
	go test ./...

# 跨平台发布（含前端嵌入）
release: embed-frontend
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -buildvcs=false -o gentry-linux-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -buildvcs=false -o gentry-windows-amd64.exe .
	@echo "✅ 发布包构建完成"

# 清理构建产物
clean:
	rm -f gentry gentry-linux-amd64 gentry-windows-amd64.exe gentry.bak
	rm -f gentry.db gentry.db-shm gentry.db-wal
