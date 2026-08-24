# Gentry 前端改造计划：Vue 3 → React 19 + TS + Tailwind 4，并嵌入 Go 后端

## 一、技术选型（默认方案）

| 项 | 选择 | 说明 |
|---|---|---|
| 构建 | Vite 7 + @vitejs/plugin-react | 保留现有 dev 代理（5173 → localhost:8080） |
| 框架 | react / react-dom 19，TypeScript 5（strict） | |
| 路由 | react-router 7（BrowserRouter 声明模式） | 与 vue-router 路由表一一对应 |
| 样式 | Tailwind CSS 4（@tailwindcss/vite 插件） | CSS-first 配置，无 tailwind.config |
| 数据层 | 保留 axios，自研 hooks（不加 TanStack Query） | 忠实移植 useResource/useToastMessages，依赖面最小 |
| 测试 | vitest + jsdom + @testing-library/react | 移植现有 4 个纯逻辑测试 |
| 包管理 | npm（删除 pnpm-lock.yaml / pnpm-workspace.yaml，文档统一为 npm） | dev 脚本本就用 npm |

## 二、前端重写（frontend/ 全量替换，删除所有 .vue 与 style.css）

**文件映射（功能等价移植，保留全部中文文案与行为）：**

- 入口：`index.html`（#root + /src/main.tsx）、`src/main.tsx`、`src/App.tsx`（AppLayout + Outlet）
- 路由 `src/router.tsx`：`/`(Dashboard)、`/add`、`/edit/:name`、`/monitor/:name`、`/push`、`/scan-rules`、`/settings`（与现有完全一致）
- API 层：`src/api/client.ts`（axios 实例、Bearer 拦截器、`{code,message,data}` 信封处理）、`src/api/monitors.ts`（全部端点函数加类型）、`src/api/types.ts`（按 backend/api/openapi.yaml 手写 Monitor/UpdateRecord/ScanRule/NotifyAccount/Stats 等接口）
- 逻辑层：`src/lib/monitorForm.ts`（原 useMonitorForm 纯函数）、`src/lib/priceScanRuleBuilder.ts`、`src/hooks/useResource.ts`（含竞态序号保护）、`src/hooks/useToastMessages.ts`（成功 3s/错误 5s 自动消失）
- 布局：`components/AppLayout.tsx`（侧边栏导航、主题切换写 `.dark` 到 `<html>` + localStorage、stats 15s 轮询、UpdatePanel）
- 视图 6 个：Dashboard / AddMonitor / MonitorDetail / PushManagement / ScanRuleManagement / Settings
- 组件 15 个：MonitorCard、StatusBadge、UpdatePanel、UpdateTable，及 monitor/form/ 下 MonitorForm、BasicMonitorForm、ExtractionEditor、PriceExtractionEditor、MonitorFormSummary、MonitorTypeSelector、MonitorValidationPanel、NotificationEditor、NumericTransitionRuleEditor、ThresholdEditor
- **删除死代码**：FieldEditor.vue、PresenceRuleEditor.vue、IdentityFieldEditor.vue（当前无引用）及未使用的 API 函数（fetchGroups/smartCreate/healthCheck/createScanRule/testScanRule）
- UI 模式保持：手写 modal（遮罩点击关闭）、toast、confirm/prompt 原生对话框、`<details>` 高级设置、骨架屏、分页、轮询

**Tailwind 4 样式方案（src/index.css）：**

- `@import "tailwindcss"` + `@custom-variant dark`（class 策略，沿用 `.dark`）
- `@theme` 移植 style.css 的设计令牌：品牌绿 `#1db954`、表面色阶、语义色（error/warning/success）、圆角（pill 等）、字体栈
- `@layer components` 复刻全局工具类 `.btn`/`.btn-primary`/`.form-input`/`.data-table`/`.modal-*`/`.toast`/`.toggle` 等，组件内其余样式用原子类 + 深色变体重写
- 保留 768px/900px/1100px 响应式断点行为

**测试移植（tests/*.test.ts）：** monitorForm、priceScanRuleBuilder 直接移植；useResource（含竞态用例）、useToastMessages（fake timers）用 renderHook + waitFor 重写。

## 三、后端嵌入前端产物

- 新增 `backend/web/embed.go`：
  - `//go:embed all:dist`（`backend/web/dist/`）
  - `s.engine.NoRoute(...)` SPA 处理器：`/api/` 前缀 → 返回 JSON 404（保持 API 语义）；其余 GET/HEAD → 在嵌入 FS 中查找文件，命中 `assets/*` 设置 1 年 immutable 缓存，未命中或 `/` 回退 `index.html`（no-cache），按扩展名设置 Content-Type
- 提交占位文件 `backend/web/dist/index.html`（提示"请先构建前端"），保证未构建前端时 `go build` 也能通过；`.gitignore` 增加 `backend/web/dist/*` + `!index.html`
- `main.go`：dbPath 支持 `DB_PATH` 环境变量（修复 compose 卷挂载不生效的问题，一行改动）
- `backend/Makefile`：`build` = 构建前端 → 拷贝 `../frontend/dist` → `web/dist/` → `go build`；新增 `build-go`（仅后端，用占位文件）；`release` 前置前端构建
- `backend/Dockerfile` 改为三阶段：node:22-alpine 构建前端 → golang:1.26-alpine 拷入 dist 并编译 → alpine 运行时
- 根 `docker-compose.yml`：移除 `gentry-web`（nginx）服务，仅保留 backend 暴露 8080
- 删除 `frontend/Dockerfile`、`frontend/nginx.conf`、`frontend/.dockerignore`
- 新增根目录 `build.bat` + `build.ps1`（Windows 一键：npm 构建 → 拷贝 dist → go build）
- 更新 README.md 与 docs/（getting-started/development/deployment）中过时描述：技术栈改为 React 19 + Tailwind 4 + TS，访问地址统一 `http://localhost:8080`，包管理统一 npm

## 四、验证

1. `npm run test`（vitest 全绿）+ `npm run build`（tsc 类型检查 + vite build）
2. `go vet ./... && go test ./...`，拷贝 dist 后 `go build`
3. 启动二进制，curl 验证：`/` 返回 index.html、`/assets/*.js` 命中缓存头、`/settings` 回退 index.html、`/api/v1/health` 正常、`/api/不存在` 返回 JSON 404
4. 用浏览器打开 http://localhost:8080 冒烟验证各页面渲染与深色模式

## 风险与说明

- 工作量约 30+ 个新文件，逐组件忠实移植；ScanRuleManagement（780 行）和 PriceExtractionEditor（504 行）是最复杂的两处，会重点核对状态联动逻辑
- 自更新功能下载的二进制体积会因嵌入资源变大，属预期
- CORS 现状（默认 `*`）不动，同源服务后基本不生效
