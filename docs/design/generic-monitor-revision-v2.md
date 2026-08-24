# 通用监控引擎第二轮复验与增量修订方案

## 1. 本轮复验结果

本轮确认已经修复：

- `MonitorDetail.vue` 可以通过生产构建；
- MonitorEvent 等模型已经提供前端需要的 snake_case JSON 字段；
- 后端 Detector 能读取自定义价格字段；
- 百分比阈值的后端判定已经存在；
- `source_url` 的后端生成函数可以使用 Site.URL；
- 价格无效后保留了旧的 PriceMinor/Currency/PriceValid；
- Snapshot、Event 和 Delivery 的核心写入已经进入同一数据库事务；
- 去重键不再依赖发生时间；
- 同步直发路径已经删除；
- 删除监控器时已清理新表数据；
- `go test ./...`、`go vet ./...`、`npm run build` 均通过。

但当前版本仍不能直接用于正式价格推送，因为还有 3 个 P0 和若干关键 P1 没有闭环。

---

## 2. 剩余 P0

### P0-1：Delivery worker 没有启动

`StartDeliveryWorker` 已经实现，但 `main.go` 没有调用它。当前价格检查只创建 pending Delivery，不再同步发送，因此实际运行时不会产生任何价格通知。

修复方式：

1. 在数据库和通知设置初始化完成后创建 worker stop channel；
2. 启动 `go monitor.StartDeliveryWorker(interval, stopCh)`；
3. 启动时先立即执行一次 DeliveryWorker，而不是等第一个 ticker；
4. 收到退出信号后关闭 worker stop channel；
5. 等待 worker 退出后再关闭程序；
6. 增加集成测试：创建 pending delivery 后 worker 能发送并转为 sent。

建议不要让 main 自己拼 stop channel，最好封装：

```go
type DeliveryService struct {
    stopCh chan struct{}
    doneCh chan struct{}
}

func (s *DeliveryService) Start()
func (s *DeliveryService) Stop(ctx context.Context) error
```

### P0-2：前端仍没有提交正确的身份和阈值配置

“页面 URL”仍被序列化为：

```json
{"identity":{"field":"source_url"}}
```

正确格式应为：

```json
{"identity":{"source":"source_url"}}
```

此外，页面上的最低降价金额和最低降价百分比仍没有写入 condition.threshold，导致用户输入没有实际效果。

前端构造方式应明确分支：

```js
const identity = form.identity_field === 'source_url'
  ? { source: 'source_url' }
  : { field: form.identity_field }

const threshold = {}
if (form.min_drop_amount > 0) threshold.amount = String(form.min_drop_amount)
if (form.min_drop_percent > 0) threshold.percent = form.min_drop_percent
```

后端金额阈值目前直接把 Amount 当 int64 最小单位读取。如果 UI 输入 `10` 表示 10 元，后端必须使用与 price 相同的 money normalizer 转成 1000 分，不能把它理解成 10 分。

需要增加一个 API 级测试，直接断言 Vue 提交的真实 JSON 可以：

- 使用 Site.URL 生成身份；
- 对 `100 → 95` 应用 10 元阈值时不通知；
- 对 `100 → 80` 应用 10 元阈值时通知；
- 对百分比阈值同样生效。

### P0-3：不同币种会产生错误降价事件

当前只检查旧、新价格是否有效，没有检查 `existing.Currency == new.Currency`。例如旧值 `CNY 100`、新值 `USD 90` 会被判断为降价。

第一阶段正确行为：

- 币种相同才允许比较；
- 币种变化时不产生价格事件；
- 保留旧有效比较基线，或把监控标记为需要人工确认；
- 记录明确诊断，不进行隐式汇率换算。

增加 `CNY 100 → USD 90` 不产生降价事件的测试。

---

## 3. 仍需完成的关键 P1

### P1-1：编辑价格监控仍会丢配置

编辑页面仍未回填 strategy_type、identity、price_field、amount/percent threshold 和 field_data_types。加载价格监控后直接保存，仍可能把它改回 presence 或清空规则。

修复：建立成对的配置映射函数，并做 round-trip 测试：

```text
API config → form → submit payload
```

在用户未修改的情况下，前后 DetectionRule 必须语义等价。

### P1-2：配置版本和重新基线仍未执行

更新监控时没有递增 ConfigVersion，也没有判断 URL、selector、identity、price field、currency 或 condition 的变化。已有 Snapshot 会直接被新配置继续使用。

修复：

- 对影响检测语义的字段计算 config fingerprint；
- fingerprint 改变时 ConfigVersion+1；
- 清理/归档旧 Snapshot；
- BaselineStatus 设置为 needs_baseline；
- 去重键加入 DefinitionVersion；
- Snapshot upsert 时更新 definition_version。

仅名称、检查间隔和通知账户变化不应重建基线。

### P1-3：Delivery 的终态聚合不正确

