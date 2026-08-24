# 通用网页监控引擎改造方案

## 1. 背景与目标

当前项目已经具备一条完整链路：定时请求网页、按 CSS 选择器提取字段、检测新增内容、写入 SQLite、通过多个通知渠道推送。现在需要增加电商商品价格监控：同一商品再次被抓取时，如果当前价格低于上一次有效价格，则推送降价通知。

本方案不把价格判断硬编码到现有的“新增公告”逻辑中，而是把监控重新抽象为：

> **数据源抓取 → 观测结果提取与规范化 → 状态比较策略 → 变化事件 → 通知投递**

目标如下：

1. 现有公告监控行为保持兼容，升级后无需重新创建已有监控器。
2. 价格下降、库存恢复、数值越过阈值、字段变化等需求可以复用同一个引擎。
3. “当前状态”和“历史事件”分离，允许同一个商品产生多次降价事件。
4. 通知具备可重试、可追踪、幂等能力，不因一次推送失败而丢失事件。
5. 第一阶段明确支持静态 HTML 或可直接请求的 JSON；动态渲染页面通过可插拔抓取器扩展。

非目标：第一阶段不承诺绕过电商登录、验证码、风控或所有 JavaScript 渲染页面，也不自动猜测用户真正关心的 SKU 规格。

---

## 2. 现有实现的边界

### 2.1 当前链路

现有监控器在 `monitor/monitor.go` 中完成以下工作：

1. 使用 `fetcher.Fetch` 获取 HTML。
2. 使用 `Extractor` 根据 `Site.Container`、`Site.Item`、`SiteField` 提取 `[]ExtractResult`。
3. 从 `UpdateRecord` 加载历史的 `title + url` 组合。
4. 使用 `compareResults` 判断当前结果是否为“新条目”。
5. 使用同一张 `UpdateRecord` 表保存历史和去重依据。
6. 直接把新条目转换为通知文本并发送。

这种实现对公告很合适，但价格监控有三个根本差异：

- 商品的身份不会因为价格改变而改变，不能用“当前是否已存在于历史记录”判断变化。
- 价格需要解析成数值，并判断方向、幅度和阈值。
- 同一商品可以反复降价，因此历史表不能承担“当前价格状态”的职责。

### 2.2 现有能力可以保留的部分

- `fetcher` 的 HTTP 客户端、超时、User-Agent 和连接复用配置。
- `Extractor` 的容器、条目、文本和属性选择器。
- 监控器的启动、停止、间隔调度和数据库加载机制。
- 通知账户、推送服务注册表和现有 Webhook/PushPlus/Server酱/Bark 实现。
- 现有前端的监控列表、详情、账户选择、运行状态和通知设置。

需要重构的是比较、状态持久化、事件格式化和通知投递之间的边界。

---

## 3. 核心领域模型

建议将“监控器”理解为一个版本化的 `MonitorDefinition`。现有 `Site` 表可以继续作为物理表名，但语义上由三部分组成：

```text
MonitorDefinition
├── SourceConfig       数据源和抓取方式
├── ExtractionSchema   如何从响应中生成观测项
└── DetectionRule      如何比较观测项并生成事件
```

### 3.1 SourceConfig

第一阶段支持：

```json
{
  "url": "https://shop.example.com/item/123",
  "fetch_mode": "http",
  "timeout_seconds": 15,
  "user_agent": "..."
}
```

未来可以增加：

- `api`：请求站点公开或用户配置的 JSON 接口；
- `browser`：通过无头浏览器执行 JavaScript；
- Cookie、请求头、地区或代理配置；
- 站点级限速和失败退避策略。

抓取器统一实现如下能力：

```go
type SourceFetcher interface {
    Fetch(ctx context.Context, source SourceConfig) (FetchedDocument, error)
}
```

现有 `fetcher.Fetch` 可以包装成 `HTTPSourceFetcher`。这样后续支持动态电商页面时只替换数据源适配器，不影响比较和通知。

### 3.2 ExtractionSchema

