# Gentry 规则中心化架构重构设计（v2 草案）

> 状态：P1 + P2 已实施（接口骨架 + 统一捕获管线 + /capture 与 /test-draft 端点）；
> P3（血缘与可观测深化）、P4（物理拆包/启发式提案器）待排期。

## 1. 背景与目标

Gentry 当前是一个"纯规则驱动"的网页新增监控系统：内容识别完全依赖已保存的扫描规则，
规则命中 URL 后按选择器提取条目，监控定时比对产生通知。

本次重构围绕三条核心链路展开：

1. **规则命中获取内容**：URL → 命中规则 → 提取条目，这条运行时链路要更清晰、可测试、可复用；
2. **URL + 关键字捕获新规则**：给定一个页面和"正确条目应包含的关键词"，系统能产出一份
   *候选规则草稿*（容器/条目/字段选择器），经人工确认后进入规则库——把现在只有 LLM 一条路
   的捕获能力升级为统一管线；
3. **LLM 接入辅助创建规则**：保留并强化现有 AI 提取，作为捕获管线中的提案器（Proposer），
   同时补齐健壮性、可观测性和多 Provider 扩展能力。

### 非目标

- 不改变"系统不自动创建监控/不自动修改已有监控"的产品原则；
- 不引入浏览器渲染抓取（登录态/JS 重站仍超范围）；
- 不更换存储引擎，不破坏现有 SQLite 数据与 API 兼容性。

## 2. 现状诊断

### 2.1 规则扫描与提取核心链路（已核实）

> 体量勘误:scanner.go 实为 226 行("约 1300 行"来自过时的 code-analysis-report.md)。
> 启发式引擎(~800 行)已删除,现有体量:scanner 226 / scanner_rules 241 / source 389 /
> extractor 288 / monitor.go 550 / manager.go 249 / web/scanrules.go 678 / web/monitors.go 336。

**链路事实**：

- 预览:`previewScan → SmartScan` 先评估 api_json 模板规则,**命中即短路返回**(不再看 HTML 规则);
  否则抓 HTML → `matchingHTMLScanRules` 合并 DB 模板(优先级≥70)与外部文件规则(60),
  过滤后按优先级降序逐条试提取,容器无命中即跳过,样本截 10 条。
- scope 匹配(`ScanRuleMatchesURL`):global 恒真;exact/route/section 要求同主机 +
  规范化 URL(path Clean、query 编码排序);遗留空 ScopeType 退化为 URLContains 子串。
  优先级分层:DB 模板 ≥70 > 外部文件 60 > 快速保存 50。
- 运行时:`CheckNow` 经容量 1 的 checkGate 串行化 → 提取 → 与 `DISTINCT(title,url)`
  基线比对(键 title|url,含 json-hash 兜底)→ 首次成功静默建基线 → 差分写 UpdateRecord →
  同步通知(**全部账户成功才标 notified**,部分失败留重试)。
- 规则↔监控:**副本语义确认**,全库无 SourceRuleID/外键;quickCreate 固定 priority=50;
  候选结果携带来源规则名(Strategy 字段)但不含规则 ID。

**耦合与坏味道（S 系列）**：

| # | 问题（事实） | 对策落点 |
|---|---|---|
| S1 | `database.GetDB()` 在 monitor 包 10+ 处直呼;fetcher 函数体内 New(),不可注入,只能带库带网测试 | §8 P1 引入轻量依赖参数(不引入 DI 框架) |
| S2 | extractor 主路径夹带**旧配置修复启发式**(narrowLegacyBroadContainers/contentContainerScore 等 4 个函数,每次提取执行打分),文档定性为"兼容存量配置",属灰色地带 | §8 P1 隔离成独立 compat 层 + 特征化测试锁行为,移除决策后置 |
| S3 | 去重键两套口径:compareResults 用 extractKey(json-hash 兜底),saveResults 直拼 title\|url | §8 P1 统一为单一键函数 |
| S4 | 三份相似的"构配置→提取→组装候选"实现(testScanRuleAPI/HTML、applyHTMLScanRule、buildJSONScanCandidate);validateMonitorSourceURL 与 validateScanRuleSource 同构 | §5.2 CapturePipeline 收敛为唯一实现 |
| S5 | 字段五件套(FieldConfig/ScanFieldConfig/SiteField/ScanRuleField/fieldRequest)同构,~10 个互转函数散落两包 | P1 集中到单一转换文件(不改结构) |
| S6 | monitor.go 六职责一体;sendCombinedNotification 单函数 77 行 | 后置,仅在触碰时顺带拆分 |
| S7 | 包级可变全局(runtimeScanRules、monitors map) | 接受现状,接口化时不扩大 |

