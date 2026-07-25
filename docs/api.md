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
| `GET` | `/api/health` | 健康检查 |
| `GET` | `/api/stats` | 系统统计 |
| `GET` | `/api/groups` | 监控分组 |
| `GET` | `/api/settings/notifications` | 获取全局通知设置 |
| `PUT` | `/api/settings/notifications` | 更新全局通知设置 |

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
| `POST` | `/api/v1/monitors/:name/baseline` | 重置基线 |
| `GET` | `/api/v1/monitors/:name/updates` | 获取旧版新增记录 |
| `GET` | `/api/v1/monitors/:name/events` | 获取变化事件 |
| `GET` | `/api/v1/monitors/:name/snapshots` | 获取当前快照 |
| `PUT` | `/api/v1/monitors/:name/notify-accounts` | 更新通知账户 |
| `PUT` | `/api/v1/monitors/:name/mark-all-notified` | 标记全部已通知 |
| `POST` | `/api/v1/monitors/:name/mark-read` | 标记记录已读 |

## 配置辅助接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/monitors/validate` | 抓取并验证监控配置，不写入基线 |
| `POST` | `/api/v1/monitors/preview` | 扫描网页候选区域 |
| `POST` | `/api/v1/monitors/smart-create` | 根据扫描结果创建监控 |

## 通知账户

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/settings/notification-accounts` | 获取通知账户 |
| `POST` | `/api/settings/notification-accounts` | 创建通知账户 |
| `PUT` | `/api/settings/notification-accounts/:id` | 更新通知账户 |
| `DELETE` | `/api/settings/notification-accounts/:id` | 删除通知账户 |
| `GET` | `/api/settings/notification-providers` | 获取通知服务元数据 |

## 扫描规则模板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/settings/scan-rules` | 获取模板列表 |
| `POST` | `/api/settings/scan-rules/quick` | 根据预扫描候选快速保存规则 |
| `GET` | `/api/settings/scan-rules/export` | 导出版本化规则 JSON |
| `POST` | `/api/settings/scan-rules/import` | 导入规则 JSON；同名规则跳过 |
| `POST` | `/api/settings/scan-rules` | 创建模板 |
| `PUT` | `/api/settings/scan-rules/:id` | 更新模板 |
| `DELETE` | `/api/settings/scan-rules/:id` | 删除模板 |
| `POST` | `/api/settings/scan-rules/:id/test` | 测试模板 |

快速保存请求示例：

```json
{
  "name": "招考录用列表",
  "url": "https://example.com/notices/?a=dir&c=100",
  "keywords": "招聘,公告",
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

JSON API 候选的 `config` 还会包含 `fetch_config`：

```json
{
  "mode": "api_json",
  "url": "https://shop.example/api/skus?id={{goods_id}}",
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

## 到价提醒策略示例

以下片段表示价格从目标价以上降到 `199.00` 或以下时产生事件：

```json
{
  "strategy_type": "field_transition",
  "strategy_config": {
    "type": "field_transition",
    "identity": { "source": "source_url" },
    "conditions": [
      {
        "field": "price",
        "value_type": "money",
        "operator": "at_or_below",
        "threshold": { "value": "199.00" }
      }
    ],
    "on_first_baseline": "silent"
  },
  "field_data_types": {
    "price": "money"
  }
}
```

创建和更新请求还需要包含名称、URL、选择器、字段和检查间隔等完整监控配置。推荐先调用 `/validate`，或者直接通过 Web 管理界面创建。