保留现有 `container`、`item` 和字段选择器，但给字段增加语义信息：

```json
{
  "container": ".product-list",
  "item": ".product-card",
  "fields": [
    {"name": "title", "selector": ".title", "type": "text"},
    {"name": "url", "selector": "a", "type": "attr", "attr": "href"},
    {"name": "price", "selector": ".price", "type": "text", "data_type": "money"}
  ]
}
```

字段建议增加：

- `data_type`：`text`、`url`、`integer`、`decimal`、`money`、`datetime`；
- `normalizer`：例如 `trim`、`currency`、`number`、`canonical_url`；
- `required`：缺少字段时是否使本条观测无效。

现有字段默认 `data_type=text`，旧配置不需要修改。当前 `applyTransform` 可以继续支持展示层变换，但数值监控应使用有类型的规范化器，不要依赖字符串正则后再比较。

提取结果统一转换为观测项：

```go
type Observation struct {
    ItemKey   string                 // 稳定商品/条目标识
    Fields    map[string]TypedValue  // 已规范化字段
    Raw       map[string]interface{} // 原始提取值，便于排查
    SeenAt    time.Time
}
```

对于单商品页面，可以把规范化后的 `source_url` 直接作为身份，不要求页面重复输出商品链接；对于商品列表页，则优先使用每个条目自己的商品 URL 或 SKU。

### 3.3 DetectionRule

不要为每一种业务建立独立的顶层监控器类型。建议先提供两类通用策略：

#### A. `presence`

检测集合中是否出现新的 `ItemKey`。这是现有公告新增监控的通用化版本。

```json
{
  "type": "presence",
  "identity": {"fields": ["url", "title"]},
  "on_first_baseline": "silent"
}
```

可选扩展：`item_removed`、`item_reappeared`，但第一阶段只实现新增。

#### B. `field_transition`

比较同一 `ItemKey` 的一个或多个字段变化，并根据操作符产生事件。价格下降只是其中一种配置：

```json
{
  "type": "field_transition",
  "identity": {"field": "url"},
  "conditions": [
    {
      "field": "price",
      "value_type": "money",
      "operator": "decreased",
      "threshold": {
        "amount": "10.00",
        "percent": "5"
      }
    }
  ],
  "on_first_baseline": "silent"
}
```

后续无需改监控主循环即可增加：

- `increased`：价格上涨；
- `changed`：库存、标题或任意文本改变；
- `crossed_below`：价格跌破目标值；
- `became_available`：库存从无货变为有货。

检测器接口建议为：

```go
type Detector interface {
    Validate(schema ExtractionSchema, config json.RawMessage) error
    Evaluate(previous SnapshotSet, current []Observation) EvaluationResult
}

type EvaluationResult struct {
    NextSnapshots []Snapshot
    Events        []ChangeEvent
}
```

检测器只负责比较，不负责写数据库、不负责发通知。这样可以用同一套事务、重试和测试框架承载所有策略。

---

## 4. 状态、事件和投递模型

### 4.1 Snapshot：当前状态

新增 `monitor_snapshots` 表，保存每个监控器最近一次成功观测到的条目状态：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `site_id` | 监控器 ID |
| `item_key` | 稳定条目标识，`site_id + item_key` 唯一 |
| `payload_json` | 当前规范化后的完整字段 |
| `fingerprint` | 规范化字段哈希，用于快速判断是否变化 |
| `definition_version` | 生成该快照时使用的监控配置版本 |
| `first_seen_at` | 首次观测时间 |
| `last_seen_at` | 最近成功观测时间 |
| `missing_checks` | 连续未出现次数 |
| `created_at` / `updated_at` | 审计字段 |

价格存在 `payload_json.price` 中，或者在实现时增加 `numeric_values_json` 做索引优化。金额比较必须使用定点数或最小货币单位，不能使用 `float64`。

推荐金额规范：内部保存 `currency` 和最小货币单位整数，例如 `CNY 1299.90` 保存为 `currency=CNY`、`minor=129990`；原始文本仍放在 `payload_json`。