**扩展点确认（好消息）**：

- `htmlScanRule.matchesURL` 已是单方法函数类型——Matcher 接口化只需动
  scanner_rules.go 加两个直调点,改动面极小;
- `NewExtractor(SiteSelectors).Extract(html) → []ExtractResult` 签名已是纯函数,
  障碍仅 S2 的修复启发式与一个包级 dateLikePattern;
- 版本语义有现成范本(Site.ConfigVersion + computeDetectionFingerprint 的 canonical SHA-256),
  规则 revision 可照搬;
- 注意:quickCreate 与 addMonitor 是两个独立请求,后端不留关联——血缘需要
  preview 候选携带 rule_id 且前端在建监控时回传。

### 2.2 LLM 捕获链路（已核实）

**现有能力**（`backend/llm/llm.go`、`backend/monitor/ai_extract.go`、`backend/web/ai.go`）：

- `llm.Chat()` 调 OpenAI 兼容 `/chat/completions`,90s 超时,响应截 4MB;
  `temperature=0`、`stream=false` 硬编码；无重试、无 usage 统计、无 json mode。
- Prompt:系统提示词定义"只输出一个 JSON 对象"+目标形状
  `{"container","item","fields":[{name,selector,type,attr,transform}]}`;用户消息为
  关键词 + 清洗后 HTML(`prepareHTMLForAI`:去 script/style 等,**按字节截断 60KB**)。
- 验证回路:`for attempt < 2`,最多 2 次 LLM 调用、1 次反馈重试;
  校验项=cascadia 编译、必含 title、原始 HTML 上真实提取、条目>0、关键词命中统计;
  重试时把上一轮具体校验错误拼进提示词。关键词未命中时立即返回 `Verified:false`+样本不重试。
- 配置存 `system_settings` 三键(llm_base_url/api_key/model 明文入库,API 层脱敏回显);
  出网有 SSRF 校验(`validateOutboundURL`)。
- 前端:设置页"先保存再测试";规则页 AI 提取单独 180s 超时,回填表单,
  `Verified:false` 黄牌警示,任意编辑清除结果。

**已确认的问题 → 对策映射**：

| # | 问题（事实） | 对策落点 |
|---|---|---|
| L1 | HTML 按**字节**截断，可切断多字节汉字 | §5.2 Normalize 步骤改为 rune 安全截断 |
| L2 | 固定从页首截 60KB,列表在后半页即丢失 | §5.2 Normalize 增加"候选区域预筛"(按关键词锚点定位重复结构邻域)，只送相关片段 |
| L3 | 无网络层重试,瞬时故障整体失败 | §5.3 Provider 包装层做指数退避重试(≤2 次,仅幂等错误码) |
| L4 | JSON 解析取首个 `{` 到末个 `}`,杂文含花括号会截错 | §5.3 平衡括号扫描 + 围栏剥离的鲁棒解析器 |
| L5 | 非法 type 静默改 text;字段名不查重 | §5.2 Validator 增加草稿规范化层,产出 diagnostics 而非静默修正 |
| L6 | 全程零日志:无耗时/token/尝试次数/原始输出 | §5.3 可观测钩子(P3) |
| L7 | 单一全局接入,temperature/stream 硬编码 | §5.3 Provider 接口 + Request 参数化 |
| L8 | prompt 硬编码在 monitor 包内 | §5.3 prompt 资产化集中管理 |
| L9 | `llm.Chat` 网络错误直接 return,不消耗也不使用重试机会 | §5.2 管线统一错误分类:可重试/不可重试 |

**可直接复用的资产**:`verifyAIProposal` 的校验逻辑与"错误反馈回填提示词"机制
正是 §5.2 Validate⇄Feedback 环节的雏形,重构是把它从 AI 专用提升为管线公共设施。

