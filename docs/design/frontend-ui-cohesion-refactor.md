# Gentry 前端界面一致性改造说明书

> 文档状态：已完成源码复审，可作为后续实施与验收依据  
> 审计日期：2026-07-26  
> 审计范围：`frontend/src`、前端构建与测试配置  
> 本文仅定义前端界面与交互改造，不改变监控、扫描规则、推送等业务语义。

---

## 1. 文档目的

Gentry 当前前端的主要页面结构已经成形，单独查看多数页面也能正常使用。界面的割裂感不是由某一个页面造成，而是因为后续功能迭代中，各页面分别实现了自己的容器宽度、按钮、卡片、颜色、异步反馈和响应式规则。

本说明书用于：

1. 审计此前提出的问题，剔除表述过度或证据不足的结论；
2. 明确真正需要修复的布局、视觉、交互和可访问性问题；
3. 规定统一的页面容器、设计 token 和基础组件；
4. 给出可分批实施的任务、文件影响范围和验收标准；
5. 建立响应式、主题、键盘操作和异步状态的测试矩阵。

---

## 2. 技术现状与约束

前端技术基线来自 `frontend/package.json`：

- Vue 3.5；
- Vue Router 4.5；
- Axios 1.7；
- Vite 6；
- 当前测试使用 Node 原生 test runner；
- 现有测试主要覆盖表单和价格扫描规则的纯逻辑；
- 尚无 Vue 组件测试、浏览器 E2E、视觉回归和自动化可访问性测试。

改造应遵守以下约束：

- 不在同一阶段同时重写所有页面；
- 先修确定性缺陷，再抽象基础组件，最后调整页面结构；
- 每次替换公共样式时至少迁移一个完整业务页面并验证；
- 不为了“统一”抹平合理的页面密度差异；
- 不能让视觉改造改变接口字段、路由地址或业务状态语义；
- 桌面、平板、手机及 light/dark 必须同时验收。

---

## 3. 复审结论摘要

此前审查的大多数核心结论成立，但有两项需要收窄表述：

1. **高级规则页并非完全没有响应式。** 页面已有 900px 和 640px 断点；真实问题是这些断点与 `AppLayout` 的 1100px/768px 断点不一致，并且按 viewport 而不是实际内容宽度计算，导致平板区间仍有拥挤风险。
2. **详情页并非所有尺寸都固定双栏。** 768px 以下已经转为单栏；真实问题集中在 769px 以上、但扣除左导航后实际内容很窄的窗口和平板场景。

其余核心结论均有直接源码证据，包括：

- 移动端隐藏设置导航；
- AppLayout 使用固定侧栏和 margin 补偿维持布局；
- 页面容器宽度与标题位置漂移；
- button、icon button、card、input 和状态颜色分裂；
- scoped dark 选择器可能无法命中根节点；
- Toast 定时器状态错误；
- 暂停成功使用错误提示；
- 单条操作触发整页刷新；
- 部分请求错误被静默吞掉；
- 筛选能力缺失；
- `accountDirty` 没有界面表达。

---

## 4. 审计结果明细

### 4.1 已确认问题

