# 通用监控引擎实现审查与修订方案

## 1. 审查结论

当前实现完成了领域类型、新表、检测器骨架、价格事件 UI 和部分 API，整体方向与原设计一致；但目前仍属于“原型接线阶段”，不建议作为可用版本发布。

主要原因不是代码风格，而是以下核心语义尚未闭环：

1. 前端生产构建失败，详情页面无法编译。
2. UI 中配置的价格字段、降价阈值和页面 URL 身份没有被后端正确执行。
3. 快照、事件和投递任务不是事务性提交，错误还会被仓储层吞掉，存在永久漏报。
4. 投递队列没有启动，定时检查又绕过队列直接发送；修一半时还可能产生重复通知。
5. 无效价格会覆盖最后一次有效价格，下一次恢复正常时无法继续比较。
6. 编辑价格监控会丢失策略配置，并可能把它保存回普通新增监控。
7. 事件 API 的 JSON 字段格式与 Vue 页面使用的字段格式不一致。
8. 配置版本、基线状态、手动检查和定时检查之间没有一致的并发及生命周期语义。

后端 `go test ./...` 和 `go vet ./...` 当前通过，但现有测试主要覆盖纯函数，没有覆盖上述端到端路径。前端 `npm run build` 失败，错误为 `MonitorDetail.vue (51:5): Element is missing end tag`。

---

## 2. 必须先修复的 P0 问题

### P0-1：修复前端编译和事件 DTO

`frontend/src/views/MonitorDetail.vue` 的详情布局存在多余的闭合标签，导致 Vue 模板无法编译。先修复模板结构，并把 `npm run build` 加入每次提交的最低验证门槛。

事件接口当前直接返回 `database.MonitorEvent`。Go 默认 JSON 字段是 `ID`、`EventType`、`OldValue`，前端使用的是 `id`、`event_type`、`old_value`，即使模板可以编译，事件表格仍然读不到数据。

修订方式：

- Web 层定义独立 `monitorEventResponse` 和 `monitorSnapshotResponse`；
- 所有响应字段显式使用 snake_case JSON tag；
- 不直接向前端暴露数据库模型和 `BeforeJSON/AfterJSON` 原始字符串；
- 增加 API 响应测试，断言真实 JSON 字段名。

### P0-2：让 DetectionRule 真正参与检测

当前 `FieldTransitionDetector` 固定读取 `obs.Fields["price"]`，没有读取 `DetectionRule.Conditions`。因此用户选择 `current_price`、`sale_price` 等价格字段时不会产生任何降价事件。

当前前端虽然显示“最低降价金额”和“最低降价百分比”，但提交请求时没有把它们写入 `threshold`；后端也没有执行阈值判断。

修订方式：

```go
type FieldTransitionDetector struct {
    rule DetectionRule
}

func NewDetector(rule DetectionRule) (Detector, error)
```

检测器必须：

1. 遍历并验证 `rule.Conditions`；
2. 根据 `condition.Field` 读取目标字段；
3. 根据 `ValueType` 选择比较方式；
4. 执行 `Operator`；
5. 执行 amount、percent 和 cooldown/new-low 等规则；
6. 为每个命中的 Condition 产生清晰的事件负载。

第一版可以只允许一个 `money + decreased` 条件，但必须拒绝其他未实现配置，不能默默忽略。

### P0-3：正确处理字段类型配置和身份字段

`parseFieldDataTypes` 在配置非空时直接返回空 map，等于忽略前端传入的 `field_data_types`。应执行 JSON 解析，并对未知类型返回错误。

页面 URL 身份当前被前端编码成：

```json
{"identity":{"field":"source_url"}}
```

而后端只在 `identity.source == "source_url"` 时使用页面 URL。需要统一协议：

```json
{"identity":{"source":"source_url"}}
```

同时，`Engine.toObservations` 必须把 `site.URL` 传给 `GenerateItemKey`，而不是把条目的 `url` 字段误当成 source URL。

禁止使用 `item_0`、`item_1` 之类的位置索引作为静默 fallback。身份字段缺失或重复时应让本次检查失败，并在预览/校验接口中明确提示，否则商品排序变化会导致不同商品互相比较价格。

### P0-4：无效价格不得推进快照

当前一次无效价格会把已有 Snapshot 更新成 `PriceValid=false`、`PriceMinor=0`。下一次价格恢复正常时，因为旧快照无效，不会再与最后一次有效价格比较，造成漏报。

正确语义：

- 新商品首次价格无效：整条观测无效，不建立价格基线；
- 已有商品本次价格无效：保留旧的价格字段、PriceMinor、Currency 和 PriceValid；
- 可以更新诊断信息和失败计数，但不能覆盖有效比较值；
- 单商品监控没有任何有效价格时，本次检查返回错误；
- 部分列表项失败时，只推进有效条目，并保留失败条目的旧状态。

需要新增 `100 → 无效 → 90` 仍然产生一次降价事件的测试。

### P0-5：把快照、事件和 Delivery 放进同一事务

当前顺序是：

