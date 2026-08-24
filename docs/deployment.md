# 部署指南

## Docker Compose

```bash
docker compose up -d --build
```

默认映射端口为 `8080:8080`（单服务：界面与 API 同端口，前端已嵌入二进制），数据库保存在 `gentry-data` 卷中。

查看日志：

```bash
docker compose logs -f gentry-backend
```

升级本地构建版本：

```bash
docker compose build --pull
docker compose up -d
```

## Docker

构建镜像（构建上下文为仓库根目录，Dockerfile 会同时构建前端并嵌入）：

```bash
docker build -f backend/Dockerfile -t gentry:latest .
```

运行：

```bash
docker run -d \
  --name gentry \
  --restart unless-stopped \
  -p 8080:8080 \
  -v gentry-data:/app/data \
  -e TZ=Asia/Shanghai \
  gentry:latest
```

## 本地二进制

生产构建需要先生成 `frontend/dist`，构建脚本会把它拷贝到 `backend/web/dist` 并由 go:embed 嵌入二进制：

```bash
cd backend
make build
./gentry
```

Windows 可使用项目根目录的 `build.bat` / `build.ps1` 一键完成。

本地二进制默认在当前工作目录创建 `gentry.db`（可通过 `DB_PATH` 覆盖）。请在固定目录运行程序并定期备份数据库。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 服务端口 |
| `GIN_MODE` | Docker 中为 `release` | Gin 运行模式 |
| `TZ` | 系统默认时区 | 推荐设置为 `Asia/Shanghai` |
| `DB_PATH` | `gentry.db` | SQLite 数据库文件路径（Docker 中为 `/app/data/gentry.db`） |
| `ALTERBOT_AUTH_TOKEN` | 空 | 可选 API Bearer Token；历史兼容命名 |
| `SCAN_RULES_FILE` | 空 | 可选的旧版启动时外部扫描规则文件路径；与管理界面的规则导出文件不是同一格式 |

设置 `ALTERBOT_AUTH_TOKEN` 后，请求 `/api/v1` 下的接口需要携带：

```text
Authorization: Bearer <token>
```

## 数据备份

核心状态存储在 SQLite 数据库中，包括监控定义、快照、事件、通知账户、通知投递记录和扫描规则库。

备份前建议暂停容器或程序，然后复制数据库文件。Docker 部署可将数据卷导出到宿主机备份系统。

## 健康检查

```bash
curl http://localhost:8080/api/v1/health
```

如果配置了认证令牌，需要增加 `Authorization` 请求头。

## 自动升级与代理

Gentry 支持通过 GitHub Releases 自动升级。侧边栏底部点击版本号检查更新，有新版时点击"升级到"按钮即可自动下载替换并重启。

### 代理设置

如果所在网络无法直接连接 GitHub（如中国大陆），需要在 **设置 → 更新代理** 中填入代理地址，例如：

```
http://127.0.0.1:7897
```

系统会自动处理：
- 检查更新时优先直连 GitHub API，失败则走代理
- 下载更新文件时优先走代理，失败则直连
- 下载中断时会自动断点续传，完成后校验文件大小和可执行文件格式
- Windows 升级后若新进程无法启动，会自动恢复并启动旧版本

代理地址保存在数据库中，重启后仍然生效。

升级会替换当前正在运行的可执行文件。因此，如果从 `gentry-windows-amd64-v1.1.0.exe` 启动，升级后的文件名保持不变，程序界面中的版本号才是实际版本；旧版本保存在同目录的 `.bak` 文件中。