| 编号 | 级别 | 审计结论 | 源码证据 | 改造方向 |
| --- | --- | --- | --- | --- |
| A01 | P0 | 移动端第 5 个导航项被隐藏，设置入口消失 | `frontend/src/components/AppLayout.vue:18-38,568` | 删除依赖 DOM 顺序的 `nth-child` 隐藏策略，提供完整底栏、横向滚动或“更多”菜单 |
| A02 | P1 | 移动端同时缺少主题和更新入口 | `frontend/src/components/AppLayout.vue:40-47,537-539` | 在移动端“更多”菜单或设置页中保留主题与更新入口 |
| A03 | P1 | 桌面外壳依赖 fixed + margin 补偿 | `frontend/src/components/AppLayout.vue:223-235,303-326` | 使用统一 CSS Grid 描述左栏、内容列、右栏 |
| A04 | P1 | 页面容器宽度各自定义且内容轴线漂移 | `AppLayout.vue:303-310`、`AddMonitor.vue:300`、`Settings.vue:59`、`ScanRuleManagement.vue:639` | 建立 PageShell 宽度等级，不允许页面自由定义 max-width |
| A05 | P1 | 页面标题和返回入口布局不统一 | `frontend/src/style.css:547-564`、`AddMonitor.vue:301-303`、`MonitorDetail.vue:533-535`、`PushManagement.vue:357-359`、`ScanRuleManagement.vue:640-642` | 建立 PageHeader，统一 back/title/description/actions |
| A06 | P1 | 按钮及图标按钮存在多套实现 | `style.css:129-226`、`MonitorCard.vue:217-272`、`MonitorDetail.vue:536-546`、`PushManagement.vue:430-443`、`ScanRuleManagement.vue:746-748` | 统一 Button 和 IconButton variant、尺寸与状态 |
| A07 | P1 | 卡片、面板和列表行没有明确层级规范 | `style.css:376-381`、`MonitorCard.vue:77-87`、`PushManagement.vue:360-365`、`ScanRuleManagement.vue:666-669` | 定义 Card 的 surface、outlined、interactive 变体 |
| A08 | P1 | 输入控件几何语言不一致 | `frontend/src/style.css:316-337,505-518` 及规则页局部控件 | 统一控件高度、圆角、边框、focus 与 disabled 状态 |
| A09 | P1 | 状态色大量绕过语义 token | `AppLayout.vue:470`、`MonitorCard.vue:93-133`、`MonitorDetail.vue:577-622`、`PushManagement.vue:394-396`、`ScanRuleManagement.vue:606-678` | 建立 primary/info/success/warning/danger 语义色阶 |
| A10 | P1 | `--accent` 被使用但没有定义 | `ScanRuleManagement.vue:678`、`components/monitor/form/ExtractionEditor.vue:352` | 替换为明确语义 token，并增加静态检查 |
| A11 | P1 | scoped dark 祖先选择器可能失效 | `MonitorCard.vue:131-134`、`components/monitor/form/MonitorFormSummary.vue:174` | 优先改成主题 token；必要时使用 `:global(.dark)` |
| A12 | P1 | 全局 motion token 使用 `transition: all` | `frontend/src/style.css:54` 及多个组件 | 按 color、opacity、transform 分离 transition token |
| A13 | P1 | 没有统一 `prefers-reduced-motion` 降级 | 全局样式与组件未形成统一规则 | reduced motion 下移除非必要位移和缩放，仅保留必要颜色/透明度反馈 |
| A14 | P1 | 暂停状态将整卡透明度降至 0.55 | `MonitorCard.vue:1-2,101-103` | 仅调整次要内容或状态样式，主要文本和操作保持正常对比度 |
| A15 | P1 | 编辑和删除操作在部分输入设备上不可发现 | `MonitorCard.vue:252-278` | 默认低强调可见，或用 `focus-within` 和 pointer media query 控制 |
| A16 | P1 | 缺少统一键盘焦点样式 | `style.css:173-225`、`MonitorCard.vue:236-272`、`ScanRuleManagement.vue:653-748` | 所有交互控件统一 `:focus-visible` ring |
| A17 | P1 | 原生 confirm/prompt 与自定义 Modal 混用 | `AddMonitor.vue:258-263`、`MonitorDetail.vue:441-443`、`ScanRuleManagement.vue:521-523` 及三个自定义删除框 | 建立 Dialog、ConfirmDialog、FormDialog |
| A18 | P1 | 现有 Modal 只有视觉层，缺少完整对话框行为 | `style.css:401-463` 及相关页面模板 | 增加 dialog 语义、焦点陷阱、Escape、初始焦点和焦点恢复 |
| A19 | P0 | 暂停成功调用 `showError` | `MonitorDetail.vue:355-366` | 改为 success 或 info，并加入回归测试 |
| A20 | P0 | success/error 共用 Timer，消息可能永久残留 | `frontend/src/composables/useToastMessages.js:3-21` | 短期分离 timer；目标形态为单一 ToastHost 队列 |
| A21 | P1 | 单条启停/删除触发 Dashboard 全列表 loading | `Dashboard.vue:115-163` | 维护目标项 pending，局部更新或后台 revalidate |
| A22 | P1 | 详情页局部操作会重新请求多组无关资源 | `MonitorDetail.vue:284-310,355-365,441-480` | 将 monitor、accounts、updates、events、snapshots 拆成独立资源状态 |
| A23 | P1 | 多处请求失败被静默处理成空数据 | `Settings.vue:34-40`、`AddMonitor.vue:172-176`、`MonitorDetail.vue:287-346,455-463`、`PushManagement.vue:257-264` | 区分 loading、error、empty；非关键降级也应可观察 |
| A24 | P1 | `accountDirty` 已记录但未呈现 | `MonitorDetail.vue:139-142,267,394-410` | 未修改禁用保存；修改后显示未保存；失败保留 dirty |
| A25 | P2 | Dashboard 与规则库缺少搜索和筛选 | `Dashboard.vue:61-111`、`ScanRuleManagement.vue:246-280` | 增加 FilterBar，并将状态同步至 URL query |
| A26 | P2 | 空态使用平台 emoji | `Dashboard.vue:21,28`、`PushManagement.vue:22`、`MonitorDetail.vue:26`、`style.css:258-262` | 使用统一 SVG 图标和 EmptyState/ErrorState |
| A27 | P1 | toggle 使用 `display:none` 隐藏 input | `frontend/src/style.css:520-544` | 使用 visually-hidden，保留键盘和辅助技术交互 |
| A28 | P2 | 规则页 tab 只有部分 ARIA，缺少标准键盘行为 | `ScanRuleManagement.vue:29-32` | 完整实现 tab pattern，或降级为普通 segmented buttons |
| A29 | P2 | 主题在 AppLayout mounted 后初始化，暗色首屏可能闪白 | `AppLayout.vue:172-175,200-203` | 在入口早期初始化主题，支持 system/light/dark |
| A30 | P2 | “最近更新”定时器给 ref 赋相同值，无法可靠触发更新 | `AppLayout.vue:205` | 维护独立 `now` ref 或删除无效 timer |
| A31 | P1 | 列表和详情对 monitor 状态判定顺序不同 | `MonitorDetail.vue:54`、`MonitorCard.vue:61-64` | 抽取 `deriveMonitorStatus()` |
| A32 | P1 | mutation 未统一处理 HTTP 200 + 非零业务码 | Dashboard、MonitorDetail 多处 mutation 与 query 的处理方式不一致 | API client 层统一将非零业务码转成异常 |