### 4.2 ChangeEvent：不可变历史事件

新增 `monitor_events` 表作为事件事实来源：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `site_id` | 监控器 ID |
| `event_type` | `item_added`、`price_dropped` 等 |
| `item_key` | 触发事件的条目标识 |
| `title` / `url` | 展示和兼容字段 |
| `before_json` | 变化前观测，可为空 |
| `after_json` | 变化后观测 |
| `old_value` / `new_value` | 主要变化值的展示字符串 |
| `change_amount` | 定点数形式的变化金额，可为空 |
| `change_percent` | 变化百分比，可为空 |
| `dedupe_key` | 防止同一次检查重复生成事件 |
| `definition_version` | 生成事件时使用的监控配置版本 |
| `occurred_at` | 事件发生时间 |
| `created_at` | 入库时间 |

现有 `UpdateRecord` 不适合作为价格事件表，因为当前通过 `title + url` 去重，同一商品的第二次降价会被跳过。建议：

- 短期保留 `update_records` 作为兼容读模型；
- 新策略只写 `monitor_events`；
- 旧公告事件可以双写，或者通过兼容查询把 `monitor_events` 映射为旧响应格式；
- 前端和 API 稳定后，停止对 `update_records` 的新写入，最后再清理旧表。

### 4.3 NotificationDelivery：投递任务

新增 `notification_deliveries` 表，把“事件发生”和“通知发送”解耦：

| 字段 | 说明 |
| --- | --- |
| `event_id` | 事件 ID |
| `account_id` | 推送账户 ID |
| `status` | `pending`、`sending`、`sent`、`failed` |
| `attempts` | 尝试次数 |
| `next_attempt_at` | 下次重试时间 |
| `last_error` | 最近错误 |
| `sent_at` | 成功时间 |

唯一键为 `event_id + account_id`，保证重启或并发调度不会对同一账户重复发送。现有 `Notified` 字段可以作为兼容聚合值，但不再作为唯一投递状态。

---

## 5. 一次检查的完整流程

```text
调度器触发
  ↓
获取 monitor 级锁，避免同一监控重叠执行
  ↓
SourceFetcher 获取响应（带 context、超时、大小限制）
  ↓
Extractor 生成原始观测项
  ↓
字段规范化 + 必填字段校验 + URL/商品标识解析
  ↓
加载该 monitor 的 SnapshotSet
  ↓
Detector.Evaluate(previous, current)
  ├─ 返回下一批 snapshots
  └─ 返回 ChangeEvents
  ↓
事务提交 snapshots、events、notification_deliveries
  ↓
通知 worker 异步消费 deliveries
  ↓
更新运行状态、指标和日志
```

### 5.1 事务边界

抓取和解析在事务外执行；状态、事件和待发送任务必须在同一个数据库事务中提交。任何解析失败、必填字段缺失或页面结构异常，都不能覆盖上一份有效 Snapshot。

数据库至少需要以下唯一约束：

- `monitor_snapshots(site_id, item_key)`；
- `monitor_events(site_id, dedupe_key)`；
- `notification_deliveries(event_id, account_id)`。

在 SQLite 上建议启用 WAL 和合理的 busy timeout。单个监控仍通过应用层互斥保证串行检查，数据库唯一约束作为最后一道幂等保护。

### 5.2 观测健康门槛

在 Detector 运行前增加健康检查，避免页面结构改变时批量污染状态：

- 当前提取结果为 0、而历史快照非空时，默认视为本次检查失败；
- `item_key` 重复时拒绝提交，避免错误覆盖两个不同商品；
- 必填字段缺失比例超过阈值时拒绝整个批次；
- 结果数量相对上次异常骤降时记录结构变化警告，可配置为暂停提交；
- 某一条价格解析失败时，只保留该商品旧快照，不把无效价格写成 0，也不立即累计为商品消失；
- 只有完整通过健康门槛的观测批次才能推进 missing 状态。

单商品监控默认要求恰好得到一个有效身份和一个有效价格；商品列表监控允许多个条目，但必须满足最低有效条目数或有效比例。

