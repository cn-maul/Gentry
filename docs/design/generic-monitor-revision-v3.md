# 通用监控引擎第三轮复验与收口方案

## 1. 结论

第三轮确认此前的 P0 主路径已经基本闭环：价格规则可以创建事件，Delivery worker 会启动并实际消费任务，币种变化不会误触发，前后端构建和已有测试均通过。

当前版本可以用于静态 HTML、单币种商品页的受控测试，但仍有若干高优先级 P1。它们不会让所有价格通知完全失效，却会导致编辑后无故丢失基线、通知状态显示不准、配置异常无法提前发现，或在并发操作下推进错误状态。

---

## 2. 高优先级 P1

### P1-1：配置指纹使用的“旧配置”实际上已被覆盖

`updateMonitor` 先把 req 的 URL、container、item、strategy 等写入 `site`，之后才计算 oldFingerprint。因此 oldFingerprint 使用的是新配置，而不是数据库中的旧配置。

同时查询 Site 时没有 Preload Fields，`site.Fields` 通常为空；newFields 又包含表单字段。因此绝大多数编辑都会被判断为检测语义变化，导致：

- ConfigVersion 无故递增；
- Snapshot 被删除；
- BaselineStatus 变为 needs_baseline；
- 仅修改名称、间隔或通知设置也可能重建基线。

修复方式：

1. 使用 `Preload("Fields")` 加载旧 Site；
2. 在修改 site 之前计算 oldFingerprint；
3. 使用独立 newSite/newConfig 计算 newFingerprint；
4. fingerprint 包含 Name 以外的检测语义字段，但字段定义必须包含 Name、Selector、Type、Attr、Transform；
5. StrategyConfig 先解析为结构体并规范化 JSON，再计算哈希，避免 JSON key 顺序造成误变化。

### P1-2：删除旧快照没有纳入更新事务

检测配置变化时，当前先直接删除 Snapshot，再调用 UpdateSiteWithFields。如果后续保存 Site/Fields 失败，旧快照已经不可恢复。

应新增事务型更新方法：

```go
func UpdateMonitorDefinition(site *Site, fields []SiteField, resetBaseline bool) error
```

同一事务完成：

- 更新 Site；
- 替换 Fields；
- 必要时删除/归档 Snapshot；
- 更新 ConfigVersion；
- 更新 BaselineStatus。

任何步骤失败必须保留旧配置和旧快照。

### P1-3：Delivery dead 后没有触发事件状态聚合

`failDelivery` 把达到最大次数的任务改为 dead 后直接返回，没有调用 aggregateEventStatus。若最后一个非终态任务变成 dead，事件可能永久保持旧状态。

修复：所有状态进入 sent、skipped、dead 后都必须统一调用一次聚合。不要让 processDelivery 各分支自行决定是否聚合，建议封装：

```go
func transitionDelivery(id uint, status string, fields map[string]interface{}) error
```

事务更新 Delivery 后重新计算 EventDeliveryStatus。

### P1-4：MonitorEvent.Notified 无法表达 skipped/partial/dead

当前只有 bool：

- 全部 skipped 时 Notified=false，前端显示“待推送”；
- 一个 sent、一个 dead 时 Notified=true，前端显示“已推送”，掩盖部分失败；
- 全部 dead 时也无法区分“仍在等待”和“永久失败”。

建议增加 EventDeliveryStatus：

```text
pending / delivered / skipped / partial / failed
```

Notified 仅作为兼容字段。事件 API 和详情页展示新的聚合状态。

### P1-5：兼容 UpdateRecord 永远无法同步真实投递状态

价格事件仍写入 UpdateRecord，但 worker 成功后只更新 MonitorEvent，不更新对应 UpdateRecord。因此旧更新历史和统计面板中的 unnotified_updates 会持续增长。

推荐停止为 field_transition 写兼容 UpdateRecord，并把价格监控详情、总事件数、待投递数和今日推送数改读 MonitorEvent/Delivery。

如果短期必须双写，则给 UpdateRecord 增加 EventID，并在聚合后同步状态。

### P1-6：无效价格没有真正保留完整旧快照

