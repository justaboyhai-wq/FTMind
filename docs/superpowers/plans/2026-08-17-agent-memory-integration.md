# External Agent Access and Memory Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an FMind-governed external Agent access center in which plugins, providers, and MemoryProxy automatically capture conversations and inject MemoryCore recall, while reviewed L3 memories publish only into team memory Wikis.

**Architecture:** FMind is the control plane and sole authority for organization, users, Agents, bindings, keys, policies, review, and Wiki publication. MemoryProxy plus OpenClaw/Hermes connectors form the data plane for automatic capture and recall injection; MemoryCore remains the L0-L3 processing and recall plane. External Agents never invoke L1-L3 extraction directly: connectors capture complete turns into L0 and MemoryCore's existing pipeline performs extraction asynchronously.

**Tech Stack:** Go 1.26, Gin, GORM, PostgreSQL/SQLite, Redis/Asynq, Vue 3, TypeScript 6, TDesign, Node.js 22, Hono/MemoryProxy, MemoryCore Gateway, Vitest, Docker Compose.

---

## Delivery slices

1. Organization-scoped AgentBinding and hashed Binding Key control plane.
2. Signed Binding Context introspection, caching, revocation, and audit.
3. MemoryProxy OpenAI/Anthropic automatic capture and recall with FMind identity.
4. OpenClaw Plugin and Hermes Provider binding support.
5. MemoryCore source-built deployment and FMind model capability gateway.
6. L3 lifecycle events, review, memory Wiki publication, and recovery.
7. FMind frontend access center, memory administration, and end-to-end rollout.

FMind's existing `enable_memory` preference, Neo4j memory service, and knowledge-base Agent memory path are outside the change set.

## Task 0: Freeze the PRD delta and existing-capability matrix

**Files:**
- Create: `docs/superpowers/specs/2026-08-17-prd-mvp-alignment.md`
- Create: `docs/superpowers/specs/2026-08-17-fmind-capability-matrix.md`

- [ ] **Step 1: Record approved PRD overrides**

State unambiguously: only reviewed L3 publishes to a memory Wiki; L2 remains in MemoryCore and is not published; memory Wiki does not create Raw/RAG assets; FMind internal memory remains unchanged.

- [ ] **Step 2: Map every PRD P0 function**

For each `FR-ORG-*`, `FR-AGT-*`, `FR-MEM-*`, `FR-RAW-*`, `FR-PAR-*`, `FR-RAG-*`, `FR-WIKI-*`, `FR-SKL-*`, `FR-GOV-*`, and `FR-OPS-*`, record `existing/reuse`, `adapt`, `new`, or `deferred`, exact FMind file/service, test evidence, and delivery task.

- [ ] **Step 3: Freeze MVP exclusions**

Move Skill execution to its own follow-on plan unless product review restores it to this memory-integration release. Keep Cognition MCP contracts in this plan because they are required for Agent consumption.

- [ ] **Step 4: Review and commit**

Verify no PRD P0 row lacks an owner or explicit deferral.

```bash
git add docs/superpowers/specs/2026-08-17-prd-mvp-alignment.md docs/superpowers/specs/2026-08-17-fmind-capability-matrix.md
git commit -m "docs: align memory integration with PRD MVP"
```

## Task 1: Persist organization-scoped Agent bindings

**Files:**
- Create: `internal/types/agent_binding.go`
- Create: `internal/types/interfaces/agent_binding.go`
- Create: `internal/application/repository/agent_binding.go`
- Create: `internal/application/repository/agent_binding_test.go`
- Modify: `internal/database/migration.go`

- [ ] **Step 1: Write failing repository tests**

Test tenant/team/department isolation, unique active binding names, key-prefix lookup, key rotation, revoked-key rejection, session mapping, and client inability to override organization scope.

```go
func TestAgentBindingRepositoryResolveKeyIsTenantScoped(t *testing.T) {
    repo := newAgentBindingRepoForTest(t)
    binding := seedBinding(t, repo, 7, "department-a", "team-a", "agent-a")
    key := issueTestBindingKey(t, repo, binding.ID)
    resolved, err := repo.ResolveActiveKey(context.Background(), key.Prefix)
    require.NoError(t, err)
    assert.EqualValues(t, 7, resolved.TenantID)
    assert.Equal(t, "team-a", resolved.TeamID)
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/application/repository -run AgentBinding -count=1`

