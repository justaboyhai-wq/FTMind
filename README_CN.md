<p align="center">
  <img src="./docs/images/logo.png" alt="Keystone" height="112" />
</p>

<h1 align="center">Keystone</h1>

<p align="center">
  面向文档、RAG、智能体与动态 Wiki 的私有化知识工作台。
</p>

<p align="center">
  <a href="./README.md">English</a> ·
  <a href="https://github.com/justaboyhai-wq/keystone">GitHub</a> ·
  <a href="./LICENSE">MIT 许可</a> ·
  <a href="https://clawhub.ai/justaboyhai-wq/skills/keystone">ClawHub Skill</a>
</p>

## 项目介绍

Keystone 将文件、网页和 Markdown 沉淀为可私有部署、可持续演进的知识工作台。它把文档解析、向量化、混合检索、流式对话、智能体编排和 Wiki 构建整合在同一应用中。

项目适用于本地与私有云部署。模型、向量数据库、对象存储、解析引擎和网络搜索服务均可独立配置，因此既可对接本地服务，也可接入兼容 API 或托管服务。

## 核心能力

- 通过文件、URL、文件夹与 Markdown 构建文档、问答和 Wiki 知识库。
- 使用向量 + 关键词混合检索、引用展示与可选重排模型获得可追溯的回答。
- 使用快速问答或可组合知识检索、MCP 工具与网络搜索的智能体完成任务。
- 自动生成相互关联的 Wiki 页面，并浏览层级结构与知识图谱。
- 集中配置对话、向量、重排、文生图、ASR 与 TTS 等模型。
- 支持 Qdrant、pgvector、Milvus、Weaviate、Elasticsearch/OpenSearch 等向量后端。
- 支持本地存储、MinIO 与兼容对象存储服务。
- 通过空间、角色、API Key、审计信息和嵌入式聊天管理访问范围。
- 通过 Keystone ClawHub Skill 或 REST API 为外部 Agent 提供知识库能力。

## 快速开始

### 环境要求

- Docker Desktop（含 Docker Compose）
- Git

### 启动 Keystone

```bash
git clone https://github.com/justaboyhai-wq/keystone.git
cd keystone
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

## 外部 Agent 接入

为 OpenClaw 或兼容的 Agent 运行环境安装官方 Keystone Skill：

```bash
openclaw skills install @justaboyhai-wq/keystone
```

在「设置 → API 集成」创建 API Key，并在 Agent 运行环境中配置可访问的地址：

```bash
export KEYSTONE_BASE_URL="http://localhost:8080/api/v1"
export KEYSTONE_API_KEY="sk-your-api-key"
```

Skill 支持上传文件、导入 URL、写入 Markdown、浏览知识库以及混合检索。其版本化源文件为 [frontend/public/keystone-skill/SKILL.md](./frontend/public/keystone-skill/SKILL.md)。

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

## 本地开发

启动前端开发服务：

```bash
cd frontend
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

前端默认将 API 请求代理到 `http://localhost:8080`；如后端位于其他地址，请设置 `VITE_DEV_PROXY_TARGET`。

提交前建议运行与改动相关的检查：

```bash
npm --prefix frontend run type-check
npm --prefix frontend run build
go test ./...
```

## 文档策略

Keystone 的项目介绍仅维护英文与简体中文版本。技术文档会在保证可用性的前提下逐步收敛，但只描述 Keystone 本身，不代表任何外部平台或上游服务。

## 许可

Keystone 采用 [MIT License](./LICENSE) 发布。第三方依赖及其许可仍以各自条款为准。
