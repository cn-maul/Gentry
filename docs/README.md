# Gentry 文档

## 使用文档

- [功能总览](features.md)：当前版本已经提供的页面、监控、扫描规则、通知和持久化能力。
- [快速开始](getting-started.md)：启动项目并创建第一个监控项。
- [监控规则说明](monitoring-rules.md)：新增检测、价格下降、到价提醒、身份字段和基线语义。
- [部署指南](deployment.md)：Docker、本地二进制、环境变量和数据持久化。
- [API 文档](api.md)：接口约定、认证方式和主要端点。
- [开发指南](development.md)：开发环境、构建测试、目录结构和核心链路。

## 架构设计

- [系统架构](design/architecture.md)：系统概览、模块职责、数据模型、核心数据流、并发模型和设计决策。

## 设计档案

`design/` 保存通用监控引擎的原始设计和历次代码审查记录。这些文件用于追溯设计决策，不代表当前版本仍存在其中列出的所有问题。

- [通用监控引擎改造方案](design/generic-monitor.md)
- [第一轮实现审查与修订](design/generic-monitor-revision.md)
- [第二轮复验与修订](design/generic-monitor-revision-v2.md)
- [第三轮复验与收口](design/generic-monitor-revision-v3.md)
- [最终收口方案](design/generic-monitor-final-cleanup.md)
- [界面视觉设计说明](design/spotify.md)
