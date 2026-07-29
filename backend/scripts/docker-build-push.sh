#!/bin/bash
set -e

echo "=== 1. 登录 ghcr.io ==="
echo "$GITHUB_TOKEN" | docker login ghcr.io -u cn-maul --password-stdin

echo "=== 2. 构建镜像 ==="
cd "$(dirname "$0")/.."
docker build -t ghcr.io/cn-maul/gentry:v1.1.0 -f Docker/Dockerfile .
docker tag ghcr.io/cn-maul/gentry:v1.1.0 ghcr.io/cn-maul/gentry:latest

echo "=== 3. 推送到 ghcr.io ==="
docker push ghcr.io/cn-maul/gentry:v1.1.0
docker push ghcr.io/cn-maul/gentry:latest

echo "=== ✅ 完成 ==="