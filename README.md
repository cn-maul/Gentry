# Gentry

Gentry 是一个可自托管的网页新内容监控系统。它定时抓取网页或公开 JSON API，通过 CSS 选择器或 JSON 路径提取结构化字段，与历史记录比对发现新增条目，并通过 PushPlus、Webhook、Server酱等渠道发送通知。

项目由 Go 后端和 React 19 管理界面组成，前端会嵌入最终二进制，默认使用 SQLite 保存监控配置、更新历史和扫描规则。

## 核心能力

- 网页新增监控：适用于公告、新闻、更新日志、上新等列表内容，按「标题 + 链接」去重。
- 纯规则驱动：内容识别完全依赖已保存的扫描规则（无内置启发式猜测），同一站点识别结果稳定一致。
- 扫描规则库：手动创建页面级、路由级、同站目录或跨网站通用结构规则，支持测试提取与 JSON 导入导出。
- 稳定基线：首次检查只建立基线，不把页面已有内容当作新内容推送。
- 结构化提取：使用容器、列表项和字段 CSS 选择器提取文本或属性。
- JSON API 数据源：可从公开 JSON 接口提取列表条目，支持数组路径、过滤条件、动态参数和受控请求头。
- 多渠道通知：支持 PushPlus、Webhook、Server酱和 Bark，可按监控选择账户并使用关键词过滤推送。
- Web 管理：通过浏览器创建、验证、编辑、启停、手动检查监控，并查看更新历史。
- 单二进制部署：生产构建会把 React 前端嵌入 Go 程序，也支持 Docker Compose。

## 适用范围

Gentry 适合监控无需登录即可访问的 HTML 页面或公开 JSON API。对于强依赖浏览器执行、验证码、复杂登录态或反爬验证且没有公开数据接口的网站，仍需要额外的抓取适配器。

## 快速开始

使用 Docker Compose 构建并启动：

```bash
docker compose up -d --build
```

打开 [http://localhost:8080](http://localhost:8080) 进入管理界面。SQLite 数据保存在 `gentry-data` Docker 卷中。

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
- 前端：React 19、TypeScript、Tailwind CSS 4、React Router、Axios、Vite
- 部署：单二进制（前端嵌入）、Docker、Docker Compose

## License

MIT
