# 开发指南

## 环境

- Go 1.26
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

修改监控规则、快照或通知投递逻辑时，应同时覆盖首次基线、幂等去重、异常价格、币种边界、通知终态和配置编辑回填。

## 核心链路

```text
网页抓取
  → URL 范围匹配与扫描规则候选
  → CSS 字段提取
  → 类型规范化
  → 健康检查
  → 加载上次快照
  → Detector 比较
  → 事务保存快照、事件和投递任务
  → Delivery worker 异步通知
```

## 目录结构

```text
database/   SQLite 模型、迁移和仓储
fetcher/    HTTP 抓取
monitor/    提取、规范化、Detector、快照、事件和投递服务
notify/     PushPlus、Webhook、Server酱等通知实现
web/        Gin API、配置验证、嵌入式前端静态资源服务（web/dist 由构建脚本生成）
frontend/   React 19 + TypeScript + Tailwind CSS 4 管理界面
docs/       使用、部署、API、开发和设计文档
main.go     应用入口与服务生命周期
```

## 监控策略

- `presence`：检测稳定身份的新条目。
- `field_transition`：比较同一条目的结构化字段，目前用于价格下降和到价提醒。

策略定义、字段类型和身份配置由前后端共同校验。涉及检测语义的配置变化会递增配置版本并重建基线。

## 数据模型

核心持久化对象包括：

- `Site`、`SiteField`：监控定义；
- `MonitorSnapshot`：每个稳定条目的当前比较基线；
- `MonitorEvent`：检测到的变化事件；
- `NotificationDelivery`：每个事件对应的异步通知任务；
- `ScanRuleTemplate`、`ScanRuleField`：预扫描规则、适用范围和字段配置；
- `UpdateRecord`：旧版新增监控兼容记录。

## 扫描规则范围

数据库规则支持 `exact`、`route` 和 `global` 三种范围。范围字段由服务端根据来源 URL 规范化生成，前端不直接提交 `match_host`、`match_path` 或 `match_query`。`ScopeType` 为空的历史记录回退到 `URLContains` 子串匹配。

修改范围匹配时至少应覆盖以下测试：主机隔离、路径段边界、查询参数顺序、查询参数值差异、根路由保护、通用结构规则和旧版规则兼容。

详细的引擎设计和历史审查记录位于[设计档案](design/)。