### 4.2 部分成立、需要准确描述的问题

| 编号 | 复审后的准确结论 | 处理要求 |
| --- | --- | --- |
| P01 | 高级规则页已有 900px/640px 响应式，但与 AppLayout 的 1100px/768px 断点不协调；769–900px 的实际内容宽度仍可能拥挤 | 不应简单“补一个 media query”；应按内容容器宽度决定列数，并重点测试 768、820、1024px |
| P02 | 详情页在 768px 以下已转单栏，但 769px 以上仍固定保留 240px 右栏；扣除 200px 左栏后，窄平板和窗口模式可能挤压主内容 | 使用 container query 或更高的内容宽度阈值折叠，不继续依赖 viewport 768px |
| P03 | MonitorCard 手机断点已让操作按钮可见，但键盘和 768px 以上的无 hover 设备仍有问题 | 使用 `@media (hover: hover) and (pointer: fine)` 限制 hover 隐藏策略，并增加 `:focus-within` |

---

## 5. 目标设计架构

### 5.1 页面外壳

桌面端使用单一 Grid 管理布局，不允许通过页面 margin 补偿侧栏：

```css
.app-shell {
  display: grid;
  grid-template-columns: var(--sidebar-width) minmax(0, 1fr) var(--inspector-width);
  min-height: 100dvh;
}

.app-shell:not(.has-inspector) {
  grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
}
```

要求：

- 左栏、内容、右栏处于同一个布局流；
- Dashboard 是否展示右栏只改变 grid template；
- 不再使用 `margin-left: 200px` 和 `margin-right: 260px`；
- 移动端变为单列内容加底部导航或抽屉；
- 底栏必须能访问所有核心路由及主题/更新入口。

### 5.2 页面容器等级

建立 `PageShell`，只允许以下宽度等级：

| 等级 | 建议上限 | 适用页面 |
| --- | ---: | --- |
| `narrow` | 640px | Settings、轻量配置 |
| `form` | 820px | AddMonitor、EditMonitor |
| `standard` | 960px | MonitorDetail、PushManagement |
| `wide` | 1120px | Dashboard、ScanRuleManagement |
| `fluid` | 可用空间 | 明确需要大表格或密集数据的页面 |