```text
更新基线状态
→ 保存快照
→ 保存事件
→ 创建 Delivery
→ 写兼容 UpdateRecord
```

这些步骤没有事务。更严重的是，`SaveSnapshotsBatch` 和 `SaveEvents` 遇到单条写入错误只记日志，最后仍然返回 nil。

可能出现：快照已更新但事件未保存。下一次检查基于新快照，不会再次产生该降价事件，形成永久漏报。

修订为单一仓储入口：

```go
func PersistEvaluation(
    tx *gorm.DB,
    site Site,
    result EvaluationResult,
    deliveryPlan []DeliveryTarget,
) ([]MonitorEvent, error)
```

事务中完成：

1. upsert 下一状态；
2. 插入幂等事件；
3. 创建每账户 Delivery；
4. 必要时写兼容记录；
5. 成功建立快照后再把基线更新为 ready。

任何一步失败都回滚。所有 JSON 序列化、SQL 执行和状态更新错误必须向上返回。

### P0-6：改为确定性的事件幂等键

当前 `dedupe_key` 包含 `OccurredAt.Unix()`。相同的状态变化只要在不同秒重试，就会得到不同 key，无法承担幂等职责。

推荐：

```text
site_id
+ definition_version
+ item_key
+ event_type
+ condition_id
+ before_fingerprint
+ after_fingerprint
```

对相同前后状态，任何重试都得到同一个 key；商品之后上涨再重新降到相同价格时，因为 before fingerprint 不同，仍然可以产生新事件。

数据库唯一约束作为最终保证，不要先 Count 再 Create；直接 Create/OnConflict，避免并发窗口。

### P0-7：只保留一条通知发送链路

当前 `SaveEvents` 会创建 pending Delivery，但定时检查随后又调用 `SendEventNotification` 立即发送。与此同时 `StartDeliveryWorker` 没有在 `main.go` 启动。

现在的实际效果是：

- 通知可能已经发送；
- Delivery 永远 pending；
- MonitorEvent 永远显示“待推送”；
- 兼容 UpdateRecord 也永远是未推送；
- 如果以后只把 worker 启动起来，又会把已经即时发送的事件重复推送。

修订方式：

1. 删除调度路径中的 `SendEventNotification`；
2. 检查流程只创建 Event 和 Delivery；
3. 在 main 生命周期中启动唯一的 Delivery worker；
4. 停机时等待 worker 安全退出；
5. worker 发送前检查全局通知开关；
6. 通过原子 claim 把任务从 pending/failed 改为 sending，避免多个 worker 重复领取；
7. 每次失败递增 Attempts，使用指数退避并设置最大次数；
8. 所有目标账户 sent/skipped 后才把事件聚合标记为 notified；
9. 一个账户成功不能提前把整个事件标为已通知；
10. 关键词不匹配使用 `skipped`，不要伪装成 `sent`。

---

## 3. 紧随其后的 P1 问题

### P1-1：保证编辑配置完整往返

编辑页面当前只回填旧字段，没有回填：

- `strategy_type`；
- `strategy_config.identity`；
- 价格字段；
- threshold；
- `field_data_types`。

打开价格监控后保存，可能把它改回 `presence`。

修订要求：

- 建立一个 `siteConfigToForm` 和 `formToSiteConfig` 映射层；
- 创建和编辑共用同一套策略表单；
- 增加前端单元测试或最少的配置往返测试：加载价格配置后不修改直接保存，请求体必须语义等价。

### P1-2：配置变化必须触发版本和基线策略

更新监控时目前没有递增 `ConfigVersion`，也不会在数据源、选择器、身份字段或价格字段变化后重建基线。

建议比较旧配置和新配置的“检测语义指纹”：

```text
URL + container + item + fields + field types + identity + conditions
```

指纹变化时：

- `config_version += 1`；
- `baseline_status = needs_baseline`；
- 删除或归档旧版本快照；
- 历史事件保留，并带 definition version；
- 仅修改名称、检查间隔、通知账户和通知过滤时不重建基线。

### P1-3：统一基线状态的数据库和内存视图

当前基线状态只更新数据库，运行中 Monitor 的 `site` 和 `MonitorStatus` 仍保留旧值，因此前端重新加载详情也可能继续显示 pending。

建立统一方法：

```go
func (m *Monitor) SetBaselineStatus(status string) error
```

它应在数据库提交成功后同步更新 `m.site.BaselineStatus` 和内存状态。重置基线需要先获取 monitor 检查锁，防止与正在执行的检查并发删除/写入快照。

### P1-4：调度检查和手动检查共用互斥及 context

当前手动 `/check` 会直接创建新 Engine 并修改状态，可以与 ticker 同时执行。`CheckOnce(ctx)` 虽然接收 context，但抓取仍调用不支持 context 的 `Fetch`。

修订方式：

- 每个 Monitor 持有 `checkMu`；
- 定时检查、手动检查和基线操作都从 Monitor 实例进入；
- `Fetcher.FetchContext(ctx, url)` 真正使用传入 context；
- stop 时 cancel 当前检查；
- `/check` 明确区分 `dry_run=true` 和提交检查，预览默认不修改 Snapshot。

