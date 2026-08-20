# FTMind

[简体中文](./README_CN.md)

FTMind is a private, deployable knowledge workspace for policy research, document intelligence and evidence-first Q&A. It combines document parsing, hybrid retrieval, streaming chat, controlled Agents, linked Wiki pages, shared workspaces and an auditable answer-feedback loop.

Keep credentials in environment files or a secret manager. Never commit production keys or test passwords.

## Product lifecycle

~~~text
official sources → parse → chunk → index → retrieve with citations
                                      ↓
                              chat / Agent answer
                                      ↓
                    favorite or report an error
                                      ↓
                  admin review → repair → isolated re-test
~~~

## Core capabilities

- Knowledge bases from files, folders, URLs, Markdown, FAQ and structured imports.
- DocReader parsing for PDF, Office, HTML, images and supported formats.
- Dense-vector, keyword and hybrid retrieval with optional reranking and citations.
- Linked Wiki pages, hierarchy browsing, issue tracking and policy-specific schemas.
- Evidence-grounded Agents with knowledge retrieval, MCP tools, web search and approvals.
- Bao'an policy Q&A grounded in official originals, attachments, tags, relations and URLs.
- Answer favorites and error feedback with frozen snapshots, admin review and re-testing.
- Workspaces, tenant isolation, roles, API keys, shared spaces and audit trails.
- REST API, CLI, MCP server, embeddable chat and configurable model providers.

## Bao'an policy collector and RSS

The independent Go module at plugins/baoan-policy-collector discovers the official zcfg.js list on every run, downloads detail JSON, policy HTML, attachments and explicit relations, and writes immutable baoan.raw/v1 packages.

~~~powershell
cd plugins/baoan-policy-collector
go test ./...
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data --incremental-cron "0 2 * * *"
go run ./cmd/baoan-policy-collector serve-rss --data-dir ./baoan-policy-data --addr :18320
~~~

Export canonical Markdown for a one-time import, or configure FTMind's RSS/Atom connector for incremental synchronization.

~~~powershell
./scripts/export-raw.ps1 -DataDir ./baoan-policy-data -OutputDir ./raw-export
~~~

See [the collector guide](./plugins/baoan-policy-collector/README.md).

## Policy Wiki schema

Schema: [config/wiki_schemas/policy-qa.v1.schema.json](./config/wiki_schemas/policy-qa.v1.schema.json).

| Namespace | Meaning | Rule |
| --- | --- | --- |
| official | Official site or attachment values | Preserve value, source and evidence; never let the model fill gaps. |
| computed | Deterministic status and normalized values | Store algorithm version, calculation time, inputs and conflicts. |
| derived_ai | New analysis dimensions | Store evidence, model, prompt version, confidence and review status. |

Current-application status is dynamic: official listing signals and application dates are evaluated together at calculation time. Wiki changes, superseded statements and quality issues remain auditable.

## Answer quality loop

Each final answer can carry feedback. The backend freezes the question, answer, citations, knowledge bases, Agent, model and channel needed for review. Statuses are pending, reviewing, needs_info, fixing, resolved and dismissed, with an event timeline.

Administrators classify the root cause (source, parsing/indexing, retrieval/reranking, Wiki, Agent prompt, model, policy update, insufficient context or non-system issue), repair the correct layer and re-test the original question in an isolated request. Feedback never directly publishes a policy or Wiki change without administrator confirmation. Favorites are user-level signals, not automatic knowledge writes.

## Shared spaces and permissions

Resources remain owned by their original workspace. Shared spaces record collaboration, while member role and resource share permission are evaluated together:

- roles: admin, editor, viewer;
- knowledge-base share: read or write;
- Agent share: read-only use;
- effective capability: intersection of role and resource permission.

See [shared spaces](./docs/共享空间说明.md) and [RBAC](./docs/RBAC说明.md).

## Docker quick start

Requirements: Docker Desktop or Docker Engine with Compose v2, a chat model, an embedding model, PostgreSQL, Redis, a vector store and DocReader as required by the profile.

~~~bash
cp .env.example .env
docker compose up -d
~~~

PowerShell:

~~~powershell
Copy-Item .env.example .env
docker compose up -d
~~~

Open http://localhost, sign in to a workspace and configure models under Settings → Model Management. Stop with docker compose down. Optional profiles include minio, neo4j, langfuse and full.

~~~bash
docker compose --profile minio --profile neo4j up -d
~~~

## Cloud demo convention

The current demo uses port 18310 for the FTMind web entry and port 18320 for the Bao'an policy RSS gateway. Keep PostgreSQL, Redis, DocReader and vector stores private behind the existing Nginx reverse proxy. See [cloud deployment](./docs/CLOUD_DEMO_DEPLOYMENT_18310.md).

## Development and verification

~~~bash
go test ./...
npm --prefix frontend install
npm --prefix frontend run type-check
npm --prefix frontend run build
npm --prefix frontend run dev -- --host 127.0.0.1 --port 5173
~~~

The frontend proxies API requests to http://localhost:8080 by default. Set VITE_DEV_PROXY_TARGET when the backend runs elsewhere. The CLI is a separate Go module under [cli](./cli/).

## Repository map

| Path | Purpose |
| --- | --- |
| cmd/server | FTMind server |
| frontend | Vue web workspace |
| docreader | Python document parser |
| plugins/baoan-policy-collector | Bao'an crawler, raw packages and RSS |
| config/wiki_schemas | Versioned Wiki schemas |
| internal/application/service | Feedback, Wiki, knowledge and sharing services |
| client, cli, mcp-server | API client, CLI and MCP |
| deploy | Cloud Compose assets |
| docs | Architecture, operations, API and product docs |

## Further reading

- [Chinese README](./README_CN.md)
- [Architecture](./docs/ARCHITECTURE.md)
- [Cloud deployment](./docs/CLOUD_DEMO_DEPLOYMENT_18310.md)
- [Bao'an collector](./plugins/baoan-policy-collector/README.md)
- [Policy Wiki schema](./config/wiki_schemas/policy-qa.v1.schema.json)
- [API reference](./docs/api/README.md)
- [CLI](./cli/README.md)
- [MCP server](./mcp-server/README.md)

## Security

Use strong, unique secrets for JWT, database, Redis, object storage and model providers. Bind internal services to loopback or the private Docker network, use HTTPS and minimum-privilege accounts, and review [the deployment runbook](./docs/DEPLOYMENT_RUNBOOK.md).
