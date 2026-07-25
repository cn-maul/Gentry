# Gentry

Gentry 是一个可自托管的网页变化监控系统。它定时抓取网页或公开 JSON API，通过 CSS 选择器或 JSON 路径提取结构化字段，比较前后状态，并在符合规则时通过 PushPlus、Webhook、Server酱等渠道发送通知。

项目由 Go 后端和 Vue 3 管理界面组成，前端会嵌入最终二进制，默认使用 SQLite 保存监控配置、快照、变化事件和通知投递状态。

## 核心能力

- 网页新增监控：适用于公告、新闻、更新日志、商品上新等列表内容。
- 价格下降监控：价格低于上一次有效价格时触发，可设置最低降价金额或百分比。
- 到价提醒：价格从目标价以上降到目标价或以下时触发；持续低于目标价不会重复通知。
- 智能预扫描：输入 URL 和关键词，自动发现候选内容区域并生成容器、条目和字段选择器。
- 扫描规则库：将确认后的扫描结果保存为页面级、路由级或跨网站通用结构规则，并支持 JSON 导入导出。
- 稳定基线：首次检查只建立基线，不发送历史内容或已有低价通知。
- 结构化提取：使用容器、列表项和字段 CSS 选择器提取文本或属性。
- API 价格源：可从公开 JSON 接口提取 SKU 和价格，支持数组路径、在售过滤和受控请求头。
- 可靠投递：变化事件与通知任务持久化，支持异步投递、重试、去重和状态追踪。
- 多渠道通知：支持 PushPlus、Webhook、Server酱和 Bark，可按监控选择账户并使用关键词过滤推送。
- Web 管理：通过浏览器创建、验证、编辑、启停、手动检查和重建监控基线，并查看快照与事件历史。
- 单二进制部署：生产构建会把 Vue 前端嵌入 Go 程序，也支持 Docker Compose。

## 适用范围

Gentry 适合监控无需登录即可访问的 HTML 页面或公开 JSON API。对于强依赖浏览器执行、验证码、复杂登录态或反爬验证且没有公开数据接口的网站，仍需要额外的抓取适配器。

## 快速开始

使用 Docker Compose 构建并启动：

```bash
docker compose up -d --build
```

打开 [http://localhost:8889](http://localhost:8889) 进入管理界面。SQLite 数据保存在 `gentry-data` Docker 卷中。

本地开发与其他部署方式请查看[快速开始](docs/getting-started.md)和[部署指南](docs/deployment.md)。

## 文档

- [文档目录](docs/README.md)
- [功能总览](docs/features.md)
- [快速开始](docs/getting-started.md)
- [监控规则说明](docs/monitoring-rules.md)
- [部署指南](docs/deployment.md)
- [API 文档](docs/api.md)
- [开发指南](docs/development.md)
- [设计与历史修订](docs/design/)

## 技术栈

- 后端：Go、Gin、GORM、SQLite、goquery
- 前端：Vue 3、Vue Router、Axios、Vite
- 部署：单二进制、Docker、Docker Compose

## License

MIT
