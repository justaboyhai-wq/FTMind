# FTMind

[English](./README.md)

FTMind 是一个可私有化部署的知识工作空间，面向政策研究、文档智能和证据优先问答。它把文档解析、混合检索、流式对话、智能体、Wiki、共享空间和回答反馈治理放进同一套产品中。

请将数据库、Redis、模型、对象存储和 API 密钥放在环境变量或密钥管理系统中，不要提交到 GitHub。

## 产品核心逻辑

~~~text
政策原文 / 附件 / URL
          ↓
解析 → 分块 → 索引 → 带引用检索
          ↓
普通问答 / 政策问答智能体
          ↓
收藏或反馈错误
          ↓
管理员审核 → 定位根因 → 修复 → 隔离复测 → 知识优化
~~~

FTMind 将内容分为三类：官网明确的 official 字段、规则计算的 computed 字段，以及必须带证据和审核状态的 derived_ai 分析字段。模型可以增加官网没有的新维度，但不能伪造或覆盖官网标签。

## 主要能力

- 知识库：文件、文件夹、URL、Markdown、FAQ 和结构化导入。
- 文档解析：DocReader 处理 PDF、Office、HTML、图片等格式。
- 混合检索：向量、关键词、重排、引用和来源定位。
- Wiki：链接页面、层级浏览、问题记录和知识图谱。
- 智能体：知识检索、MCP 工具、网络搜索、审批和受控流程。
- 宝安政策问答：官网原文、附件、官方标签、关系和来源 URL。
- 回答质量闭环：收藏、错误反馈、回答快照、管理员审核、公开回复和复测。
- 工作区与共享空间：租户隔离、角色、API Key、共享知识库和智能体。
- REST API、CLI、MCP、嵌入式聊天和多模型配置。

## 宝安政策采集器与 RSS

plugins/baoan-policy-collector 是独立 Go 模块。每次运行从宝安官网入口重新发现 zcfg.js，获取详情 JSON、政策原文 HTML、附件、官方标签和显式关系，输出不可变 baoan.raw/v1 Raw Package。

~~~powershell
cd plugins/baoan-policy-collector
go test ./...
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data --incremental-cron "0 2 * * *"
go run ./cmd/baoan-policy-collector serve-rss --data-dir ./baoan-policy-data --addr :18320
~~~

一次性导入时导出规范 Markdown；持续更新时在 FTMind 知识库中配置 RSS/Atom 数据源：

~~~powershell
./scripts/export-raw.ps1 -DataDir ./baoan-policy-data -OutputDir ./raw-export
~~~

详见[宝安政策采集器文档](./plugins/baoan-policy-collector/README.md)。

## 政策 Wiki Schema

规则文件：[config/wiki_schemas/policy-qa.v1.schema.json](./config/wiki_schemas/policy-qa.v1.schema.json)。

| 命名空间 | 含义 | 规则 |
| --- | --- | --- |
| official | 官网或官方附件字段 | 原样保存值、来源和证据，禁止模型补写。 |
| computed | 申报状态等规则计算字段 | 保存算法版本、计算时间、输入证据和冲突。 |
| derived_ai | 适用条件、材料、流程和风险等分析 | 保存证据、模型、提示词版本、置信度和审核状态。 |

“当前可申报”是动态状态：综合官网申报标识、申报起止日期和计算时间判断。Wiki 更新、失效、替代、回滚和质量问题都保留操作记录，历史官方事实不会被静默覆盖。

## 问答、收藏与错误反馈

问答先检索政策原文、附件、Wiki 页面和来源 URL，再形成回答。日期、金额、资格、材料和办理流程等高风险内容必须回到原始证据核验。

收藏是用户级、空间隔离的价值信号，当前不会自动改写知识库。错误反馈冻结一次问答的快照，包括问题、回答、引用、知识库、智能体、模型和渠道。

反馈状态为：pending、reviewing、needs_info、fixing、resolved、dismissed。管理员选择根因，修复正确层级，并用原问题创建隔离复测；反馈不会直接修改官网权威字段。

## 共享空间和权限

知识库和智能体归属于原工作区。共享空间保存跨空间协作关系，成员角色和资源共享权限共同决定最终能力：

- 角色：管理员 admin、编辑者 editor、只读 viewer；
- 知识库共享：只读或可写；
- 智能体共享：只读使用；
- 实际权限是成员角色上限和资源共享权限的交集。

详见[共享空间说明](./docs/共享空间说明.md)和[RBAC 说明](./docs/RBAC说明.md)。

## Docker 快速启动

环境要求：Docker Desktop 或 Docker Engine + Compose v2；聊天模型、Embedding 模型；按场景使用 PostgreSQL、Redis、向量库和 DocReader。

~~~bash
cp .env.example .env
docker compose up -d
~~~

Windows PowerShell：

~~~powershell
Copy-Item .env.example .env
docker compose up -d
~~~

浏览器打开 http://localhost，在“设置 → 模型管理”配置模型。停止服务：

~~~bash
docker compose down
~~~

## 云端演示入口

当前云端演示约定：

- FTMind 主站：http://115.191.64.43:18310；
- 登录入口：http://115.191.64.43:18310/login；
- 宝安政策 RSS：端口 18320，Feed 路径 /feed.xml，健康检查 /healthz。

推荐由现有 Nginx 对外提供 HTTPS；PostgreSQL、Redis、DocReader 和向量库保持在内网或 Docker 网络中。测试账号和密码属于环境凭据，不写入公开 README。详见[云端部署手册](./docs/CLOUD_DEMO_DEPLOYMENT_18310.md)。

## 开发与验证

~~~bash
go test ./...
npm --prefix frontend install
npm --prefix frontend run type-check
npm --prefix frontend run build
npm --prefix frontend run dev -- --host 127.0.0.1 --port 5173
~~~

前端默认把 API 代理到 http://localhost:8080；后端地址不同可设置 VITE_DEV_PROXY_TARGET。CLI 是独立 Go 模块，位于 [cli](./cli/)。

## 目录说明

| 目录 | 用途 |
| --- | --- |
| cmd/server | FTMind 后端入口 |
| frontend | Vue Web 工作区 |
| docreader | Python 文档解析服务 |
| plugins/baoan-policy-collector | 宝安政策采集、Raw Package 和 RSS |
| config/wiki_schemas | 版本化 Wiki Schema |
| internal/application/service | 反馈、Wiki、知识库和共享空间服务 |
| client、cli、mcp-server | API 客户端、CLI 和 MCP 集成 |
| deploy | 云端 Compose 和部署资产 |
| docs | 架构、运维、API 和产品文档 |

## 推荐阅读

- [English README](./README.md)
- [架构说明](./docs/ARCHITECTURE.md)
- [云端部署手册](./docs/CLOUD_DEMO_DEPLOYMENT_18310.md)
- [宝安政策采集器](./plugins/baoan-policy-collector/README.md)
- [政策 Wiki Schema](./config/wiki_schemas/policy-qa.v1.schema.json)
- [API 文档](./docs/api/README.md)
- [CLI 文档](./cli/README.md)
- [MCP 服务](./mcp-server/README.md)
- [共享空间](./docs/共享空间说明.md)

## 安全提示

请为 JWT、数据库、Redis、对象存储和模型服务配置强随机密钥；不要公开管理端口；正式环境使用 HTTPS、反向代理和最小权限账号。部署前请阅读[运维手册](./docs/DEPLOYMENT_RUNBOOK.md)。