### 5.3 首次基线

默认策略为 `silent`：首次成功抓取只建立快照，不生成事件。提供三种明确配置：

- `silent`：首次只建立基线；
- `notify_current`：首次结果也作为事件发送；
- `require_manual_baseline`：用户点击“建立基线”后才开始比较。

新增或修改商品选择器、身份字段、价格字段时，建议自动把监控置为 `needs_baseline=true`，避免配置变化后把所有当前商品误判为新增或降价。

### 5.4 商品身份

身份字段必须稳定。优先级建议为：

1. 用户显式配置的 `sku`、商品详情 URL 或站点商品 ID；
2. 单商品页面的 `source_url`，或者列表条目中规范化后的 URL；
3. `title + URL`；
4. 只有在用户明确允许时，才对完整字段做哈希。

如果身份字段为空或每次抓取都变化，应拒绝启动或在预览时给出高亮警告。不能默默使用包含价格的整行文本作为身份，否则每次价格变化都会被误判成新商品。

### 5.5 页面暂时缺失

单次抓取中商品消失不应立即删除快照。建议设置 `missing_grace_checks`，连续若干次未出现后才标记为 inactive。这样可以避免分页抖动、短暂风控和页面错误导致商品重新出现时产生大量假新增。

---

## 6. 价格下降规则的具体语义

### 6.1 价格字段

商品监控至少需要以下字段：

- `item_key`：商品或 SKU 的稳定标识；
- `title`：通知标题；
- `url`：跳转链接；
- `price`：当前实际关注的价格。

“划线价”“券后价”“会员价”“最低价”不能自动猜测。UI 应要求用户明确选择一个价格字段，并在预览中展示解析后的数值和货币。

### 6.2 降价判定

设旧价格为 `P0`，新价格为 `P1`：

```text
decrease = P0 - P1
percent = decrease / P0 * 100
```

产生 `price_dropped` 的条件：

```text
P1 < P0
且 decrease >= min_drop_amount（如果配置）
且 percent >= min_drop_percent（如果配置）
```

阈值的组合关系建议为 AND，并在 UI 中明确说明。金额和百分比都未配置时，任何有效降价都会触发。

以下情况不产生降价事件：

- 价格相等；
- 价格上涨；
- 新价格无法解析；
- 币种变化且无法换算；
- 页面只返回“暂无报价”；
- 低于阈值的微小波动。

价格上涨仍然要更新 Snapshot，使下一次比较基于最新有效价格，而不是永远基于历史最低价。

如果用户修改了数据源、选择器、身份字段、比较字段、字段类型或币种，必须增加 `config_version` 并重新建立基线；只修改通知账户、通知开关或检查间隔时不需要重建基线。

### 6.3 事件去重和重复通知

同一商品从 `100 → 90 → 95 → 90` 应产生两次降价事件，而不是只允许一次。建议 `dedupe_key` 包含：

```text
site_id + item_key + before_price + after_price + observation_window
```

同一次检查重试只允许一个事件；不同时间重新降到同一价格，可以根据用户配置选择：

- `every_transition`：每次下降都通知；
- `cooldown`：在指定时间内只通知一次；
- `new_low_only`：只有创历史新低才通知。

第一阶段默认 `every_transition`，并提供可选冷却时间。

---

## 7. 代码分层改造建议

建议按以下包边界改造，避免把业务分支继续堆在 `monitor.go`：

```text
monitor/
  engine.go             调度一次检查、错误处理、运行状态
  source.go             SourceFetcher 接口适配
  extraction.go         页面解析和 Observation 构造
  normalize.go          字段类型、金额、URL 规范化
  identity.go           ItemKey 生成和校验
  detector.go           Detector 接口和公共类型
  detectors/
    presence.go         公告/新增条目
    transition.go       通用字段变化
    price_drop.go       数值下降语义（可作为 transition 的实现）
  state_repository.go   Snapshot 读写
  event_repository.go   ChangeEvent 读写
  delivery_worker.go    通知任务重试
```