现有代码只恢复 PriceValid、PriceMinor 和 Currency，没有恢复 Payload/Fingerprint。新增测试只断言 title，而新旧 title 相同，所以无法发现该问题。

测试应至少断言：

```text
snapshot.Payload[price] 仍为旧有效价格
snapshot.Fingerprint 与旧快照相同
```

实现上，无效价格时完整复用 existing Payload/Fingerprint；如果希望更新 title 等字段，应执行字段级合并，但价格字段必须保留旧值。

### P1-7：基线状态的数据库和内存状态仍不同步

Engine 在事务中把 Site.BaselineStatus 更新为 ready，但运行中的 Monitor.site 和 MonitorStatus 仍保留 pending/needs_baseline。详情 API 读取的是内存状态，因此页面可能一直显示“待建立”。

基线提交成功后应通知 Monitor 实例同步：

```go
m.SetBaselineStatus("ready")
```

reset baseline 也必须同时更新数据库、内存状态，并与检查锁互斥。

### P1-8：编辑页面虽回填配置，但没有对应编辑控件

编辑模板没有显示监控类型、价格字段、身份和阈值控件。数据在脚本中被隐藏回填并再次提交，但用户无法查看或修改价格规则。

创建和编辑应复用同一个 StrategyEditor 组件，避免两套模板继续漂移。

---

## 3. 可靠性 P1/P2

### 检查并发和 context

- ticker 检查与 `/check` 仍可并发；
- reset baseline 可与检查并发删除 Snapshot；
- CheckOnce 的 ctx 没有传到 HTTP Fetch；
- StopMonitor 不能取消正在进行的请求。

仍建议增加 monitor 级 checkMu、FetchContext 和可取消检查。

### 配置校验仍为空实现

`/validate` 仍然固定返回 valid，Detector.Validate 仍未实现。至少应验证：

- StrategyType 与 StrategyConfig.type 一致；
- identity 字段存在；
- price field 存在且 data type 为 money；
- operator 受支持；
- amount/percent 可解析且非负；
- 单商品预览能提取唯一身份和有效价格。

### 金额精度和币种最小单位

Amount threshold 仍经 float64 再乘 100，`10.10` 等值可能受浮点截断影响；JPY/KRW 等零小数币种的 Normalize/Format 也没有统一 exponent。

生产前应改为十进制定点解析，并建立 currency exponent 表。

### source_url 事件链接

Detector 的事件 URL 只从 payload.url 获取。单商品页面使用 source_url 身份但没有提取 url 字段时，通知链接为空。Engine 应在事件 URL 为空时回退到 Site.URL。

### DeliveryService 停机

main 使用 context.Background 调用 Stop，并忽略错误。如果 worker 卡在外部通知请求，退出可能无限等待。应使用 `context.WithTimeout` 并记录超时错误。

---

## 4. 测试补强

下一轮测试应从 Detector 单元测试扩展到真实数据库和 worker：

1. 仅修改检查间隔不会增加 ConfigVersion 或删除 Snapshot；
2. 修改 price selector 会增加版本并事务性重建基线；
3. 更新配置失败时旧 Snapshot 保留；
4. invalid 观测保留旧 price payload 和 fingerprint；
5. delivery 最后一次失败进入 dead 后事件变为 failed/partial；
6. 全 skipped 事件显示 skipped，不显示 pending；
7. UpdateRecord/统计与 EventDeliveryStatus 一致；
8. baseline ready 同时更新数据库和 MonitorStatus；
9. 手动检查与 ticker 不会并发提交；
10. source_url 监控的通知链接回退到 Site.URL。

---

## 5. 推荐收口顺序

1. 修复 updateMonitor 旧指纹和事务性基线重建。
2. 修复 Delivery dead 聚合，并引入 EventDeliveryStatus。
3. 统一或移除价格事件的 UpdateRecord 兼容写入。
4. 同步内存基线状态，补检查锁和 context。
5. 实现真实 Validate 和编辑 StrategyEditor。
6. 最后处理定点金额、币种 exponent 和 presence 统一引擎。

前两项完成后，系统才适合长时间运行并接受频繁配置编辑；当前版本更适合先用测试推送账户和少量监控进行验证。