### 2.3 前端流程与 API 消费（已核实）

**现状事实**：

- 路由 6 页无 query/state 传参;`useResource` 三态模式仅 Dashboard 使用,
  其余 5 页手写加载状态;双 toast 体系(`useToastMessages` 与 Settings 自建)并存;
  `client.ts` 拦截器已保证 code===0 才进 .then,但各页仍普遍冗余判断。
- 「按规则识别」:`previewScan({url})` 返回候选数组(含 config 完整可回填);
  恰好一个候选时自动套用,多个渲染卡片人工选,未命中引导建规则。流程健康。
- 「高级规则」页 882 行:本地 `RuleDraft` 状态模型独立于监控表单;
  scope 可用性由 URL 解析实时派生(此逻辑质量不错);保存走 quickCreate。
- 「测试提取」实现是**拼一个假监控请求**(interval=3600、is_active=false)
  打 `/monitors/validate` ——草稿验证被迫伪装成监控配置。

**问题 → 对策映射**：

| # | 问题（事实） | 对策落点 |
|---|---|---|
| F1 | 规则库只增删不可改:后端有 `PUT /settings/scan-rules/:id`,前端 `updateScanRule` 零调用、无编辑 UI | §7 补规则编辑流(配合 revision 血缘) |
| F2 | 测试提取 = 假请求打 /monitors/validate | §5.2 CapturePipeline 提供 draft 直测能力,前端改调 |
| F3 | 字段编辑器、quickCreate payload、JSON API 配置块各两份重复;Esc 弹窗 ×3 | §7 抽共享组件/工具 |
| F4 | `suggestedScanRuleScope()` 恒返 'exact'(逻辑被掏空) | §5.1 Matcher 就绪后恢复真实的范围建议 |
| F5 | AddMonitor 用两个从不读取的 state 模拟 Vue watch;每渲染全量指纹计算 | §7 表单状态整理(不阻塞主线) |
| F6 | 零组件测试:AI 回填、候选套用、导入导出均无覆盖 | §8 各阶段验收含关键交互测试 |
| F7 | window.confirm/prompt 与自绘弹窗并存 | 低优先,随规则编辑流顺带统一 |

**可直接复用的资产**:previewScan 的"单候选自动套用/多候选卡片选择/空态引导"
三段式交互成熟稳定,捕获管线的结果确认视图应延续同一模式;
ScanRuleManagement 的 scope 派生预览(scopeSummary)质量好,保留。

## 3. 设计原则

1. **捕获是人发起的动作，产物是草稿**。任何"发现规则"的能力（LLM 或本地启发式）都只输出
   候选草稿 + 样本 + 诊断信息，必须经用户确认才写入规则库。这与历史上被删除的"运行时启发式
   猜测"有本质区别：后者在无人监督的检查循环里做不可预测的事，前者是显式、一次性、可预览的操作。
2. **提取必须是纯函数**。`Extract(profile, page) → []Item` 无隐藏状态、无 IO，抓取由调用方完成。
   运行时、配置验证、捕获管线的样本生成共用同一实现。
3. **匹配是可组合的策略**。四种 scope（exact/route/section/global）各自是一个无状态 Matcher，
   由统一的入口按优先级编排，禁止 if/else 散布。
4. **LLM 是可替换的提案器，不是基础设施**。业务依赖 `Proposer` 接口而不是某个 SDK；
   prompt 是版本化资产；每次调用可观测（耗时/成败/token）。
5. **失败反馈闭环**。捕获管线中每一步校验失败都要把结构化的失败原因喂回给提案器重试，
   而不是简单报错给用户。
6. **分阶段停靠**。每个阶段结束时仓库都是可构建、测试全绿的状态。

## 4. 目标架构