Expected: compilation failure because binding types and repository do not exist.

- [ ] **Step 3: Implement entities**

Define `AgentBinding`, `AgentBindingKey`, `AgentBindingSession`, and `AgentBindingAudit`. Required fields include tenant, department, workspace, project, team, user, Agent, task, connector type, capture/recall/L3-Wiki flags, capability scopes, asset scopes, status, policy version, key hash/prefix/expiry/revocation, external session, last seen, and timestamps.

```go
type AgentBinding struct {
    ID             string `gorm:"primaryKey;size:36"`
    TenantID       uint64 `gorm:"not null;index:idx_binding_scope,priority:1"`
    DepartmentID   string `gorm:"size:64;index:idx_binding_scope,priority:2"`
    TeamID         string `gorm:"size:64;not null;index:idx_binding_scope,priority:3"`
    WorkspaceID    string `gorm:"size:64;index"`
    ProjectID      string `gorm:"size:64;index"`
    UserID         string `gorm:"size:64;not null"`
    AgentID        string `gorm:"size:64;not null"`
    ConnectorType  string `gorm:"size:32;not null"`
    CaptureEnabled bool   `gorm:"not null"`
    RecallEnabled  bool   `gorm:"not null"`
    L3WikiEnabled  bool   `gorm:"not null"`
    PolicyVersion  uint64 `gorm:"not null;default:1"`
    Status         string `gorm:"size:24;not null"`
}
```

Hash keys with SHA-256 over a cryptographically random 32-byte value plus a server-side pepper. Store only prefix and hash; show plaintext once.

- [ ] **Step 4: Implement repository and migration**

All reads require tenant scope except constant-time key resolution. Key rotation creates a replacement and revokes the previous key in one transaction. Session mapping uses `binding_id + external_session_id` uniqueness.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/application/repository -run AgentBinding -count=1`

Expected: PASS.

```bash
git add internal/types/agent_binding.go internal/types/interfaces/agent_binding.go internal/application/repository/agent_binding.go internal/application/repository/agent_binding_test.go internal/database/migration.go
git commit -m "feat(bindings): persist external agent bindings"
```

## Task 2: Build Binding Key management and organization authorization

**Files:**
- Create: `internal/application/service/agentbinding/service.go`
- Create: `internal/application/service/agentbinding/service_test.go`
- Create: `internal/handler/agent_binding.go`
- Create: `internal/handler/agent_binding_test.go`
- Modify: `internal/container/container.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Write failing service and handler tests**

Cover create/list/get/update, organization membership validation, connector allowlist, one-time plaintext key response, rotate, revoke, disable, connection-test request, and audit history.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/application/service/agentbinding ./internal/handler -run AgentBinding -count=1`

Expected: compilation failure.

- [ ] **Step 3: Implement management service**

Never accept `tenant_id` from JSON. Resolve it from auth context and validate department/team/user/Agent references through existing FMind organization services. Supported connectors are constants: `openclaw_plugin`, `hermes_provider`, `openai_proxy`, `anthropic_proxy`, `generic_sdk`.

- [ ] **Step 4: Add management routes**

```text
POST   /api/v1/agent-bindings
GET    /api/v1/agent-bindings
GET    /api/v1/agent-bindings/:id
PATCH  /api/v1/agent-bindings/:id
POST   /api/v1/agent-bindings/:id/keys/rotate
POST   /api/v1/agent-bindings/:id/revoke
POST   /api/v1/agent-bindings/:id/test
GET    /api/v1/agent-bindings/:id/audit
```

Return generated plugin/provider configuration or proxy Base URL according to connector type.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/application/service/agentbinding ./internal/handler -run AgentBinding -count=1`

Expected: PASS.

```bash
git add internal/application/service/agentbinding internal/handler/agent_binding.go internal/handler/agent_binding_test.go internal/container/container.go internal/router/router.go
git commit -m "feat(bindings): manage external agent access"
```

