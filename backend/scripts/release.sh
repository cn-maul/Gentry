#!/bin/bash
set -e

REPO="cn-maul/Gentry"
TAG="v1.0.0"

# 获取 GitHub Token（环境变量，不得硬编码）
if [ -z "$GITHUB_TOKEN" ]; then
  echo "❌ 请设置 GITHUB_TOKEN 环境变量"
  echo "   export GITHUB_TOKEN='你的token'"
  exit 1
fi

# 构建发布包（跨平台）
echo "=== 构建发布包 ==="
cd "$(dirname "$0")/.."
make release

# Create release
echo "=== Creating release ==="
RELEASE_JSON=$(curl -s -X POST "https://api.github.com/repos/$REPO/releases" \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tag_name": "'"$TAG"'",
    "name": "v1.0.0 — 多账户推送 + Server酱 + 关键词过滤",
    "body": "## v1.0.0\n\n### 新增\n- 多账户推送系统：DB 存储推送账户（PushPlus / Webhook / Server酱），每个监控器独立选择启用哪些账户\n- Server酱 推送支持\n- 关键词过滤推送：监控器可设置为仅命中关键词时推送\n- 监控项未读计数标记已读功能\n\n### 修复\n- 编辑监控器时推送账户配置被清空\n- 重启后推送开关状态丢失\n- 切换推送账户不同步运行中监控器\n- 移除了过渡期的 config.json 自动导入\n\n### 编译\n- Linux amd64 / Windows amd64",
    "draft": false,
    "prerelease": false
  }')
RELEASE_ID=$(echo "$RELEASE_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id', 'ERROR'))" 2>/dev/null)
echo "Release ID: $RELEASE_ID"

# Upload assets
for FILE in gentry-linux-amd64 gentry-windows-amd64.exe; do
  if [ ! -f "$FILE" ]; then
    echo "❌ 文件 $FILE 不存在，请确保 make release 已成功执行"
    exit 1
  fi
  BASENAME=$(basename "$FILE")
  echo "=== Uploading $BASENAME ==="
  curl -s -X POST "https://uploads.github.com/repos/$REPO/releases/$RELEASE_ID/assets?name=$BASENAME" \
    -H "Authorization: token $GITHUB_TOKEN" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @"$FILE" > /dev/null
  echo "Uploaded $BASENAME"
done

echo "=== Release published ==="