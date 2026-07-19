# Keystone

[简体中文](./README_CN.md)

## Overview

Keystone turns files, web pages and Markdown into a private, searchable knowledge workspace. It combines document parsing, embeddings, hybrid retrieval, streaming chat, controlled Agents and Wiki generation in one deployable application.

This is a controlled application repository for private deployment and internal maintenance. Deploy it only through approved infrastructure, configure external services with managed credentials, and retain operational data in the intended environment. Models, vector stores, object storage, parsing engines and web-search providers are independently configurable.

## What you can do

- Build document, FAQ and Wiki knowledge bases from files, URLs, folders and Markdown.
- Search with dense-vector and keyword retrieval, citations and configurable reranking.
- Use Quick Q&A or Agents that can combine knowledge retrieval, MCP tools and web search.
- Generate linked Wiki pages and browse their hierarchy and knowledge graph.
- Configure chat, embedding, rerank, vision-language and ASR models centrally.
- Run with Qdrant, pgvector, Milvus, Weaviate, Elasticsearch/OpenSearch and other supported backends.
- Store files locally, in MinIO, or in compatible object storage.
- Control access with workspaces, roles, API keys, audit information and embeddable chat.
- Connect approved integrations through the REST API, CLI or MCP service.

## Quick start

### Requirements

- Docker Desktop with Docker Compose

### Start Keystone

```bash
cp .env.example .env
docker compose up -d
```

On Windows PowerShell:

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
| Production deployment runbook | [docs/DEPLOYMENT_RUNBOOK.md](./docs/DEPLOYMENT_RUNBOOK.md) |

## Development and verification

For a local frontend development server:

```bash
cd frontend
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

The frontend proxies API requests to `http://localhost:8080` by default. Set `VITE_DEV_PROXY_TARGET` when the backend runs elsewhere.

Run the narrowest relevant checks first, then include the affected API, queue, provider or deployment path:

```bash
npm --prefix frontend run type-check
npm --prefix frontend run build
go test ./...
```

## Documentation policy

Project entry documentation is maintained in English and Simplified Chinese. The required design and operations baseline is [Architecture and deployment](./docs/ARCHITECTURE.md); use it before changing infrastructure, models, retrieval engines or background tasks.