宽度不同是允许的，但必须共享：

- 相同内容左轴；
- 相同顶部间距；
- 相同标题结构；
- 明确的居中或对齐规则；
- 相同的移动端 padding。

### 5.3 PageHeader

统一结构：

```text
[返回/面包屑]
[标题]                         [页面操作]
[说明文本]
```

要求：

- 标题不再依靠页面局部 `margin-top` 调整；
- back、title、description、actions 均为显式区域；
- 页面无说明文本时不保留幽灵间距；
- H1 每页只能有一个；
- 标题与首个 section 的距离统一。

### 5.4 响应式策略

不继续增加彼此独立的 viewport 断点。目标策略：

1. App shell 只负责全局导航形态；
2. 页面内部多列布局优先用 container query；
3. 组件根据自己的可用宽度降级，而不是猜测 viewport；
4. 需要保留的全局断点集中定义，不在页面内散落数字；
5. 重点保障 769–1024px 的中间区间。

建议的全局层级可为：

- compact：小于 640px；
- medium：640–899px；
- expanded：900–1199px；
- wide：1200px 及以上。

具体数值应在实际截图验收后确定，不能只按代码推断。

---

## 6. 设计系统规范

### 6.1 Token 分层

建议将 token 分为三层：

1. **基础值**：色值、间距、圆角、字体、时长；
2. **语义值**：surface、text、border、primary、danger 等；
3. **组件值**：button height、field radius、card padding 等。

最少需要补齐：

```css
/* surfaces */
--color-canvas;
--color-surface;
--color-surface-raised;
--color-surface-subtle;

/* content */
--color-text-primary;
--color-text-secondary;
--color-text-muted;
--color-border;
--color-border-strong;

/* semantic states */
--color-primary;
--color-primary-hover;
--color-info;
--color-info-surface;
--color-success;
--color-success-surface;
--color-warning;
--color-warning-surface;
--color-danger;
--color-danger-surface;

/* controls */
--control-height-sm;
--control-height-md;
--control-height-lg;
--focus-ring-color;
--focus-ring-width;
--focus-ring-offset;

/* motion */
--duration-fast;
--duration-normal;
--ease-out-ui;
--ease-in-out-ui;
```

禁止事项：

- 页面中直接硬编码状态色；
- 未定义的 CSS 变量；
- 用品牌色同时表达 success；
- 继续使用单一 `transition: all` token。

### 6.2 Button

统一 variant：

- `primary`：页面首要操作；
- `secondary`：普通操作；
- `ghost`：低强调操作；
- `danger`：删除等不可逆操作；
- `icon`：只有图标的操作。

统一状态：

- default；
- hover，仅在支持 hover 的设备应用；
- active，建议使用轻微 `scale(0.97)`，时长 100–160ms；
- focus-visible；
- disabled；
- pending，保留原尺寸避免布局跳动。

要求：

- icon-only 按钮必须有可访问名称；
- 命中区域建议不小于 40×40 CSS px；
- 编辑按钮不能使用 danger hover；
- 不通过完全透明隐藏可聚焦操作。

### 6.3 Card

建议 variant：

- `surface`：普通内容分区；
- `outlined`：需要边界的独立实体；
- `interactive`：可点击或带行操作；
- `danger`：只用于明确错误/危险上下文。

规范：

- light 模式下卡片必须能与 canvas 区分；
- 边框和阴影不能在同一层级中随机混用；
- paused 是业务状态，不把整个 card 当作 disabled；
- 卡片内边距使用固定等级，不在每页微调 2–4px。

### 6.4 表单控件

TextField、Select、Textarea 和 segmented control 应统一：

- 控件高度；
- border 和 focus ring；
- label、description、error 的排版；
- disabled 和 readonly 的差异；
- light/dark 对比度；
- pending 时的行为。

Textarea 可以采用不同高度和适度不同圆角，但必须是定义过的组件变体，不能临时覆盖。

### 6.5 Badge 与状态

监控状态应由共享函数派生：

```js
deriveMonitorStatus(monitor)
```

建议优先级由产品语义明确后固化，例如：

1. error；
2. running；
3. stopped。