### P1-5：实现真正的配置校验

当前 `/validate` 只做 JSON bind，然后始终返回 valid。`Detector.Validate` 也都是空实现，`NewEngine` 忽略 ParseDetectionRule 错误，并可能在非法 JSON 下发生 nil 解引用。

修订要求：

- `NewEngine` 返回 `(*Engine, error)`；
- 未知 StrategyType 返回错误，不要回退到 presence；
- 校验 container/item/fields；
- 校验 identity 引用存在且为稳定字段；
- 校验 condition field 存在、类型受支持；
- 校验 threshold 格式和范围；
- 校验 price 字段可以成功解析至少一个样本；
- `/validate` 返回 warnings 和 sample observations，而不是固定字符串。

### P1-6：统一 presence 与 field_transition 的执行管线

当前只有 `field_transition` 进入新引擎，`presence` 继续使用旧的 `UpdateRecord title+url` 路径，因此新增的 `PresenceDetector` 在生产调度中实际未使用。

这会导致两种监控拥有不同的状态、事件、通知和统计语义。应在 P0 稳定后让两种策略都经过：

```text
Extract → Normalize → Evaluate → PersistEvaluation → Delivery
```

旧 `/updates` 和统计接口通过兼容查询读取新事件，不再维护两套主流程。

### P1-7：补齐删除和数据生命周期

`DeleteSiteCascade` 目前只删除 SiteField 和 UpdateRecord，没有删除新增的 Snapshot、Event 和 Delivery。

应在同一事务中按顺序清理：

1. notification_deliveries；
2. monitor_events；
3. monitor_snapshots；
4. update_records；
5. site_fields；
6. sites。

账户删除还应明确处理历史 Delivery：保留投递审计时使用 nullable account 或账户快照；否则先清理不可重试的 pending 任务。

### P1-8：完善价格规范化

当前金额解析使用 `ParseFloat * 100`，并对所有货币都假设两位小数。JPY 等零小数货币会出现存储和格式化不一致；欧洲小数逗号也可能被误读。

第一阶段至少需要：

- currency exponent 表：JPY/KRW 为 0，常见币种为 2；
- 使用十进制字符串算法或 decimal/fixed-point，不经过 float64；
- 支持显式配置默认币种，不能所有无符号价格都默认为 CNY；
- 区分 `1,299.00` 和 `1.299,00`；
- 多个价格文本时必须通过选择器/转换得到唯一数值，否则报错；
- 币种变化默认不比较，并给出诊断。

---

## 4. 推荐修订顺序

### Step 1：建立失败测试

在修改实现前先添加以下回归用例：

1. 自定义价格字段 `sale_price` 能产生事件；
2. amount/percent 阈值生效；
3. `identity.source=source_url` 使用 Site.URL；
4. 身份缺失或重复时检查失败；
5. `100 → invalid → 90` 仍然降价；
6. 快照保存成功、事件保存失败时整笔事务回滚；
7. 相同 evaluation 重试只产生一个事件；
8. 两个通知账户中一个失败时事件不提前 notified；
9. 编辑价格监控不改变其策略配置；
10. Event API 输出 snake_case；
11. 删除监控后新表无孤儿数据；
12. `npm run build` 成功。

### Step 2：修复配置执行链

- 修复 DTO 和前端模板；
- 解析 FieldDataTypes；
- 修复 source_url；
- Detector 持有并执行规则；
- 实现阈值；
- 拒绝非法身份 fallback；
- 让 validate 真实工作。

### Step 3：修复状态一致性

- 无效价格保留旧状态；
- 增加健康门槛；
- 建立确定性 fingerprint 和 dedupe key；
- 用单事务提交 Snapshot/Event/Delivery/Baseline；
- 所有仓储错误向上传递。

### Step 4：修复投递闭环

- 移除同步直发；
- 启动带生命周期的 worker；
- claim、attempt、backoff、max attempts；
- 全局开关和关键词 skipped；
- 多账户聚合通知状态；
- 兼容 UpdateRecord/统计从 Delivery 聚合状态。

### Step 5：统一引擎并完成迁移

- presence 迁移到新引擎；
- 配置版本和基线重建；
- 手动与定时检查加锁；
- 删除级联；
- 最终清理旧比较和直发代码。

---

## 5. 完成标准

修订后应同时满足：

- `go test ./...`、`go vet ./...`、`npm run build` 全部成功；
- 所有策略配置都经过真实 Validate，并在执行时被使用；
- 状态推进、事件创建和投递任务创建具备原子性；
- 推送只能通过 Delivery worker 发生；
- 任何失败不会把有效价格覆盖为无效值；
- 相同检查重试不重复事件，不同时间的真实再次降价可以产生新事件；
- 编辑、重启、重置基线和配置迁移不会改变未授权的监控语义；
- 公告新增和价格下降共享同一引擎主流程；
- 事件和投递状态在 API、详情页、统计页之间一致。

在完成 P0 前，不建议用真实电商页面和正式推送账户运行；否则可能出现漏报、重复推送和界面永久显示“待推送”。