```
┌────────────────────────────────────────────────────────────────┐
│                        Gentry 单二进制                          │
│                                                                │
│  Web 层 (Gin)                                                  │
│    /api/v1/monitors/*        监控 CRUD / 手动检查 / 配置验证      │
│    /api/v1/settings/scan-rules/*        规则 CRUD / 测试 / 导入导出│
│    POST /settings/scan-rules/capture   ★ 统一捕获端点            │
│                                                                │
│  ┌────────────────────── 规则域 ──────────────────────┐         │
│  │ Rule        定义、字段、scope、指纹                  │         │
│  │ Matcher     exact | route | section | global      │         │
│  │ Store       scan_rule_templates (+ revision)       │         │
│  └──────────────┬─────────────────────────────────────┘         │
│                 │ 按 URL 命中（候选按优先级排序）                  │
│  ┌──────────────▼─────────────┐   ┌──────────────────────────┐ │
│  │ Extractor（纯函数）          │   │ Capture Pipeline（人发起）│ │
│  │  HTML: container/item/field │   │  Fetch → Propose →       │ │
│  │  JSON: items_path/filter    │   │  Validate ⇄ 反馈重试 →    │ │
│  │  → []Item                   │   │  RuleDraft + Samples     │ │
│  └──────────┬──────────────────┘   │      ↓ 用户确认           │ │
│             │ 监控保存副本           │      入规则库             │ │
│  ┌──────────▼──────────────────┐   └───────────┬──────────────┘ │
│  │ Monitor Runtime              │               │ Proposer 接口  │
│  │  ticker → fetch → extract →  │               │ ├ LLMProposer │
│  │  compare(title+link) →       │               │ └ Heuristic(可选)│
│  │  save → 同步通知              │───────────────┘                │
│  └──────────────────────────────┘                                │
│  数据层：SQLite（Site/SiteField/UpdateRecord/ScanRule*/…）        │
└────────────────────────────────────────────────────────────────┘
```

### 包结构调整（两档策略）

| 档位 | 做法 | 说明 |
|---|---|---|
| 第一档（随 §8 P1） | 保持 `monitor` 单包，包内按文件分层整理，抽出 `Matcher`/`Extractor` 明确接口 | 零迁移成本，先立骨架 |
| 第二档（随 §8 P4，可选） | 物理拆包 `rule/`、`extract/`、`capture/`，`monitor/` 只留运行时 | 待第一档稳定后执行 |

## 5. 三大流程的目标形态

### 5.1 规则命中获取内容

- `Matcher` 接口：`Match(scope ScopeInfo, rawURL string) bool`，四个实现 +
  `MatchRules(url) []ScoredRule` 编排器（scope 过滤 → 优先级排序 → 候选生成）。
- 运行时不变：监控仍使用保存的选择器副本（隔离性是正确的），但增加血缘：
  `Site.SourceRuleID` + `SourceRuleRevision`（可空列），编辑表单展示"来自规则 X@vN，
  规则已有更新"提示，一键采纳为新的候选配置（仍走确认流程）。
- `Extractor` 收敛为纯函数集，`ValidateExtraction`（配置验证）与捕获管线样本复用同一实现。

### 5.2 URL + 关键字捕获新规则（统一捕获管线）

```text
POST /api/v1/settings/scan-rules/capture
{ "url": "...", "keywords": ["公告", "公示"], "proposer": "llm" | "auto" }

CapturePipeline.Run(ctx, req):
  1. Fetch        抓取页面（HTML 或公开 JSON API），出网安全校验
  2. Normalize    清洗 DOM/文本，截断到预算内
  3. Propose      Proposer 接口产出 RuleDraft（选择器 + 字段映射）
  4. Validate     本地验证（全部复用 Extractor）：
                    ✓ 选择器可编译  ✓ 条目数 ≥ 1  ✓ title 字段非空
                    ✓ 关键词命中率 ≥ 阈值  ✓ （可选）二次抓取结果一致
  5. Feedback     校验失败 → 结构化错误（哪一步、为什么、观测值）→ Proposer 重试 ≤ 2
  6. Result       CaptureResult{ drafts []RuleDraft, samples, diagnostics }
                  —— 仅返回草稿，不入库；用户在前端确认后走既有 quickCreate 保存
```

- `proposer=auto` 时策略：关键词强命中且 DOM 结构规整 → 先试本地启发式（确定性、毫秒级）；
  否则直接 LLM。两条路产出的都是同一 `RuleDraft`，验证标准完全一致。
- 启发式提案器（如实施）必须满足：纯函数、固定输入→固定输出、单测覆盖典型列表页结构、
  失败静默降级到下一提案器。它只服务捕获，绝不进入运行时检查循环。