列表和详情必须使用同一逻辑、同一文案和同一颜色。`success` 颜色只表示成功/健康状态，品牌主色仅用于主要操作和品牌识别。

### 6.6 Empty / Error / Loading

统一组件应覆盖：

- 首次加载；
- 请求失败；
- 真正空数据；
- 筛选无结果；
- 后台刷新失败但保留旧数据；
- 局部区域重试。

空态不得继续依赖 emoji。装饰 SVG 应设置 `aria-hidden="true"`。

### 6.7 Motion

| Before | After | Why |
| --- | --- | --- |
| `transition: all ...` | 明确列出 `color`、`background-color`、`border-color`、`opacity` 或 `transform` | 避免意外动画布局属性 |
| 所有设备都使用 hover 动效 | 仅在 `@media (hover: hover) and (pointer: fine)` 下启用 | 防止触屏伪 hover |
| 没有 reduced motion | 在 `prefers-reduced-motion: reduce` 下去除非必要位移和缩放 | 避免运动不适并提高可访问性 |
| 快速操作触发整页 loading | 保留页面，只在目标控件显示 pending | 改善感知性能和操作连续性 |

高频键盘操作不增加开合动画。Modal、Toast 等偶发元素的过渡应控制在约 150–250ms，并使用快速响应的 ease-out。

---

## 7. 交互与异步状态规范

### 7.1 Toast

短期修复：

- success 和 error 不再共用会互相取消的 timer；
- 展示一种消息时明确决定是否清理另一种；
- 组件卸载时清理所有 timer。

目标实现：

```ts
{ id, type, message, duration, action? }
```

由全局 `ToastHost` 管理队列和生命周期。至少支持 success、info、warning、error。

### 7.2 Dialog

所有删除、重置、命名输入统一使用 Dialog 系列组件，禁止新增 `window.confirm`、`window.prompt`。

必须支持：

- `role="dialog"`；
- `aria-modal="true"`；
- 可访问标题；
- 打开后设置合理初始焦点；
- Tab 焦点不逃逸；
- Escape 关闭，危险提交过程中可按产品规则限制；
- 关闭后恢复触发元素焦点；
- 提交中禁用重复操作；
- 请求失败时保持弹窗并允许重试。

### 7.3 数据资源状态

每个资源区块使用独立状态：

```ts
{
  data,
  loading,
  refreshing,
  error,
  updatedAt
}
```

要求：

- 初次 loading 可以显示 skeleton；
- refreshing 时保留旧数据；
- error 不得伪装成 empty；
- 后台刷新失败时显示 stale 提示；
- 分页按钮在对应请求 pending 时禁用；
- 并发请求需要避免旧响应覆盖新响应。

### 7.4 Mutation

启停、删除、保存账户等操作使用目标级 pending：

- 只禁用当前实体相关操作；
- 保持列表和滚动位置；
- 成功后局部更新或后台 revalidate；
- 失败后回滚并保留上下文；
- API client 统一将 HTTP 200 + 非零业务码转换为异常。

### 7.5 Dirty state

账户选择等可编辑区域必须明确区分：

- clean；
- dirty；
- saving；
- save failed；
- saved。

`accountDirty` 的要求：

- 初始 clean，保存按钮禁用；
- 修改后显示“有未保存更改”；
- 改回原值时恢复 clean；
- 保存成功清除 dirty；
- 保存失败保留 dirty；
- 离开页面是否阻止，由产品决策明确，不能保持当前无提示状态。

---

## 8. 分阶段实施计划

## Phase 0：建立测试基线

### 目标

先保护确定性行为，再修改公共样式和组件。

### 工作项

1. 保留现有 Node 逻辑测试；
2. 引入 Vitest 和 Vue Test Utils；
3. 引入 Playwright，初期至少覆盖 Chromium；
4. 为以下问题先写失败测试：
   - success/error 连续触发；
   - 移动端设置入口可达；
   - 暂停成功提示类型；
   - `accountDirty` 保存状态；
   - Dialog Escape 和焦点恢复；
5. 建立关键 viewport 截图基线。

### 预计文件

- `frontend/package.json`；
- 新增测试配置；
- `frontend/tests`；
- 新增组件测试和 E2E 目录。

### 完成标准

- 现有测试继续通过；
- 新增 component、e2e 命令；
- CI 能捕获移动导航、Toast、dirty state 回归。