## Task 3: Add signed Binding Context introspection

**Files:**
- Create: `internal/middleware/service_auth.go`
- Create: `internal/middleware/service_auth_test.go`
- Create: `internal/handler/binding_introspection.go`
- Create: `internal/handler/binding_introspection_test.go`
- Modify: `internal/config/config.go`
- Modify: `.env.example`

- [ ] **Step 1: Write failing security tests**

Test valid key, wrong key, expired/revoked key, disabled binding, timestamp replay, body tampering, policy-version increment, and immediate revocation response.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/middleware ./internal/handler -run 'ServiceAuth|BindingIntrospection' -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement service HMAC**

Canonical service signature:

```text
<unix timestamp>\n<nonce>\n<method>\n<path>\n<sha256 body>
```

Use `X-FMind-Timestamp`, `X-FMind-Nonce`, and `X-FMind-Signature`, a ±300 second window, constant-time comparison, Redis nonce storage, and bounded Lite fallback.

- [ ] **Step 4: Implement introspection**

```text
POST /internal/v1/agent-bindings/introspect
```

Input contains the Connector Secret. Output is a five-minute Binding Token plus signed Context containing binding, tenant, department, workspace, project, team, user, Agent, task, roles, capability/asset scopes, capture/recall/L3 policy, policy version, token ID, and expiry. Do not echo the secret. The Connector Secret never enters Prompt or tool context.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/middleware ./internal/handler -run 'ServiceAuth|BindingIntrospection' -count=1`

Expected: PASS.

```bash
git add internal/middleware/service_auth.go internal/middleware/service_auth_test.go internal/handler/binding_introspection.go internal/handler/binding_introspection_test.go internal/config/config.go .env.example
git commit -m "feat(bindings): issue signed binding contexts"
```

## Task 4: Move MemoryProxy identity authority to FMind

**Files:**
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/fmind/binding-client.ts`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/fmind/binding-cache.ts`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/fmind/binding-client.test.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/db/binding-repo.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/identity.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/handler.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/anthropicHandler.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/config.ts`

- [ ] **Step 1: Write failing binding tests**

Verify first-request introspection, cache hit, TTL expiry, policy-version rejection, revoked key, FMind outage behavior, and that request-body/header team/user/Agent values cannot override signed Context.

- [ ] **Step 2: Verify failure**

Run from `MemoryProxy`: `npm test -- --run src/fmind`

Expected: FAIL because FMind binding client does not exist.

- [ ] **Step 3: Implement client and bounded cache**

Read Connector Secret from the proxy credential position, exchange it for a short-lived Binding Token, verify Context signature and expiry, and cache no longer than the token lifetime. Never log plaintext secret, token, or full Context.

- [ ] **Step 4: Replace identity resolution**

Make FMind Context authoritative for space/team/user/Agent. Retain existing Redis `BindingRepo` only as a disposable session hot cache keyed by `binding_id + external_session_id`; remove its role as long-term organization authority.

- [ ] **Step 5: Verify and commit in Memory repository**

Run: `npm test -- --run src/fmind src/db src/identity`

Expected: PASS.

```bash
git add MemoryProxy/src/fmind MemoryProxy/src/db/binding-repo.ts MemoryProxy/src/identity.ts MemoryProxy/src/handler.ts MemoryProxy/src/anthropicHandler.ts MemoryProxy/src/config.ts
git commit -m "feat(proxy): use FMind agent bindings"
```

## Task 5: Preserve automatic proxy capture and recall

**Files:**
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/tdai/recorder.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/tdai/pending-writes.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/handler.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/anthropicHandler.ts`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/src/fmind/automatic-memory.test.ts`

- [ ] **Step 1: Write protocol-flow tests**

For streaming and non-streaming OpenAI and Anthropic requests verify: recall occurs before upstream prompt construction; injected tags are removed before capture; complete user/assistant/tool content is assembled; L0 write occurs after completion; duplicate stream finalization writes once; capture/recall flags work independently.

- [ ] **Step 2: Verify failure against the new Binding Context path**

