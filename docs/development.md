# 开发指南

## 环境

- Go 1.27
- Node.js 22 或兼容版本
- npm（前端依赖与构建统一使用 npm）
- SQLite 由 Go 驱动内置，不需要单独安装数据库服务

## 启动开发环境

后端（在 `backend/` 目录下）：

```bash
make dev
```

前端：

```bash
cd frontend
npm install
npm run dev
```

后端默认监听 `8080`，Vite 开发服务器默认监听 `5173`。

## 构建

在 `backend/` 目录下执行：

```bash
make build
```

构建顺序是：安装前端依赖、生成 `frontend/dist`、把产物拷贝到 `backend/web/dist`、编译 Go 二进制并嵌入前端资源。未构建前端时也可以 `make build-go`，此时二进制只包含占位页面。

只构建前端：

```bash
cd frontend
npm run build
```

## 测试与静态检查

```bash
go test ./...
go vet ./...
cd frontend
npm test
npm run build
```

修改监控规则或通知逻辑时，应同时覆盖首次基线、幂等去重和配置编辑回填。

## 核心链路

```text
Monitor.CheckNow()
  → Fetcher 抓取（HTML 页面或公开 JSON API）
  → Extractor 按保存的选择器提取字段
  → compareResults 与历史记录比对（按「标题 + 链接」去重）
  → saveResults 写入更新历史（update_records）
  → 有新条目时同步发送通知（PushPlus / Webhook / Server酱 / Bark）
```

首次成功检查只建立基线，不推送页面已有内容；通知随检查循环同步发送。

## 目录结构

```text
database/   SQLite 模型、迁移和仓储
fetcher/    HTTP 抓取
monitor/    提取、配置校验、扫描器、规则库匹配和监控循环
notify/     PushPlus、Webhook、Server酱、Bark 等通知实现
web/        Gin API、配置验证、嵌入式前端静态资源服务（web/dist 由构建脚本生成）
frontend/   React 19 + TypeScript + Tailwind CSS 4 管理界面
docs/       使用、部署、API、开发和设计文档
main.go     应用入口与服务生命周期
```

## 数据模型

核心持久化对象包括：

- `Site`、`SiteField`：监控定义；
- `UpdateRecord`：更新历史，同时作为新增检测的去重基线；
- `NotificationAccount`：推送账户；
- `ScanRuleTemplate`、`ScanRuleField`：扫描规则、适用范围和字段配置；
- `SystemSetting`：系统设置键值对。

数据库由 GORM 在启动时自动迁移；旧版价格监控（field_transition）站点及其关联数据会在启动时一次性清理。

## 扫描规则范围

数据库规则支持 `exact`、`route`、`section` 和 `global` 四种范围。范围字段由服务端根据来源 URL 规范化生成，前端不直接提交 `match_host`、`match_path` 或 `match_query`。`ScopeType` 为空的历史记录回退到 `URLContains` 子串匹配。

修改范围匹配时至少应覆盖以下测试：主机隔离、路径段边界、查询参数顺序、查询参数值差异、根路由保护、同站目录规则、通用结构规则和旧版规则兼容。

详细的引擎设计和历史审查记录位于[设计档案](design/)。