---

## Phase 1：修复确定性缺陷与可达性

### 目标

不进行大规模视觉重构，先修复功能和可访问性问题。

### 工作项

1. 将暂停成功反馈改为 success/info；
2. 修复 Toast timer；
3. 让 `accountDirty` 驱动按钮和提示；
4. 删除移动端 `nth-child(n+5)`；
5. 为移动端提供设置、主题和更新入口；
6. 修复 scoped dark selector；
7. 替换未定义的 `--accent`；
8. 取消 stopped 整卡 opacity；
9. 修复 hover-only 操作；
10. 添加全局 focus-visible；
11. 修复 toggle 的隐藏方式。

### 预计文件

- `frontend/src/composables/useToastMessages.js`；
- `frontend/src/views/MonitorDetail.vue`；
- `frontend/src/components/AppLayout.vue`；
- `frontend/src/components/MonitorCard.vue`；
- `frontend/src/components/monitor/form/MonitorFormSummary.vue`；
- `frontend/src/components/monitor/form/ExtractionEditor.vue`；
- `frontend/src/views/ScanRuleManagement.vue`；
- `frontend/src/style.css`。

### 完成标准

- 手机可访问全部核心入口；
- 暂停操作不再显示错误样式；
- Toast 交错触发不会残留；
- 键盘和无 hover 设备可发现卡片操作；
- stopped 卡片主要内容保持正常对比度；
- dark badge 样式可验证生效。

---

## Phase 2：统一 token 与基础组件

### 目标

收口视觉语言，减少页面局部 CSS。

### 工作项

1. 重构颜色、surface、spacing、radius、control、focus、motion token；
2. 建立 Button、IconButton、Card、Field、Badge；
3. 建立 EmptyState、ErrorState、ToastHost；
4. 建立 Dialog、ConfirmDialog、FormDialog；
5. 删除页面重复的 `.circle-btn`、`.icon-btn`、`.icon-button`；
6. 替换 emoji；
7. 清理硬编码状态色；
8. 删除 `transition: all` 并增加 reduced motion。

### 预计文件

- `frontend/src/style.css`；
- `frontend/src/components/*.vue`；
- `frontend/src/components/monitor/form/*.vue`；
- `frontend/src/views/*.vue`；
- 新增基础组件目录。

### 完成标准

- 同一 variant 的按钮跨页尺寸和状态一致；
- icon button 危险色只用于危险操作；
- 不再存在多套同用途 icon button CSS；
- 状态颜色全部来自语义 token；
- 无未定义 CSS 变量；
- 无 `transition: all`；
- 空态不再使用平台 emoji。

---

## Phase 3：重构 AppLayout 与页面容器

### 目标

解决结构性割裂和中间宽度布局问题。

### 工作项

1. AppLayout 改为 CSS Grid；
2. 建立 PageShell 和 PageHeader；
3. 页面迁移到 narrow/form/standard/wide 容器等级；
4. 统一标题、返回、说明和 actions 的位置；
5. 详情页按内容宽度折叠右栏；
6. 规则页按容器宽度调整列数；
7. 收敛全局断点。

### 预计文件

- `frontend/src/components/AppLayout.vue`；
- `frontend/src/style.css`；
- `frontend/src/views/Dashboard.vue`；
- `frontend/src/views/AddMonitor.vue`；
- `frontend/src/views/MonitorDetail.vue`；
- `frontend/src/views/PushManagement.vue`；
- `frontend/src/views/ScanRuleManagement.vue`；
- `frontend/src/views/Settings.vue`；
- 新增 PageShell/PageHeader。

### 完成标准

- 主内容不再有 200px/260px margin 补偿；
- 所有页面标题遵守相同基线规则；
- 769–1024px 下详情区不被右栏挤压；
- 规则页在 768、820、1024px 无非预期横向溢出；
- 页面切换时宽度变化符合已定义容器等级。

---

## Phase 4：统一异步反馈与更新模型

### 目标

消除整页闪烁、重复提交、假空态和页面间状态不一致。

### 工作项