现有函数迁移关系：

| 现有逻辑 | 目标位置 |
| --- | --- |
| `CheckForUpdates` | `Engine.CheckOnce` 编排流程 |
| `compareResults` | `PresenceDetector` |
| `saveResults` | `SnapshotRepository` + `EventRepository` |
| `buildNotifyContent` | `EventFormatter` |
| `sendCombinedNotification` | `DeliveryWorker` |
| `Extractor` | `ExtractionSchema` 执行器，保留主体 |

第一阶段不必一次性删除旧函数。可以先让旧 API 调用新引擎，再删除旧的比较和直接发送代码。

### 7.1 运行锁和取消

现有 ticker 模式需要补充：

- 每个 monitor 一个检查互斥锁；
- `context.WithTimeout` 贯穿抓取和解析；
- 停止监控时取消当前请求；
- 抓取失败按错误类型记录，不更新 Snapshot；
- 连续失败采用指数退避，但不改变用户配置的基础检查间隔。

### 7.2 运行状态

`MonitorStatus` 建议增加：

- `monitor_type` / `strategy_type`；
- `last_event_at`；
- `baseline_status`；
- `last_success_at`；
- `consecutive_failures`；
- `active_snapshot_count`。

`updates_count` 继续作为兼容字段，但新界面应区分“事件总数”“待投递数量”和“当前商品数量”。

---

## 8. 数据库迁移与兼容策略

### 阶段 0：增加迁移版本和备份

在自动迁移前增加 schema 版本记录，并在生产部署时备份 SQLite 文件。当前 `AutoMigrate` 可以继续使用，但新增唯一索引和数据回填必须有显式迁移步骤，不能只依赖 GORM 自动推断。

### 阶段 1：兼容字段和新表

向 `sites` 增加：

- `strategy_type`，旧数据默认 `presence`；
- `strategy_config`；
- `fetch_config`；
- `baseline_status`；
- `config_version`。

新增 `monitor_snapshots`、`monitor_events`、`notification_deliveries`。此阶段不删除任何旧列和旧表。

### 阶段 2：旧监控自动转换

对已有 `Site`：

```text
strategy_type = presence
identity = [url, title]
on_first_baseline = silent
```

历史 `UpdateRecord` 无法可靠还原当前页面的完整集合，因此不要把旧历史强行当成完整 Snapshot。推荐设置 `baseline_status=needs_baseline`，首次升级后的成功检查只建立新快照。

旧记录可以按 `event_type=item_added` 映射到新事件查询，但不必伪造 `before_json`。

### 阶段 3：双读、单写

- 新引擎只写 Snapshot/Event/Delivery；
- 旧 `/updates` API 从新事件表读取，并按旧字段格式返回；
- 对未迁移数据必要时回退读取 `update_records`；
- 通知状态由 Delivery 汇总，旧的 `Notified` 只做兼容展示。

### 阶段 4：收敛

运行一个版本周期确认新表数据和通知投递稳定后：

- 停止写 `update_records`；
- 将旧 API 标记为兼容接口；
- 未来再考虑删除旧去重代码和旧表，删除前必须提供导出或迁移工具。

---

## 9. API 设计

### 9.1 新的配置形态

推荐新增配置形态，同时继续接受现有扁平请求：

```json
{
  "name": "商品降价监控",
  "source": {
    "url": "https://shop.example.com/product/123",
    "fetch_mode": "http"
  },
  "extraction": {
    "container": "body",
    "item": ".product-detail",
    "fields": [
      {"name": "title", "selector": ".title", "type": "text"},
      {"name": "price", "selector": ".current-price", "type": "text", "data_type": "money"}
    ]
  },
  "detection": {
    "type": "field_transition",
    "identity": {"source": "source_url"},
    "conditions": [{
      "field": "price",
      "operator": "decreased",
      "threshold": {"percent": "5"}
    }]
  },
  "check_interval": 1800,
  "notify_account_ids": [1],
  "is_active": true
}
```

服务端把旧请求体转换为同一内部结构：