Run: `npm test -- --run src/fmind/automatic-memory.test.ts`

Expected: FAIL until handlers consume Binding Context policies.

- [ ] **Step 3: Implement policy-controlled automatic flow**

Keep `recordTdaiTurn`, `trackWrite`, `withL0Retry`, streaming parsers, and recall injection. Replace old identity inputs with signed Context. Do not add an external L1/L2/L3 extraction endpoint.

- [ ] **Step 4: Verify and commit**

Run: `npm test -- --run src/fmind/automatic-memory.test.ts src/tdai`

Expected: PASS.

```bash
git add MemoryProxy/src/tdai MemoryProxy/src/handler.ts MemoryProxy/src/anthropicHandler.ts MemoryProxy/src/fmind/automatic-memory.test.ts
git commit -m "feat(proxy): capture and recall by binding policy"
```

## Task 6: Bind OpenClaw Plugin and Hermes Provider

**Files:**
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/openclaw-plugin/src/hooks/capture.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/openclaw-plugin/src/hooks/recall.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/openclaw-plugin/index.ts`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/openclaw-plugin/README_CN.md`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/hermes-plugin/memory/memory_tencentdb/__init__.py`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/openclaw-plugin/src/fmind-binding.test.ts`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/hermes-plugin/tests/test_fmind_binding.py`

- [ ] **Step 1: Write failing connector tests**

OpenClaw: `before_prompt_build` recalls, `agent_end` captures, `before_message_write` removes injected tags, and Binding Context determines identity. Hermes: `prefetch` recalls, `sync` captures asynchronously, flush triggers session completion, and revoked binding disables both.

- [ ] **Step 2: Implement Binding Key configuration**

Replace separately configured team/user/Agent authority with `FMIND_BINDING_KEY` plus FMind introspection URL. Keep framework-native session IDs.

- [ ] **Step 3: Preserve original extraction semantics**

Connectors only capture L0 and recall memory. They must not run local L1/L2/L3 extraction or decide extraction timing.

- [ ] **Step 4: Verify and commit**

Run OpenClaw tests and build; run `pytest MemoryCore/hermes-plugin/tests/test_fmind_binding.py` in its configured Python environment.

Expected: PASS.

```bash
git add MemoryCore/openclaw-plugin MemoryCore/hermes-plugin
git commit -m "feat(connectors): bind OpenClaw and Hermes through FMind"
```

## Task 7: Add FMind model capability gateway and wire MemoryCore

**Files:**
- Create: `internal/application/service/modelcap/service.go`
- Create: `internal/application/service/modelcap/service_test.go`
- Create: `internal/handler/model_capability.go`
- Create: `internal/handler/model_capability_test.go`
- Modify: `internal/container/container.go`
- Modify: `internal/router/router.go`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/src/gateway/config.ts`

- [ ] **Step 1: Write failing routing tests**

Test `memory_extract` Chat selection, `memory_embedding` selection, tenant context, budgets/timeouts, signed service auth, and redacted provider errors.

- [ ] **Step 2: Implement internal OpenAI-compatible endpoints**

```text
POST /internal/model-capabilities/v1/chat/completions
POST /internal/model-capabilities/v1/embeddings
```

Reuse FMind `ModelService`; do not create another provider registry.

- [ ] **Step 3: Point MemoryCore at FMind**

Use existing OpenAI-compatible LLM and embedding configuration. Preserve prompts, thresholds, capture/extraction/pipeline settings, and recall ranking.

- [ ] **Step 4: Verify and commit in each repository**

Run: `go test ./internal/application/service/modelcap ./internal/handler -run ModelCapability -count=1`

Run from MemoryCore: `npm test -- --run src/gateway && npm run build:plugin`

Expected: PASS.

## Task 8: Implement L0-L2 governance and privacy lifecycle