1. Dashboard 使用目标级 pending；
2. 删除成功后本地移除目标，不进入全页 loading；
3. MonitorDetail 拆分 monitor/config/accounts/updates/events/snapshots 资源；
4. 每个区域独立 loading/error/empty/retry；
5. API 层统一业务码处理；
6. 共享 `deriveMonitorStatus()`；
7. 修复 stats 最近更新时间；
8. 统一首次加载和后台刷新表现。

### 预计文件

- `frontend/src/api/monitors.js` 及相关 API 模块；
- `frontend/src/views/Dashboard.vue`；
- `frontend/src/views/MonitorDetail.vue`；
- `frontend/src/components/AppLayout.vue`；
- `frontend/src/components/MonitorCard.vue`；
- 可能新增资源状态 composable。

### 完成标准

- 单条启停不让整个列表消失；
- 操作期间只禁用目标实体；
- updates/events/snapshots 请求失败显示局部错误而非空数据；
- 列表和详情对同一 monitor 显示相同状态；
- 非零业务码不会显示成功反馈；
- 相对更新时间正常刷新。

---

## Phase 5：补足搜索和筛选

### 目标

确保监控和规则数量增长后仍可快速定位。

### 工作项

1. Dashboard 增加关键词、运行状态、监控类型、错误状态和分组筛选；
2. 规则库增加关键词、来源、scope、启用状态筛选；
3. 将筛选同步到 URL query；
4. 区分“从未创建”和“筛选无结果”；
5. 根据数据规模决定分页或虚拟化。

### 完成标准

- 用户可快速定位错误、暂停或特定类型监控；
- 刷新、后退后筛选状态保留；
- 筛选无结果提供清除筛选入口；
- 列表操作不重置筛选条件。

---

## 9. 推荐实施顺序与提交边界

建议按以下顺序执行：

1. 测试基础设施；
2. Toast、暂停反馈、dirty state 等确定性 Bug；
3. 移动导航、焦点、toggle 等可达性；
4. Token 和基础组件；
5. AppLayout、PageShell、PageHeader；
6. Dashboard 与 MonitorDetail 异步状态；
7. ScanRuleManagement 平板布局；
8. 搜索和筛选；
9. 完整视觉回归与可访问性验收。

建议不要把 Phase 2–4 合并成一个巨型提交。更合理的提交边界是：

- `fix(ui): correct feedback and mobile navigation`；
- `refactor(ui): introduce semantic design tokens`；
- `refactor(ui): unify buttons cards and fields`；
- `refactor(layout): migrate app shell to grid`；
- `refactor(ui): unify page shells and headers`；
- `fix(ui): isolate async resource states`；
- `feat(ui): add monitor and rule filters`。

---

## 10. 测试矩阵

### 10.1 Viewport

| 尺寸 | 重点验证 |
| --- | --- |
| 320×568 | 极窄手机、底栏溢出、设置入口 |
| 375×667 | 常规手机、表单单列、Dialog |
| 390×844 | 长屏手机、底栏与内容滚动 |
| 768×1024 | 全局断点临界值 |
| 820×1180 | 高风险区：左栏和页面两列组合 |
| 1024×768 | 横向平板、详情右栏、规则网格 |
| 1280×720 | 常规桌面、右侧统计栏 |
| 1440×900 | 宽屏容器和空白分配 |
| 1920×1080 | max-width 与居中策略 |

### 10.2 输入方式

- 鼠标；
- 纯键盘；
- 触屏/无 hover；
- 200% 浏览器缩放；
- Windows 高对比模式（测试环境允许时）。

### 10.3 主题与动效

- light；
- dark；
- system theme；
- 从 localStorage 恢复 dark 时无明显白屏闪烁；
- `prefers-reduced-motion: reduce`；
- dark 下所有 badge、dialog、toast 和 field 有足够对比度。

### 10.4 导航与布局

- 手机可访问所有核心路由；
- 手机可访问主题和更新入口；
- Dashboard 有/无右栏时内容轴线合理；
- 详情账户为 0、1、很多个时均正常；
- 规则页 769–900px 不产生页面级水平滚动；
- 页面 H1 与首个 section 间距一致。

### 10.5 键盘与 Dialog

- Tab 顺序符合视觉顺序；
- 所有交互控件有 focus-visible；
- 卡片操作在键盘和无 hover 设备可见；
- Dialog 自动聚焦；
- Tab 不逃逸；
- Escape 可关闭；
- 关闭后焦点恢复；
- icon-only 按钮有可访问名称；
- toggle 可用 Space 键切换。

