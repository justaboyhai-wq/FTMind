<p align="center">
  <img src="./docs/images/logo.png" alt="Keystone" height="112" />
</p>

<h1 align="center">Keystone</h1>

<p align="center">
  A self-hosted knowledge workspace for documents, RAG, Agents and living Wiki.
</p>

<p align="center">
  <a href="./README_CN.md">简体中文</a> ·
  <a href="https://github.com/justaboyhai-wq/keystone">GitHub</a> ·
  <a href="./LICENSE">MIT License</a> ·
  <a href="https://clawhub.ai/justaboyhai-wq/skills/keystone">ClawHub Skill</a>
</p>

## Overview

Keystone turns files, web pages and Markdown into a private, searchable knowledge workspace. It combines document parsing, embeddings, hybrid retrieval, streaming chat, configurable Agents and Wiki generation in one deployable application.

The project is designed for local and private-cloud deployment. Models, vector stores, object storage, parsing engines and web-search providers are independently configurable, so the stack can run with local services, compatible APIs, or managed providers.

## What you can do

- Build document, FAQ and Wiki knowledge bases from files, URLs, folders and Markdown.
- Search with dense-vector and keyword retrieval, citations and configurable reranking.
- Use Quick Q&A or Agents that can combine knowledge retrieval, MCP tools and web search.
- Generate linked Wiki pages and browse their hierarchy and knowledge graph.
- Configure chat, embedding, rerank, image, ASR and TTS models centrally.
- Run with Qdrant, pgvector, Milvus, Weaviate, Elasticsearch/OpenSearch and other supported backends.
- Store files locally, in MinIO, or in compatible object storage.
- Control access with workspaces, roles, API keys, audit information and embeddable chat.
- Connect external agents through the versioned Keystone ClawHub Skill or the REST API.

## Quick start

### Requirements

- Docker Desktop with Docker Compose
- Git

### Start Keystone

```bash
git clone https://github.com/justaboyhai-wq/keystone.git
cd keystone
cp .env.example .env
docker compose up -d
```

On Windows PowerShell, copy the environment file with:

```powershell
Copy-Item .env.example .env
```

Open [http://localhost](http://localhost), create or sign in to a workspace, then configure models from **Settings → Model Management**. The default API endpoint is `http://localhost:8080/api/v1`.

To stop the local stack:

```bash
docker compose down
```

### Optional services

Enable only the components your workspace needs:

| Profile | Purpose | Command |
| --- | --- | --- |
| Default | Core application services | `docker compose up -d` |
| `minio` | S3-compatible object storage | `docker compose --profile minio up -d` |
| `neo4j` | Wiki knowledge graph | `docker compose --profile neo4j up -d` |
| `langfuse` | Tracing and observability | `docker compose --profile langfuse up -d` |
| `full` | All optional services | `docker compose --profile full up -d` |

Profiles can be combined, for example:

```bash
docker compose --profile minio --profile neo4j up -d
```

## Configure a knowledge workspace

1. In **Settings → Storage Engine**, choose Local, MinIO, or another configured object store.
2. In **Settings → Vector Database Engine**, create or choose a vector-store connection.
3. In **Settings → Model Management**, add the chat, embedding and optional rerank models required by your workflow.
4. Create a knowledge base and select its parsing, chunking, retrieval and storage options.
5. Upload documents or import URLs. For a Wiki knowledge base, choose a template before setting extraction granularity.
6. Open Chat, select the knowledge base and an Agent, then ask a question.

Existing deployments can keep their current data and service configuration when upgrading. Refer to [migration troubleshooting](./docs/migration-troubleshooting.md) before changing storage or vector-store identifiers.

## External-agent integration

Install the official Keystone Skill for OpenClaw or any compatible agent runtime:

```bash
openclaw skills install @justaboyhai-wq/keystone
```

Create an API key in **Settings → API Integration**, then expose a reachable API address to the agent:

```bash
export KEYSTONE_BASE_URL="http://localhost:8080/api/v1"
export KEYSTONE_API_KEY="sk-your-api-key"
```

The Skill supports file upload, URL import, Markdown authoring, knowledge browsing and hybrid retrieval. Its tracked source is [frontend/public/keystone-skill/SKILL.md](./frontend/public/keystone-skill/SKILL.md).

## Interfaces and extensions

| Interface | Entry point |
| --- | --- |
| Web workspace | `http://localhost` |
| REST API | `http://localhost:8080/api/v1` |
| CLI | [cli/README.md](./cli/README.md) |
| MCP server | [mcp-server/README.md](./mcp-server/README.md) |
| API reference | [docs/api/README.md](./docs/api/README.md) |
| Built-in models | [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) |
| Agent skills | [docs/agent-skills.md](./docs/agent-skills.md) |
| Architecture and deployment | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) |

## Development

For a local frontend development server:

```bash
cd frontend
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

The frontend proxies API requests to `http://localhost:8080` by default. Set `VITE_DEV_PROXY_TARGET` when the backend runs elsewhere.

Before contributing, run the relevant checks:

```bash
npm --prefix frontend run type-check
npm --prefix frontend run build
go test ./...
```

## Documentation policy

Keystone maintains project introductions in English and Simplified Chinese. Technical guides may retain their most useful original language while they are progressively consolidated; they describe Keystone only and do not represent any external platform or upstream service.

## License

Keystone is distributed under the [MIT License](./LICENSE). Third-party dependencies and their licenses remain subject to their own terms.
