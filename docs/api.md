# API 文档

## 基本约定

- API 前缀：`/api`
- 监控器接口前缀：`/api/v1/monitors`
- 内容类型：`application/json`
- 成功响应：`{"code":0,"message":"success","data":...}`
- 失败响应：`{"code":<错误码>,"message":"<错误信息>","data":null}`

如果设置了 `ALTERBOT_AUTH_TOKEN`，所有 `/api` 请求都需要提供：

```http
Authorization: Bearer <token>
```

## 系统接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/health` | 健康检查 |
| `GET` | `/api/v1/stats` | 系统统计 |
| `GET` | `/api/v1/groups` | 监控分组 |
| `GET` | `/api/v1/settings/notifications` | 获取全局通知设置 |
| `PUT` | `/api/v1/settings/notifications` | 更新全局通知设置 |

## 监控器接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/monitors/` | 获取监控器列表 |
| `POST` | `/api/v1/monitors/` | 创建监控器 |
| `GET` | `/api/v1/monitors/:name` | 获取运行状态 |
| `GET` | `/api/v1/monitors/:name/config` | 获取完整配置 |
| `PUT` | `/api/v1/monitors/:name` | 更新配置 |
| `DELETE` | `/api/v1/monitors/:name` | 删除监控器及关联状态 |
| `POST` | `/api/v1/monitors/:name/start` | 启动监控器 |
| `POST` | `/api/v1/monitors/:name/stop` | 停止监控器 |
| `POST` | `/api/v1/monitors/:name/check` | 立即检查 |
| `GET` | `/api/v1/monitors/:name/updates` | 获取更新历史记录 |
| `PUT` | `/api/v1/monitors/:name/notify-accounts` | 更新通知账户 |
| `PUT` | `/api/v1/monitors/:name/mark-all-notified` | 标记全部已通知 |
| `POST` | `/api/v1/monitors/:name/mark-read` | 标记记录已读 |

创建和更新请求需要包含名称、URL、容器/条目选择器、字段和检查间隔等完整监控配置。推荐先调用 `/validate`，或者直接通过 Web 管理界面创建。

## 配置辅助接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/monitors/validate` | 抓取并验证监控配置，不写入基线 |
| `POST` | `/api/v1/monitors/preview` | 按已保存扫描规则识别网页内容结构 |
| `POST` | `/api/v1/settings/scan-rules/capture` | 规则捕获：按 URL + 关键词产出候选规则草稿（LLM 提案 + 本地验证），草稿需人工确认后保存 |
| `POST` | `/api/v1/settings/scan-rules/test-draft` | 草稿直测：不落库按选择器配置执行一次只读提取 |

`capture` 请求体为 `{"url": "...", "keywords": "公告,公示"}`（keywords 可省略）。
返回 `config`（container/item/fields 候选）、`samples` 样本、`verified` 是否通过关键词验证、
`message` 说明，以及 `diagnostics`（尝试次数、每轮失败原因、关键词命中数、条目数）。
验证失败会携带具体错误自动反馈重试一次；鉴权类错误立即返回。
需要在「设置」页完成 AI 模型接入配置。

`test-draft` 请求体为 `{"url", "container", "item", "fields", "fetch_config"?}`，
响应结构与 `/monitors/validate` 一致。

`preview` 请求体只需要 `url`。返回命中规则的候选列表（含样本条目与可保存的监控配置）；未命中规则时返回空列表。

## 通知账户

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings/notification-accounts` | 获取通知账户 |
| `POST` | `/api/v1/settings/notification-accounts` | 创建通知账户 |
| `PUT` | `/api/v1/settings/notification-accounts/:id` | 更新通知账户 |
| `DELETE` | `/api/v1/settings/notification-accounts/:id` | 删除通知账户 |
| `GET` | `/api/v1/settings/notification-providers` | 获取通知服务元数据 |

## 扫描规则模板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings/scan-rules` | 获取模板列表 |
| `POST` | `/api/v1/settings/scan-rules/quick` | 根据提取配置快速保存规则 |
| `GET` | `/api/v1/settings/scan-rules/export` | 导出版本化规则 JSON |
| `POST` | `/api/v1/settings/scan-rules/import` | 导入规则 JSON；同名规则跳过 |
| `POST` | `/api/v1/settings/scan-rules` | 创建模板 |
| `PUT` | `/api/v1/settings/scan-rules/:id` | 更新模板 |
| `DELETE` | `/api/v1/settings/scan-rules/:id` | 删除模板 |
| `POST` | `/api/v1/settings/scan-rules/:id/test` | 测试模板 |
| `POST` | `/api/v1/settings/scan-rules/ai-extract` | AI 提取规则：抓取页面，让 AI 识别内容列表结构并本地验证 |

## AI 模型接入

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings/llm` | 获取接入配置（api_key 脱敏返回） |
| `PUT` | `/api/v1/settings/llm` | 保存接入配置；api_key 传回脱敏值时保留原密钥 |
| `POST` | `/api/v1/settings/llm/test` | 向已配置模型发送一次最小对话验证连通 |

配置为 OpenAI 兼容接口（Base URL + API Key + 模型名称），支持 DeepSeek、通义千问、Moonshot、OpenAI、Ollama 等。

`ai-extract` 请求体为 `{"url": "...", "keywords": "公告,公示"}`。关键词是正确条目应包含的文字，用于辅助 AI 定位内容区域（可留空）。返回建议的 `config`（container/item/fields）、本地提取的 `samples`、是否通过关键词验证的 `verified` 与说明 `message`。AI 识别的是 HTML 规则；JSON API 规则仍需手动配置。

快速保存请求示例：

```json
{
  "name": "招考录用列表",
  "url": "https://example.com/notices/?a=dir&c=100",
  "scope_type": "exact",
  "config": {
    "container": "ul.notice-list",
    "item": "li",
    "fields": [
      { "name": "title", "selector": "a", "type": "text" },
      { "name": "url", "selector": "a", "type": "attr", "attr": "href" }
    ]
  }
}
```

`scope_type` 支持：

- `exact`：主机、路径和规范化查询参数完全匹配；默认值。
- `route`：主机相同且路径位于同一路由；源 URL 有查询参数时查询参数也必须一致。
- `section`：主机相同，匹配源页面父目录下的页面。
- `global`：不限制 URL，按页面结构匹配。

JSON API 规则的 `config` 还会包含 `fetch_config`：

```json
{
  "mode": "api_json",
  "url": "https://shop.example/api/list?id={{goods_id}}",
  "items_path": "data",
  "filter_path": "is_selling",
  "filter_equals": "true",
  "headers": {
    "Accept": "application/json",
    "Referer": "{{page_url}}",
    "X-Requested-With": "XMLHttpRequest"
  },
  "variables": {
    "goods_id": {
      "source": "html",
      "selector": "#goods_id",
      "attr": "value"
    }
  }
}
```

`headers` 仅允许 `Accept`、`Accept-Language`、`Referer` 和 `X-Requested-With`。`variables` 从当前页面提取动态值；`{{page_url}}` 是当前页面 URL。API URL 仍受出站地址安全校验约束，模板变量不能用于协议或主机。

导出接口返回的数据部分格式为：

```json
{
  "version": 1,
  "exported_at": "2026-07-24T18:25:58+08:00",
  "rules": []
}
```

导入接口直接接收上述数据对象，单次允许 1 到 500 条规则。导入会校验规则名称、CSS 选择器、`title` 字段和适用范围，并返回 `imported` 与 `skipped` 数量。
