# Keystone

[English](./README.md)

## 项目介绍

Keystone 将文件、网页和 Markdown 沉淀为可私有部署、可持续演进的知识工作台。它把文档解析、向量化、混合检索、流式对话、受控智能体和 Wiki 构建整合在同一应用中。

这是一个面向私有部署和内部维护的受控应用仓库：仅通过获准的基础设施部署，使用受管凭据配置外部服务，并将业务与运行数据保留在指定环境内。模型、向量数据库、对象存储、解析引擎和网络搜索服务均可独立配置。

## 核心能力

- 通过文件、URL、文件夹与 Markdown 构建文档、问答和 Wiki 知识库。
- 使用向量 + 关键词混合检索、引用展示与可选重排模型获得可追溯的回答。
- 使用快速问答或可组合知识检索、MCP 工具与网络搜索的智能体完成任务。
- 自动生成相互关联的 Wiki 页面，并浏览层级结构与知识图谱。
- 集中配置对话、向量、重排、视觉语言与 ASR 等模型。
- 支持 Qdrant、pgvector、Milvus、Weaviate、Elasticsearch/OpenSearch 等向量后端。
- 支持本地存储、MinIO 与兼容对象存储服务。
- 通过空间、角色、API Key、审计信息和嵌入式聊天管理访问范围。
- 通过 REST API、CLI 或 MCP 服务接入已获准的外部集成。

## 快速开始

当前生产环境是杭州 ECS 上的云端 MVP：应用、PostgreSQL、Qdrant 与 DocReader 使用 Docker Compose，队列使用阿里云 Tair，文件使用阿里云 OSS，模型通过受管 API 调用。新员工和运维人员应先阅读[部署与运维手册](./docs/DEPLOYMENT_RUNBOOK.md)；本节仅用于本地开发。

### 环境要求

- Docker Desktop（含 Docker Compose）

### 启动 Keystone

```bash
cp .env.example .env
docker compose up -d
```

Windows PowerShell 可使用：

```powershell
Copy-Item .env.example .env
```

启动后访问 [http://localhost](http://localhost)，创建或登录工作区，再从「设置 → 模型管理」配置模型。默认 API 地址为 `http://localhost:8080/api/v1`。

停止本地服务：

```bash
docker compose down
```

### 可选服务

按实际需要启用服务：

| Profile | 用途 | 命令 |
| --- | --- | --- |
| 默认 | 核心应用服务 | `docker compose up -d` |
| `minio` | S3 兼容对象存储 | `docker compose --profile minio up -d` |
| `neo4j` | Wiki 知识图谱 | `docker compose --profile neo4j up -d` |
| `langfuse` | 链路追踪与可观测性 | `docker compose --profile langfuse up -d` |
| `full` | 启动全部可选服务 | `docker compose --profile full up -d` |

多个 Profile 可以叠加，例如：

```bash
docker compose --profile minio --profile neo4j up -d
```

## 配置知识工作台

1. 在「设置 → 存储引擎」选择 Local、MinIO 或已配置的对象存储。
2. 在「设置 → 向量数据库引擎」创建或选择向量库连接。
3. 在「设置 → 模型管理」添加对话模型、向量模型，以及工作流需要的重排模型。
4. 新建知识库，设置解析、分块、检索与存储参数。
5. 上传文档或导入 URL。创建 Wiki 知识库时，请先选择模板，再设置提取颗粒度。
6. 进入对话，选择知识库和智能体后即可提问。

已有部署升级时可以继续使用现有数据和服务配置。更换存储或向量库标识前，请先阅读[迁移排障说明](./docs/migration-troubleshooting.md)。

## 接口与扩展

| 接口 | 入口 |
| --- | --- |
| Web 工作台 | `http://localhost` |
| REST API | `http://localhost:8080/api/v1` |
| 命令行 | [cli/README.md](./cli/README.md) |
| MCP 服务 | [mcp-server/README.md](./mcp-server/README.md) |
| API 参考 | [docs/api/README.md](./docs/api/README.md) |
| 内置模型 | [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) |
| Agent Skills | [docs/agent-skills.md](./docs/agent-skills.md) |
| 架构与部署说明 | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) |
| 生产部署与运维手册 | [docs/DEPLOYMENT_RUNBOOK.md](./docs/DEPLOYMENT_RUNBOOK.md) |
| 云端 MVP Compose | [deploy/cloud-mvp/README.md](./deploy/cloud-mvp/README.md) |
| 业务功能架构图 | [docs/diagrams/keystone-business-architecture.png](./docs/diagrams/keystone-business-architecture.png) |

## 本地开发与验证

启动前端开发服务：

```bash
cd frontend
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

前端默认将 API 请求代理到 `http://localhost:8080`；如后端位于其他地址，请设置 `VITE_DEV_PROXY_TARGET`。

先执行与改动最相关的检查，再覆盖受影响的 API、队列、Provider 或部署路径：

```bash
npm --prefix frontend run type-check
npm --prefix frontend run build
go test ./...
```

## 文档策略

项目入口文档仅维护英文与简体中文。基础设施、模型、检索引擎和后台任务的必读基线见[架构与部署说明](./docs/ARCHITECTURE.md)，修改相关能力前必须先阅读。
