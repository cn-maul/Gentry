# 通用监控引擎最终收口方案（第四轮差量）

## 1. 复核结论

第三轮修复已经消除了已知的主链路 P0：价格下降能够生成事件，事件能够进入 Delivery 队列，worker 能启动并处理任务，首次基线静默、币种不一致保护、无效价格保留和配置更新事务也已经落地。

当前版本可以进入少量、单实例、低并发的受控试运行，但还不建议直接作为长期无人值守的生产版本。剩余问题主要不是编译问题，而是配置切换、手动操作与定时检查并发时的状态一致性，以及事件投递状态没有真正端到端闭环。

本文件只列第三轮后仍未解决的差量项，不重复已经修复的内容。

---

## 2. 发布前建议完成的 P1

### P1-1：为检查、重置基线和配置切换建立同一个 monitor 级串行边界

当前定时检查由运行中的 `Monitor` 执行，但手动 `/check` 会另建一个 `Engine`；重置基线直接删数据库快照；配置更新时 `Stop()` 只关闭通道，不等待正在执行的检查结束。

这会产生真实的时序错误：

- 定时检查和手动检查可能同时读取同一旧快照，再以不同抓取结果覆盖快照；
- reset 删除快照后，一个已经开始的旧检查可能再次写回旧基线并把状态改成 ready；
- 检测配置更新已经事务性删除旧快照后，旧 Monitor 的在途检查仍可能把旧定义的快照写回；
- 新 Monitor 随后会加载旧版本快照，用新选择器/新字段与旧数据比较，造成误报或漏报。

建议改造：

1. `Monitor` 增加 `checkMu`、当前检查的 cancel function 和 `doneCh`；
2. 定时检查与手动检查都必须调用同一个 `Monitor.CheckNow(ctx)`，禁止 handler 直接创建 Engine；
3. reset baseline 在持有同一把锁时，用事务完成 Snapshot 删除和 BaselineStatus 更新，并同步内存状态；
4. 配置替换先取消旧检查并等待退出，再提交/启用新定义；至少必须保证旧实例在新快照删除后不能再写库；
5. 快照读取应按 `site_id + definition_version` 隔离，作为并发防线，而不是只依赖删除旧数据。

验收测试：

- ticker 与手动检查同时触发时，fetch/evaluate/persist 全链路最多只有一个执行；
- 检查执行到 persist 前触发 reset，最终数据库中不存在旧基线；
- 检查执行中修改 price selector，旧版本检查不能在新配置提交后写入 Snapshot/Event；
- 手动建立首个基线后，数据库和 `MonitorStatus.BaselineStatus` 同时为 ready。

### P1-2：补全并规范化配置语义指纹

`computeDetectionFingerprint` 当前字段部分只包含 `Name/Selector/Type`，遗漏 `Attr/Transform`。修改属性提取方式或转换规则会改变实际观测值，却不会递增 ConfigVersion 或重建基线。

此外，`StrategyConfig` 仍按原始 JSON 字符串参与指纹；语义相同但 key 顺序、空字段表达不同的 JSON 可能误触发基线重建。

建议：

1. 定义稳定的 `DetectionDefinition` 结构体，包含 URL、Container、Item、字段的 Name/Selector/Type/Attr/Transform、StrategyType、解析后的 DetectionRule、FieldDataTypes；
2. 字段按稳定规则排序；JSON 配置先解析、补默认值、再 canonical marshal；
3. 使用 SHA-256 等固定长度摘要，不直接拼接字符串；
4. 增加“仅字段顺序变化不重建”“Attr/Transform 变化必须重建”“JSON key 顺序变化不重建”测试。

### P1-3：事件幂等键必须包含 DefinitionVersion，并改成固定长度哈希

当前 dedupe key 只包含 `siteID + eventType + itemKey + beforeFP + afterFP`。配置重建后，如果同一商品再次出现相同的价格前后状态，数据库会把它误判成旧配置版本的重复事件，从而不再创建 Delivery，形成永久漏报。

建议键语义为：

```text
SHA-256(site_id, definition_version, event_type, item_key, before_fingerprint, after_fingerprint)
```

同时让数据库唯一约束直接落在完整哈希上，避免长 URL/item key 拼接超过字段长度或不同数据库发生截断。

验收测试：同一版本重试只生成一个事件；版本递增后相同 before/after 可以生成新事件。

### P1-4：把 DeliveryStatus 真正接到 API、前端和无收件人场景

后端虽然新增了 `pending/delivered/skipped/partial/failed`，但详情页仍只读取 `notified`，所以：

- skipped、failed 都显示为“待推送”；
- partial 显示为“已推送”，看不到部分失败；
- 没有配置通知账户时不会创建 Delivery，也不会触发聚合，事件会永久停留在 pending。

建议：

1. 事件创建后，即使 `accountIDs` 为空也执行一次状态初始化；建议明确增加 `no_targets`，或统一映射为 skipped 并记录原因；
2. 详情页直接按 `delivery_status` 显示中文状态与颜色，`notified` 只保留兼容用途；
3. 明确混合终态规则，包括 sent+skipped、skipped+dead，不要让 `sentCount > 0` 自动等同于 delivered；
4. `transitionDelivery` 和聚合函数返回 error，更新或 Count 失败必须记录并可重试，不能静默忽略。

验收测试覆盖：无账户、全 sent、全 skipped、全 dead、sent+dead、sent+skipped、skipped+dead。

### P1-5：停止价格事件与旧 UpdateRecord 的不完整双写