- 旧的 `container/item/fields` → `extraction`；
- 旧监控 → `detection.type=presence`；
- `group`、通知账户和检查间隔保持不变。

### 9.2 预览和校验

保留 `/monitors/preview`，但响应应增加：

- 规范化后的字段值；
- 推断的数据类型；
- 生成的 `item_key`；
- 价格解析结果、货币和解析错误；
- 选择器匹配数量和必填字段缺失警告。

新增或扩展以下接口：

- `POST /monitors/validate`：只验证配置，不写入数据库；
- `POST /monitors/:name/baseline`：手动建立当前基线；
- `GET /monitors/:name/snapshots`：查看当前商品和当前价格；
- `GET /monitors/:name/events`：按事件类型、时间和通知状态查询；
- `POST /monitors/:name/check`：手动触发一次检查，返回评估结果。

现有 `/updates` 可以作为 `/events` 的兼容别名，避免一次性破坏前端和外部调用者。

---

## 10. 前端改造

### 10.1 创建/编辑流程

在现有新增页面增加“检测方式”选择：

- 新增条目；
- 字段变化；
- 价格下降。

价格下降模式显示：

- 商品身份字段；
- 价格字段；
- 货币和价格解析预览；
- 最低降价金额；
- 最低降价百分比；
- 首次基线行为；
- 重复降价通知策略。

字段编辑器保留原来的 CSS 选择器体验，只增加字段类型和“必填”选项。智能扫描可以继续推荐容器和字段，但价格字段、SKU 字段必须由用户确认。

### 10.2 详情页

公告监控继续展示标题、链接和通知状态。价格监控改为展示：

```text
商品 | 当前价格 | 上次价格 | 最近变化 | 最近观测 | 通知状态
```

事件历史展示：

```text
时间 | 商品 | 原价 | 现价 | 降幅 | 通知状态
```

详情页需要同时区分：

- 当前快照数量；
- 历史事件数量；
- 待投递事件数量；
- 最近一次成功抓取时间。

### 10.3 操作反馈

增加“重新建立基线”操作，并在危险操作前说明它会清除比较基准，但不会删除历史事件。手动检查应显示解析失败字段，而不是只显示一个笼统的 HTTP 错误。

---

## 11. 通知设计

通知格式化从监控主循环移到事件格式化器：

```go
type EventFormatter interface {
    Format(event ChangeEvent) (title string, content string)
}
```

价格下降示例：

```text
商品降价：无线耳机

原价：¥1,299.00
现价：¥1,099.00
降价：¥200.00（15.40%）
链接：https://shop.example.com/product/123
```

通知投递规则：

1. 事件入库后创建每个账户的一条 pending delivery。
2. worker 发送成功后标记 `sent`。
3. 失败按照指数退避重试，并记录最近错误。
4. 达到最大次数后标记 `failed`，在 UI 中可手动重试。
5. 账号删除、监控删除和事件删除的关联行为要有明确约束。

现有“全部账户成功后标记 `Notified`”的语义可以保留为聚合展示，但底层应以每个账户的 Delivery 为准。

现有关键词过滤放到 `DeliveryPlanner`：事件始终落库，只有命中关键词的事件才为所选账户创建 Delivery。公告模式默认匹配事件标题；价格模式可选择匹配商品标题或指定字段。这样通知过滤不会影响状态推进，也不会导致下次重复检测同一事件。

---

## 12. 可靠性、安全和站点适配

### 12.1 抓取可靠性

- 请求必须设置 context 和超时；
- 限制响应大小；
- 每个域名控制并发和频率；
- 失败不覆盖有效快照；
- 对 HTTP 403、429、5xx 分别记录并退避；
- 记录响应解析耗时和结果数量；
- 同一监控不允许并发检查。

### 12.2 SSRF 和敏感配置

沿用现有出站 URL 校验，并扩展到：

- 禁止访问本地、内网和云元数据地址；
- 限制重定向目标；
- 用户自定义 Header/Cookie 脱敏存储；
- 日志中不输出 Cookie、Token 和完整授权头；
- Webhook URL 继续进行出站地址校验。