### 10.6 Toast

必须自动化覆盖：

1. success 单独出现并按时消失；
2. error 单独出现并按时消失；
3. success 后立即 error；
4. error 后立即 success；
5. 连续两次同类型消息；
6. 路由离开后无悬挂 timer；
7. 暂停成功不是错误样式。

### 10.7 数据状态

对 monitor、updates、events、snapshots、accounts 分别覆盖：

- loading；
- success with data；
- success empty；
- HTTP error；
- HTTP 200 + `code !== 0`；
- 慢请求；
- 重试；
- 并发请求乱序；
- 后台刷新失败但保留旧数据。

### 10.8 列表操作

- 启动、暂停只更新目标卡片；
- pending 期间只禁用目标操作；
- 失败后回滚并保留列表；
- 删除成功移除目标，失败保留；
- 删除分组最后一项后分组正确消失；
- 筛选状态下操作不重置筛选。

### 10.9 Dirty state

- 初始未修改：保存禁用；
- 修改后：提示出现、保存启用；
- 改回原值：恢复 clean；
- 保存成功：dirty 清除；
- 保存失败：dirty 保留；
- 离开页面：符合明确的产品决策。

---

## 11. 全局验收标准

全部阶段完成后，至少满足：

### 布局

- AppLayout 不再依赖固定侧栏加 margin 补偿；
- 移动端无核心入口丢失；
- 320–1920px 无非预期页面级横向滚动；
- 页面宽度差异来自明确容器等级；
- 中间宽度区间不再是布局盲区。

### 视觉

- Button、IconButton、Card、Field、Badge 只有统一实现；
- 无未登记的硬编码状态色；
- 无未定义 CSS custom property；
- 无 `transition: all`；
- light/dark 下层级与对比度一致；
- 空态不依赖平台 emoji。

### 交互

- 所有 mutation 有目标级 pending 和重复提交保护；
- 请求失败不会伪装成空数据；
- 不再使用原生 confirm/prompt；
- 列表和详情状态语义一致；
- 用户能够识别未保存更改。

### 可访问性

- 所有交互控件可通过键盘到达；
- focus-visible 清晰且不被裁切；
- Dialog 满足基本焦点管理和 ARIA 要求；
- toggle 保留原生 input 可访问性；
- reduced motion 有效；
- 关键文字和状态达到 WCAG AA 对比度目标。

### 测试

- 现有逻辑测试通过；
- 新增组件测试通过；
- 核心 Playwright 流程通过；
- 关键 viewport 截图经过人工或自动视觉回归确认；
- `npm run build` 通过。

---

## 12. 风险与回滚策略

### 主要风险

1. AppLayout 改为 Grid 时影响所有路由；
2. 全局 token 替换可能造成 dark 模式回归；
3. 抽取 Button/Card 后 scoped CSS 优先级发生变化；
4. optimistic update 可能与后台刷新产生竞态；
5. Dialog 焦点管理可能影响现有表单提交；
6. 引入测试工具会增加安装体积和 CI 时间。

### 控制方式

- AppLayout 在独立提交中完成，并保留旧 CSS 便于单提交回滚；
- 每迁移一个基础组件，立即删除同页旧样式并截图验证；
- 先完成 API 业务码统一，再实施 optimistic update；
- dark/light 和 820px viewport 作为每个 UI 提交的固定验收项；
- 不在布局重构提交中同时更改业务请求逻辑；
- 每个 Phase 均保持可独立发布。

---

## 13. 最终判断

Gentry 前端目前不是需要“推倒重做”，而是需要一次系统性收口：

- 外壳需要从 fixed + margin 补偿迁移到统一布局模型；
- 页面需要使用明确的容器和标题原语；
- 颜色、按钮、卡片、输入和状态需要回归语义 token；
- Toast、Dialog、loading、error、empty 和 pending 需要统一状态模型；
- 中间宽度、键盘、无 hover 设备及 dark/reduced-motion 需要成为正式验收维度。

优先级上，应先修 P0/P1 的可达性和反馈错误，再建设基础组件，最后重构外壳和异步数据体验。这样既能快速消除明显缺陷，也能避免在缺少测试保护的情况下进行一次高风险的全量 UI 重写。