### 5.3 LLM 辅助创建规则

- `llm.Provider` 接口：`Complete(ctx, Request) → Response`，OpenAI 兼容实现保留，
  预留多 Provider 注册；请求级超时与取消贯通。
- 结构化输出：prompt 要求 JSON + 本地鲁棒解析（剥 ```json 围栏、前后缀修剪、
  首个平衡 JSON 对象截取），可选 json_mode 参数透传。
- Prompt 资产化：集中常量管理并附版本注释，输入截断策略显式可配。
- 可观测：每次调用记录耗时、成败、错误分类、token 用量（响应里有的话）到日志；
  为后续"设置页显示最近 N 次 AI 调用记录"留好钩子。
- 现有 `/ai-extract` 保留一个兼容期，内部转发到 CapturePipeline（`proposer=llm`）。

## 6. 数据模型与 API 演进

- `sites` 新增可空列 `source_rule_id INTEGER`、`source_rule_revision INTEGER`
  （AutoMigrate 自动加列，老数据不受影响）。
- `scan_rule_templates` 新增 `revision INTEGER DEFAULT 1`；任何编辑递增，用于血缘提示。
- preview 候选响应增加 `rule_id`（现只有规则名 Strategy 字段），
  使"从候选创建监控"能回传来源，形成完整血缘链。
- 新端点仅一个：`POST /api/v1/settings/scan-rules/capture`。
- 其余 API 不变；前端 `quickCreateScanRule` 成为草稿确认后的唯一落库通道；
  血缘写入路径：前端从候选带回 `source_rule_id` → addMonitor 请求透传 → dbSiteFromRequest 落列。

## 7. 前端改造

- 「高级规则」页：「AI 提取」按钮升级为「智能捕获」，关键词输入提升为一级表单项；
  结果区展示草稿对比视图（选择器/样本/诊断信息），确认后调 quickCreate；
  「测试提取」改调捕获管线的草稿直测能力（替代假请求打 /monitors/validate）。
- **补齐规则编辑流**：后端 `PUT /settings/scan-rules/:id` 已存在，前端新增编辑入口，
  保存时 revision 递增，为血缘提示提供数据基础。
- 「新增监控」页：识别流程不变；若监控带 `source_rule_id`，详情页展示血缘徽标与
  "规则已更新"提醒。
- 共享层收敛：字段编辑器、quickCreate payload 组装、JSON API 配置块各保留一份实现
  供监控表单与规则页共用；toast 统一到 `useToastMessages`。
- 类型与 API client 增加 `CaptureResult/RuleDraft` 定义；`suggestedScanRuleScope`
  基于 Matcher 恢复真实建议逻辑。
- 测试：为"捕获结果确认回填""候选自动套用""规则编辑保存"补充组件级测试。

## 8. 分阶段实施计划

| 阶段 | 内容 | 验收 |
|---|---|---|
| P1 骨架 | Matcher/Extractor/JSON 提取统一接口化(改动面已确认很小);S3 去重键统一;S4 三份"构配置→提取→组装"收敛到捕获管线雏形;S2 修复启发式隔离成 compat 文件并加特征化测试锁行为;S5 字段互转集中 | 全部现有测试绿 + 新增接口单测/特征化测试 |
| P2 管线 | CapturePipeline + Proposer(LLM) + 验证反馈闭环 + `/capture` 端点；`/ai-extract` 兼容转发 | 端到端：真实页面 → 草稿 → 保存 → 建监控行通 |
| P3 血缘与观测 | source_rule_id/revision 列 + 升级提示；LLM 调用日志；前端血缘徽标 | 老库升级迁移无损；提示链路可用 |
| P4（可选） | 物理拆包 rule//extract//capture；启发式提案器 | 拆包后构建/测试绿 |

风险与对策：P2 的 prompt 质量决定捕获体验——用 ai_extract_test 的既有用例做回归基线；
P4 拆包机械量大收益低，放最后视情况取舍。

## 9. 兼容性承诺

- 现有 API 除新增外不签名变更；`/ai-extract` 行为保持到 P3 结束；
- SQLite 老库启动即自动加列，无需手工迁移；
- 规则删除不影响已建监控（副本语义延续）。