当前事件只有在所有 Delivery 都是 sent 时才标记 notified，但关键词过滤和全局通知关闭会把 Delivery 标记为 skipped。`status != sent` 会把 skipped 永久计入 pendingCount，使事件一直显示待推送。

修复：

```text
非终态：pending / sending / failed-retryable
成功终态：sent
跳过终态：skipped
失败终态：dead
```

事件聚合状态建议单独定义：

- delivered：至少一个 sent，且没有非终态任务；
- skipped：全部 skipped；
- partial：部分 sent，部分 dead；
- failed：全部 dead；
- pending：仍有非终态任务。

不要只用一个 bool 表达所有情况。若暂时保留 bool，至少把 sent 和 skipped 都视为终态，并确保 UI 不显示为“待推送”。

### P1-4：sending 任务在进程崩溃后永久卡住

worker claim 后直接把状态改为 sending，但没有 lease 时间。进程在发送前崩溃时，该任务不会再次被 PendingDeliveries 查询到。

增加：

- claimed_at 或 lease_until；
- 查询时回收超过 lease 的 sending；
- claim 同时更新 attempts/worker id/lease；
- 对发送接口使用合理 timeout。

### P1-5：兼容 UpdateRecord 状态仍与 Event/Delivery 不一致

兼容记录写入失败仍只记日志并继续提交，且 Delivery 成功后不会把对应 UpdateRecord 标记为 notified。旧更新列表和统计面板会持续增加“待推送”数量。

两种可选方案：

1. 给 UpdateRecord 增加 EventID，Delivery 聚合成功后同步更新；
2. 价格监控的历史和统计完全改读 MonitorEvent/Delivery，不再写兼容记录。

推荐第二种，避免继续维护两份事实来源。过渡期间兼容写入失败必须决定是事务失败还是明确降级，不能声称“任何一步失败都会回滚”但实际只打印日志。

### P1-6：无效价格只保留数值，没有保留完整旧价格快照

当前无效价格会保留 PriceMinor/Currency/PriceValid，但 Payload 和 Fingerprint 已被替换成无效观测。随后 `100 → invalid → 90` 的事件 OldValue 正确，但 BeforeJSON 和 dedupe before fingerprint 表示的是 invalid，而不是最后有效的 100。

修复：

- 无效价格时完整保留旧价格字段；
- 最简单的第一版可以完整保留 existing.Payload 和 existing.Fingerprint；
- 如果还要更新 title 等非价格字段，应只合并有效字段，价格字段保留旧值；
- 单独记录 last_observation_error，不混入可比较 Snapshot。

### P1-7：检查并发、context 和校验 API 仍未完成

目前：

- 定时检查和手动检查没有共享 monitor 级互斥锁；
- CheckOnce 接受 ctx，但 Fetch 使用 context.Background；
- reset baseline 可以与正在执行的检查并发；
- `/validate` 仍然固定返回 valid；
- Detector.Validate 仍为空实现。

需要按上一版方案完成 checkMu、FetchContext、dry-run check、事务性 baseline reset 和真实 Validate。

### P1-8：presence 仍走旧引擎

field_transition 使用 Event/Delivery，新公告监控仍使用旧 UpdateRecord 和同步通知。两种策略的重试、统计和历史语义不同。

完成上述 P0/P1 后，把 presence 切换到同一 Engine/PersistEvaluation/Delivery 流程，再移除旧的 compareResults 和 sendCombinedNotification 主路径。

---

## 4. 补充测试门槛

现有新增测试主要是 Detector 纯函数测试，尚未覆盖事务和 worker。下一轮至少增加：

1. worker 实际启动并消费 pending；
2. 发送失败后 attempts 和 next_attempt_at 正确；
3. stale sending 可以重新领取；
4. 两账户一成功一失败时事件聚合正确；
5. skipped 不会永久显示 pending；
6. PersistEvaluation 任一步失败时 Snapshot/Event/Delivery 全部回滚；
7. 重试相同 Evaluation 不增加事件和 Delivery 数量；
8. 不同币种不比较；
9. 金额阈值按货币单位解释；
10. 编辑配置 round-trip；
11. 配置语义变化触发新基线；
12. 手动检查与定时检查不能并发推进状态。

每轮最低验证：

```text
go test ./...
go vet ./...
npm run build
```

---

## 5. 推荐实施顺序

1. 启动 Delivery worker，补消费集成测试。
2. 修复前端 identity/threshold 请求体和后端金额阈值单位。
3. 增加币种一致性保护。
4. 修复编辑配置 round-trip。
5. 修复 Delivery 终态、lease 和事件聚合。
6. 完成 ConfigVersion、基线、并发和真实 Validate。
7. 统一 presence 到新引擎。

前三项完成前，价格事件虽然能入库，但无法保证能正确推送或按用户配置判断，因此仍不建议用于正式监控。