**Files:**
- Create: `internal/application/service/external_memory/governance.go`
- Create: `internal/application/service/external_memory/governance_test.go`
- Create: `internal/handler/external_memory_governance.go`
- Create: `internal/handler/external_memory_governance_test.go`
- Modify: `internal/types/external_memory.go`
- Modify: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/src/gateway/v2-router.ts`

- [ ] **Step 1: Write lifecycle tests**

Cover pre-capture redaction, L0 visibility, retention expiry, legal deletion, L1 correct/confirm/revoke, L2 confirm scope/evidence/result, conflict/supersede/expire, and propagation to recall plus derived candidates.

- [ ] **Step 2: Implement governance APIs**

Add scoped endpoints for L1 correction and withdrawal, L2 confirmation and rejection, conflict resolution, retention policy, and deletion request. Every mutation records actor, reason, before/after version, source evidence impact, and trace ID.

- [ ] **Step 3: Preserve MemoryCore algorithms**

Use existing MemoryCore mutation mechanisms where available and add adapters where absent; do not reimplement extraction. A deleted/revoked source must stop future recall and mark dependent L3/Wiki content for review.

- [ ] **Step 4: Verify and commit**

Run focused Go and MemoryCore tests. Expected: PASS.

## Task 9: Persist L3 review and publication state

**Files:**
- Create: `internal/types/external_memory.go`
- Create: `internal/types/interfaces/external_memory.go`
- Create: `internal/application/repository/external_memory.go`
- Create: `internal/application/repository/external_memory_test.go`
- Modify: `internal/database/migration.go`

- [ ] **Step 1: Write idempotency/isolation tests**

Insert duplicate matured events and assert one integration event, snapshot, review task, and later publication. Verify equal `memory_id` values in different tenant/team/binding scopes remain isolated.

- [ ] **Step 2: Implement entities and repository**

Add `MemoryL3Snapshot`, `MemoryReviewTask`, `MemoryReviewHistory`, `MemoryWikiPublication`, and `MemoryIntegrationEvent`. Unique projection key is `tenant_id + team_id + binding_id + memory_id + memory_version`.

- [ ] **Step 3: Verify and commit**

Run: `go test ./internal/application/repository -run ExternalMemory -count=1`

Expected: PASS.

## Task 10: Emit and receive signed L3 lifecycle events

**Files:**
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/src/integrations/fmind/l3-events.ts`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/src/integrations/fmind/l3-events.test.ts`
- Create: `internal/handler/internal_memory.go`
- Create: `internal/handler/internal_memory_test.go`
- Modify: committed L3 create/update/conflict/revoke call sites in MemoryCore.

- [ ] **Step 1: Write cross-language signature fixtures and lifecycle tests**

Cover matured, updated, conflicted, revoked, duplicate delivery, transient retry, permanent failure, checksum, size limits, and binding/organization scope.

- [ ] **Step 2: Implement MemoryCore sender**

Write a durable outbox record in the same committed unit as native L3. An independent worker delivers it to FMind; callback failure is observable/retryable and never rolls back L3.

- [ ] **Step 3: Implement FMind intake**

Add `POST /internal/v1/memory/events`; validate service signature, schema, Binding Context scope, checksum, Markdown ≤1 MiB, and evidence ≤256 KiB; persist before returning 202.

- [ ] **Step 4: Verify and commit**

Run focused Go and Vitest suites. Expected: PASS.

## Task 11: Implement L3 review and memory Wiki publication

**Files:**
- Create: `internal/application/service/external_memory/review.go`
- Create: `internal/application/service/external_memory/review_test.go`
- Create: `internal/application/service/external_memory/wiki_publisher.go`
- Create: `internal/application/service/external_memory/wiki_publisher_test.go`
- Modify: `internal/types/knowledgebase.go`
- Modify: `internal/types/wiki_page.go`
- Create: `internal/handler/external_memory.go`
- Create: `internal/handler/external_memory_test.go`

- [ ] **Step 1: Write state and Wiki invariant tests**

Test no publication before approval, approve/reject/request-changes, one memory Wiki per team, one page per memory, revision per version, duplicate checksum no-op, revoke to deprecated, claim-level evidence coverage of 100%, and zero Raw/document/chunk creation.

- [ ] **Step 2: Implement review API**

```text
GET  /api/v1/external-memory/l3/reviews
GET  /api/v1/external-memory/l3/reviews/:id
POST /api/v1/external-memory/l3/reviews/:id/approve
POST /api/v1/external-memory/l3/reviews/:id/reject
POST /api/v1/external-memory/l3/reviews/:id/request-changes
```

- [ ] **Step 3: Implement memory Wiki publisher**

Create `<team> · 记忆知识库` with `type=wiki`, `wiki_source=memory`; render `fmind.cognition/v1` Markdown; store source memory/binding/review/revision provenance plus paragraph/claim-to-evidence locators. Reject formal publication when any factual claim lacks valid evidence. Never call Docreader, chunking, embedding, or document ingestion.

- [ ] **Step 4: Verify and commit**

Run focused service/handler tests. Expected: PASS.

## Task 12: Add durable jobs and recovery

**Files:**
- Create: `internal/task/external_memory_publication.go`
- Create: `internal/task/external_memory_publication_test.go`
- Create: `internal/container/recover_pending_memory_tasks.go`
- Create: `internal/container/recover_pending_memory_tasks_test.go`

- [ ] **Step 1: Write recovery tests**

Cover restart from approved/rendering/publishing/callback-failed, no duplicate Wiki revision, dead letter after retry limit, and audited manual retry.

- [ ] **Step 2: Implement PostgreSQL-as-source-of-truth jobs**

Redis/Asynq only schedules. Every worker reloads state, performs one idempotent transition, and persists before scheduling the next step.

- [ ] **Step 3: Verify and commit**

Run: `go test ./internal/task ./internal/container -run ExternalMemory -count=1`

Expected: PASS.

## Task 13: Add Cognition MCP and Context Package contract

**Files:**
- Create: `internal/mcpserver/cognition/server.go`
- Create: `internal/mcpserver/cognition/server_test.go`
- Create: `internal/types/context_package.go`
- Modify: `internal/container/container.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Write MCP contract tests**