价格事件会同时写 `MonitorEvent` 和兼容 `UpdateRecord`，但 Delivery 完成后只更新前者。结果是详情页“更新历史”、首页 `unnotified_updates` 和“今日已推送”等旧统计会持续失真；价格详情页还会同时出现旧记录与价格事件两套历史。

推荐方案：

- `field_transition` 只写 MonitorEvent/Delivery；
- 价格监控的历史、未投递数和推送统计全部从新事件模型读取；
- presence 在迁移到新引擎前继续使用 UpdateRecord。

如果必须短期双写，则给 UpdateRecord 增加 EventID，并在事件聚合事务中同步 Notified/NotifiedAt，但这只应是迁移方案。

### P1-6：实现真实配置校验，拒绝“策略类型与实际 detector 不一致”

`/validate` 和两个 Detector 的 `Validate` 当前仍是空实现。特别是 `performCheck` 按 `Site.StrategyType` 进入新引擎，而 `NewEngine` 实际从 `StrategyConfig.type` 选择 detector；API 客户端若提交 `field_transition + 空配置`，就可能进入新引擎却使用 PresenceDetector。

最低校验集：

- `Site.StrategyType == DetectionRule.Type`；
- identity 恰好配置一种来源，引用字段存在且样本中非空、唯一；
- field_transition 至少有一个受支持条件，price field 存在且为 money；
- operator 必须为当前实现支持的 decreased；
- amount/percent 可精确解析、非负且不能静默降级为 0；
- 预览抓取能提取至少一个有效价格；
- 多商品页面使用 source_url 作为 identity 时给出明确错误。

建议创建和更新接口也复用同一个服务端 validator，不能只依赖前端调用 `/validate`。

### P1-7：修正多币种金额模型

金额解析目前统一乘以 100，格式化时 JPY 又直接输出 minor，导致 `JPY 1000` 被保存/显示成 `100000`；KRW 等零小数币种也存在同类问题。欧洲格式 `1.299,99` 也会被错误解析。

建议：

1. 建立 currency exponent 表，不再假设所有币种都有两位小数；
2. 金额和阈值使用十进制定点解析，禁止经 float64 乘 100；
3. 明确 locale/decimal separator，无法确定时返回 invalid，不要猜出错误价格；
4. `ChangeAmount`、formatPrice 和 amount threshold 共用同一 Money 类型。

测试至少覆盖 CNY/USD、JPY/KRW、`1,299.99`、`1.299,99`、`10.10` 阈值和非法阈值。

---

## 3. 可随后处理的 P2

### P2-1：URL 回退必须使用 Site.URL，不能使用任意 item key

Detector 在 payload 没有 url 时把 `itemKey` 当成 URL。identity 为 SKU/商品编号时，通知链接会变成不可点击的 SKU；PersistEvaluation 也会保留这个非空错误值，使 worker 无法回退到 Site.URL。

应让 Detector 只表达“URL 未知”，由 Engine 在具备 Site 上下文的位置统一回退到 Site.URL。

### P2-2：贯通 context 与有界停机

`CheckOnce(ctx)` 没有把 ctx 传给 Fetcher；手动检查使用 `context.Background()`；主程序用无截止时间的 Background 停止 DeliveryService。

建议增加 `Fetcher.FetchContext(ctx, url)`，handler 使用请求 context，Monitor.Stop 取消正在进行的抓取；停机使用 `context.WithTimeout` 并记录超时错误。

### P2-3：补齐快照版本和 Delivery 细节

- Snapshot upsert 冲突更新时同步 `definition_version`，并在加载时过滤当前版本；
- stale sending 回收时清理 lease，并记录回收原因；
- 首次失败退避当前实际为 10 分钟，若设计要求 5 分钟，应使用 `5m * 2^(attempts-1)`；
- transition/aggregate 的数据库错误不得丢弃；
- 唯一冲突应使用数据库错误类型或 `OnConflict DoNothing`，不要匹配错误字符串中的 `UNIQUE`。

### P2-4：逐步把 presence 迁入统一引擎

当前 presence 仍走旧的比较、UpdateRecord 和同步通知路径，而 field_transition 走 Snapshot/Event/Delivery。短期兼容没有问题，但双主链会让状态、统计、重试和后续新增规则继续分叉。

建议在上述 P1 稳定后：

1. 让 PresenceDetector 也通过 Engine/PersistEvaluation；
2. 将旧 UpdateRecord 变成只读迁移数据或兼容视图；
3. 所有策略共用检查锁、事件模型、Delivery worker 和状态统计；
4. 最终删除 `performCheck` 中的策略分叉。

---

## 4. 推荐实施顺序

1. P1-1 检查生命周期和版本隔离；
2. P1-2/P1-3 配置指纹与版本化幂等；
3. P1-4/P1-5 投递状态与历史统计闭环；
4. P1-6 服务端校验；
5. P1-7 金额模型；
6. P2 context、URL、worker 可靠性；
7. presence 迁移到统一引擎。

完成前三步后，价格监控才适合长期运行并频繁编辑配置；完成 P1-6/P1-7 后，才能较可靠地对外宣称支持通用电商页面和多币种。

---

## 5. 本轮验证结果

- `npm.cmd run build`：通过；
- `go test ./...`：通过；
- `go vet ./...`：通过。

现有测试主要覆盖金额解析和 Detector 单元行为，尚未覆盖配置更新、reset/manual/ticker 的并发时序，也没有覆盖 Delivery 状态从数据库到前端的端到端一致性。下一轮应优先增加上述验收测试，而不是继续只补 Detector 单元测试。