### 12.3 动态电商页面

在配置中显式展示 `fetch_mode`，不要让用户以为普通 HTTP 一定能得到浏览器看到的价格。若页面依赖 JavaScript、登录或地区信息，应返回可诊断提示：当前响应中没有价格字段，建议改用 API 或浏览器抓取模式。

---

## 13. 测试计划

### 13.1 单元测试

- URL、商品 ID 和复合身份生成；
- 金额解析：货币符号、千分位、小数、空值、非法文本；
- 定点金额比较和百分比计算；
- 首次基线静默；
- 价格不变、上涨、下降、低于阈值；
- `100 → 90 → 95 → 90` 的多次事件；
- 页面暂时缺失和恢复；
- 旧公告 `presence` 策略与现有行为一致。

### 13.2 集成测试

- SQLite 迁移和旧数据库启动；
- 状态、事件、Delivery 的事务原子性；
- 重复检查不会生成重复事件；
- 进程重启后继续使用上次 Snapshot；
- 通知失败后的重试和幂等；
- 监控停止时取消 HTTP 请求；
- 旧 API 仍能读取新事件。

### 13.3 端到端测试

使用 `httptest.Server` 模拟商品页面：

1. 第一次返回 `¥100`，确认只建立基线；
2. 第二次返回 `¥90`，确认生成并推送降价事件；
3. 第三次返回 `¥95`，确认更新状态但不推送；
4. 第四次返回 `¥80`，确认再次推送；
5. 返回无效价格，确认保留 `¥80` 状态并报告错误。

---

## 14. 分阶段实施顺序

### Phase 1：抽象而不改变行为

- 定义 `Observation`、`Snapshot`、`ChangeEvent`、`Detector`；
- 把现有 `compareResults` 移到 `PresenceDetector`；
- 新引擎运行公告监控，建立回归测试；
- 通知暂时可以由适配层调用现有实现。

### Phase 2：新状态和事件模型

- 增加 schema 版本和新表；
- 实现 Snapshot/Event repository；
- 旧监控自动转换为 `presence + silent baseline`；
- API 双读、新引擎单写；
- 增加检查锁、context 和失败不覆盖状态。

### Phase 3：字段类型和价格策略

- 增加字段数据类型和规范化器；
- 实现定点金额解析；
- 实现 `field_transition` 和 `price_dropped`；
- 加入阈值、冷却和历史新低规则；
- 加入价格预览与手动建立基线。

### Phase 4：投递队列和 UI

- 引入 NotificationDelivery worker；
- 更新通知格式化、重试和失败重试按钮；
- 前端增加监控类型、当前快照和价格事件历史；
- 保留旧接口兼容。

### Phase 5：抓取器扩展与收敛

- 根据真实目标电商页面决定是否实现 API 或浏览器抓取；
- 增加站点适配器和登录态管理；
- 观察一个完整版本周期后停止写旧 `update_records`；
- 提供旧数据导出和清理工具。

---

## 15. 验收标准

改造完成后至少应满足：

1. 现有公告监控的首次基线、新增检测、关键词过滤和已有通知账户行为不回归。
2. 同一商品价格从 `100` 降到 `90` 能只生成一次降价事件，并包含原价、现价、降幅和链接。
3. 价格上涨不会推送，但会更新当前快照。
4. 同一商品后续再次降价仍能产生新的事件。
5. 页面解析失败不会污染上一次有效价格。
6. 进程重启、通知失败和重复检查不会造成事件丢失或无限重复推送。
7. 动态页面无法通过当前抓取模式获得价格时，用户能在预览或错误信息中明确知道原因。
8. 旧数据库可以自动迁移，旧 API 和已有监控配置仍可使用。

该设计的关键取舍是：把“公告新增”和“价格下降”都定义为对观测状态的不同比较规则。这样价格需求不是一次性特例，今后的库存、目标价、评分和任意字段变化都可以沿用同一套状态、事件和投递基础设施。