Cover `memory_get_context`, `memory_search`, `memory_capture_turn`, `memory_confirm_candidate`, `knowledge_search`, `wiki_get_page`, `document_read`, and `context_assemble`; verify final-user identity, project/task scope, Agent capabilities, asset policy, risk flags, trace IDs, and denial behavior.

- [ ] **Step 2: Implement atomic tools**

Reuse existing FMind knowledge/document services and MemoryCore adapter. `memory_capture_turn` is an explicit connector/repair tool, not an L1-L3 extraction trigger.

- [ ] **Step 3: Implement Context Package**

Return separate Memory/RAG/Wiki/Raw/Skill sections with per-source Token budgets, provenance, conflicts, partial-failure warnings, permission decisions, and used asset/version IDs. Do not merge storage or search engines.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/mcpserver/cognition -count=1`

Expected: PASS.

## Task 14: Build the FMind Agent access and memory UI

**Files:**
- Create: `frontend/src/api/agent-binding/index.ts`
- Create: `frontend/src/api/external-memory/index.ts`
- Create: `frontend/src/views/external-memory/AgentBindingList.vue`
- Create: `frontend/src/views/external-memory/AgentBindingEditor.vue`
- Create: `frontend/src/views/external-memory/BindingKeyDialog.vue`
- Create: `frontend/src/views/external-memory/MemoryList.vue`
- Create: `frontend/src/views/external-memory/L3ReviewList.vue`
- Create: `frontend/src/views/external-memory/L3ReviewDetail.vue`
- Create: `frontend/src/views/external-memory/PublicationList.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/stores/menu.ts`
- Modify: `frontend/src/components/menu.vue`
- Modify: `frontend/src/views/knowledge/components/KbWikiBadge.vue`
- Modify: `frontend/src/views/knowledge/wiki/WikiBrowser.vue`

- [ ] **Step 1: Write API/state tests**

Test one-time key display, connector configuration rendering, rotate/revoke, status filters, optimistic review version, memory-Wiki badge, and source-memory navigation.

- [ ] **Step 2: Implement “外部 Agent 接入”**

Provide connector choice, organization/department/team/user/Agent binding, capture/recall/L3-Wiki policies, generated install/proxy instructions, connection test, key rotation/revocation, and audit.

- [ ] **Step 3: Implement external memory and review pages**

Support L0-L3 browse, L1 correction, L2 confirmation, conflict/supersede/expiry handling, evidence permissions, review actions, publication status/retry, privacy/deletion status, and bidirectional L3/Wiki navigation. Keep existing General Settings memory toggle unchanged.

- [ ] **Step 4: Verify and commit**

Run from frontend: `npm test && npm run type-check && npm run build`

Expected: PASS.

## Task 15: Source-build and deploy MemoryProxy plus MemoryCore

**Files:**
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryCore/Dockerfile`
- Create: `E:/worktest/TencentDB-Agent-Memory/MemoryProxy/Dockerfile`
- Create: `docker/memory-core.config.yaml`
- Create: `docker/memory-proxy.config.yaml`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`
- Modify: `.env.example`

- [ ] **Step 1: Add reproducible Node 22 multi-stage builds**

Install from lockfiles, compile from source, copy runtime-only artifacts, run non-root, and add health checks.

- [ ] **Step 2: Configure network exposure**

MemoryCore uses internal `expose` only. MemoryProxy exposes only its authenticated OpenAI/Anthropic data endpoint; its management and health internals stay private. Production fails closed when Binding or service secrets are empty.

- [ ] **Step 3: Validate and build**

Run: `docker compose --profile memory config`

Run: `docker compose --profile memory build fmind-memory-proxy fmind-memory-core fmind-server fmind-frontend`

Expected: valid configuration and successful local-source builds without downloaded FMind/Memory images.

- [ ] **Step 4: Commit each repository separately**

Do not include unrelated dirty-worktree files in either commit.

## Task 16: End-to-end acceptance, metrics, and rollout

**Files:**
- Create: `tests/integration/external_agent_memory_flow_test.go`
- Create: `tests/integration/fixtures/memorycore_l3_matured.json`
- Create: `docs/EXTERNAL_AGENT_MEMORY.md`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: Test each connector path**

For OpenClaw, Hermes, OpenAI Proxy, and Anthropic Proxy: create binding, use key, recall before answer, capture complete turn after answer, verify L0, wait for automatic L1-L3 extraction, and confirm no Agent called an extraction API.

- [ ] **Step 2: Test organization and key security**

Attempt forged tenant/team/user/Agent fields, cross-team recall, expired key, rotated key, revoked key, stale Proxy cache, disabled capture, disabled recall, and disabled L3 Wiki policy.

Also test workspace/project/task scope, capability/asset intersections, Connector Secret exclusion from Prompt/logs, short-lived token expiry, L0 redaction/retention/deletion, L1 correction, L2 confirmation, and derived-memory invalidation.

- [ ] **Step 3: Test L3 knowledge projection**

Deliver matured L3, verify no Wiki before review, approve, verify memory Wiki revision and provenance, verify no Raw/document/chunk records, resend event, and verify idempotency.

- [ ] **Step 4: Run regression gates**

Run FMind `go test ./...`, frontend `npm test && npm run type-check && npm run build`, MemoryCore `npm test && npm run build:plugin`, MemoryProxy `npm test`, and Compose smoke tests. Assert L0 capture success ≥99% in the soak fixture, Context Package P95 ≤4s, authorization violations = 0, claim evidence coverage = 100%, and end-to-end trace identifiers on every sampled flow.

Expected: PASS; FMind existing memory behavior remains unchanged.

- [ ] **Step 5: Roll out by checkpoint**

Deploy disabled schema/routes; enable Binding control plane; enable Proxy for one test binding; enable OpenClaw/Hermes; enable L3 intake without publishing; enable review/Wiki for one team; then expand tenants. Rollback disables integration while preserving L0-L3, review history, and published Wiki revisions.

- [ ] **Step 6: Commit acceptance tests and operations documentation**

```bash
git add tests/integration/external_agent_memory_flow_test.go tests/integration/fixtures/memorycore_l3_matured.json docs/EXTERNAL_AGENT_MEMORY.md docs/ARCHITECTURE.md
git commit -m "test: verify bound external agent memory flow"
```
