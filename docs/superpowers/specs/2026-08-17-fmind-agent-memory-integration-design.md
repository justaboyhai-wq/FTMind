# FMind 与 TencentDB Agent Memory 融合设计

- 状态：待用户评审
- 版本：1.0
- 日期：2026-08-17
- 适用阶段：第一阶段服务融合；兼顾第二阶段 Go 原生化演进

## 1. 背景与目标

FMind 已具备知识库、Wiki、模型管理、文档处理、检索问答和知识库 Agent 能力。TencentDB Agent Memory（下文简称 MemoryCore）具备面向外部 Agent 的 L0–L3 长期记忆提取、演化和召回机制。

本次融合的目标不是合并两套记忆，也不是用 MemoryCore 替换 FMind 内部记忆，而是让 FMind 增加一套面向外部 Agent 的长期记忆能力，并将审核通过的成熟 L3 认知发布为可维护、可问答的 FMind Wiki。

最终产品同时具备：

1. FMind 原有知识库与知识库 Agent 能力；
2. MemoryCore 面向外部 Agent 的完整 L0–L3 记忆能力；
3. 成熟 L3 经审核后形成团队“记忆 Wiki”的知识沉淀能力；
4. 统一的模型能力、身份范围、管理入口和运行治理能力。

## 2. 已确认的核心决策

### 2.1 两套记忆保持独立

FMind 原有记忆服务于 FMind 内部知识库 Agent，继续使用现有开关、Episode、Entity、Relationship 和 Neo4j 机制，本次不改造。

MemoryCore 服务于外部 Agent，对外部对话执行 L0–L3 提取、演化、存储和原生记忆召回。

两套记忆不互相写入、不互相转换、不共享召回结果，也不共用启停开关。

### 2.2 双路径、单向知识投影

MemoryCore 中沉淀完成的 L0–L3 始终保留在原记忆链路中，并继续使用 MemoryCore 原生召回机制。

审核通过的 L3 额外生成 FMind Wiki 投影。该投影是知识副本，不迁移、不删除、不替代原 L3。

数据流严格单向：

```text
MemoryCore L3
    ├─ 原生记忆召回 → 外部 Agent
    └─ 人工审核通过 → 记忆 Wiki → Wiki 问答
```

Wiki 人工修订不得反向覆盖 MemoryCore 的 L0–L3 数据。

### 2.3 记忆 Wiki 不是 RAG 知识库

审核通过的 L3 仅存入 Wiki 类型知识库，并标识为“记忆知识库”。它不进入普通 Raw/RAG 文件知识库，不触发 Docreader、普通文件切片、Embedding 或 RAG 索引任务。

标准 Markdown 是 Wiki 页面的内容格式和导出格式，不代表 Raw 文件资产。

### 2.4 两阶段演进

第一阶段保留 TypeScript MemoryCore，通过内部服务接口接入 FMind，优先验证 L0–L3 行为和产品闭环。

第二阶段在接口契约和质量基线稳定后，将 MemoryCore 机制逐步迁移为 FMind Go 原生记忆引擎。迁移不得改变外部 Agent 接口、审核流程或 Wiki 发布契约。

## 3. 范围

### 3.1 第一阶段包含

- MemoryCore 作为 FMind 内部受管服务运行；
- 外部 Agent 对话写入和 L0–L3 原生召回；
- FMind 与 MemoryCore 的稳定适配接口；
- FMind 向 MemoryCore 提供统一 Chat 和 Embedding 模型能力；
- 成熟 L3 提交人工审核；
- 审核通过后发布到团队记忆 Wiki；
- L3、审核任务、Wiki 页面及修订版本之间的血缘追踪；
- 失败重试、幂等、审计和运行状态管理；
- FMind 前端中的外部记忆管理、审核和发布记录页面；
- Docker Compose 内部网络部署和服务认证。

### 3.2 第一阶段不包含

- 改造或替换 FMind 原有记忆功能；
- 将 FMind Episode/Entity 迁移为 MemoryCore L0–L3；
- 将外部 Agent 记忆注入 FMind 内部 Agent；
- 将 L3 发布到普通 Raw/RAG 知识库；
- 合并记忆召回和 Wiki 问答路径；
- 第一阶段重写 MemoryCore 的 L0–L3 算法；
- 第一阶段完整实现租户、部门和跨部门权限重构；
- MemoryKnowledge Wiki、MemoryPanel 和 MemoryProxy 的长期保留。

## 4. 总体架构

```text
外部 Agent
    │
    │ 对话写入 / 记忆召回
    ▼
FMind 外部记忆 API
    │
    ▼
Memory Engine Adapter
    │
    ▼
MemoryCore（第一阶段 TypeScript）
    ├─ L0 → L1 → L2 → L3
    ├─ 原生记忆存储与召回
    ├─ 调用 FMind Model Capability Gateway
    └─ 发出 L3 生命周期事件
                    │
                    ▼
             FMind L3 审核中心
                    │ 人工审核通过
                    ▼
             Memory Wiki Publisher
                    │
                    ▼
             团队记忆 Wiki / Wiki 问答

FMind 内部知识库 Agent
    └─ FMind 原有记忆模块（保持不变）
```

FMind 是唯一产品入口和控制面。MemoryCore 仅开放内部网络服务，不直接向公网暴露管理接口。

## 5. 组件职责

### 5.1 FMind

FMind 负责：

- 外部 Agent 接入鉴权和身份上下文；
- 租户、团队和用户标识的权威来源；
- MemoryCore 调用编排和适配；
- 模型供应商、模型密钥、用量和调用审计；
- L3 审核任务和操作审计；
- 记忆 Wiki 创建、页面发布、修订、废弃和问答；
- 投影状态、失败恢复及运行监控；
- 统一前端管理入口。

### 5.2 MemoryCore

MemoryCore 负责：

- 外部 Agent 对话捕获；
- L0 原始事件；
- L1 原子记忆；
- L2 场景和阶段聚合；
- L3 稳定认知；
- 记忆去重、合并、冲突、证据链和生命周期；
- 定时、空闲、会话结束等提取触发；
- L0–L3 原生记忆召回；
- 向 FMind 发出 L3 成熟、更新、冲突和撤销事件。

### 5.3 不纳入最终架构的组件

- MemoryKnowledge Wiki：由 FMind Wiki 完整替代；
- MemoryPanel：管理能力整合进 FMind 前端；
- MemoryProxy：外部 Agent 接入和模型调用分别由 FMind API 与 Model Capability Gateway 承担；
- MemoryCore 独立模型供应商配置界面：由 FMind 模型管理替代。

## 6. 身份与作用域

第一阶段不实施完整组织权限重构，但所有新增接口和数据必须携带稳定作用域：

```text
FMind tenant_id ↔ MemoryCore team_id
FMind team_id   ↔ 记忆 Wiki 归属
FMind user_id   ↔ MemoryCore user_id
FMind agent_id  ↔ MemoryCore agent_id
session_id      ↔ 外部 Agent 会话
```

关联必须使用稳定内部 ID。邮箱、名称和显示标签不得作为关联主键。

所有查询、事件、审核任务和 Wiki 投影至少包含 `tenant_id` 与 `team_id`。未来部门权限落地时可扩展 `department_id`，但不得改变已有主键和事件版本。

## 7. 外部 Agent 记忆接口

FMind 对外提供稳定的记忆门面，隐藏 MemoryCore 的具体实现：

```text
POST /api/v1/agent-memory/conversations
POST /api/v1/agent-memory/search
GET  /api/v1/agent-memory/memories/{memory_id}
GET  /api/v1/agent-memory/memories
```

对话事件的最小字段：

```json
{
  "event_id": "evt_uuid",
  "tenant_id": "tenant_uuid",
  "team_id": "team_uuid",
  "agent_id": "agent_uuid",
  "user_id": "user_uuid",
  "session_id": "session_uuid",
  "message_id": "message_uuid",
  "role": "user",
  "content": "消息内容",
  "occurred_at": "2026-08-17T10:00:00Z",
  "metadata": {}
}
```

`event_id` 是写入幂等键。相同事件重复提交必须返回原处理结果，不得生成重复 L0。

FMind Go 后端内部依赖 `MemoryEngine` 抽象；第一阶段由 `TencentMemoryAdapter` 实现，第二阶段由 Go 原生引擎实现。

## 8. 统一模型能力

MemoryCore 不直接保存模型供应商密钥，改为调用 FMind 内部模型能力接口：

```text
POST /internal/model-capabilities/v1/chat/completions
POST /internal/model-capabilities/v1/embeddings
```

接口携带租户、调用主体、用途、追踪 ID、超时和预算。FMind 根据用途路由到具体模型并记录用量。

第一阶段不强制改变 MemoryCore 原生召回排序，也不强制接入 FMind Rerank。待建立召回质量基线后，再决定是否通过可选 Hook 统一 Rerank。

## 9. L3 成熟、审核和发布

### 9.1 成熟与发布分离

MemoryCore 判定 L3 已成熟后发送 `memory.l3.matured`。成熟只表示可以提交审核，不表示允许发布。

只有 FMind 人工审核通过，L3 才能成为记忆 Wiki 页面，并形成 `memory.l3.published` 结果。

### 9.2 事件类型

MemoryCore 向 FMind 发送：

- `memory.l3.matured`：新成熟版本待审核；
- `memory.l3.updated`：已存在 L3 产生新版本；
- `memory.l3.conflicted`：记忆出现冲突；
- `memory.l3.revoked`：记忆被撤销。

事件通过以下内部接口提交：

```text
POST /internal/v1/memory/events
```

接口使用服务身份认证、请求签名、时间戳、防重放窗口和事件幂等键。

### 9.3 L3 事件最小结构

```json
{
  "event_id": "evt_uuid",
  "event_type": "memory.l3.matured",
  "schema_version": "1.0",
  "occurred_at": "2026-08-17T10:00:00Z",
  "tenant_id": "tenant_uuid",
  "team_id": "team_uuid",
  "agent_id": "agent_uuid",
  "memory_id": "memory_uuid",
  "memory_version": 3,
  "title": "客户异常件处置经验",
  "summary": "经过多次任务验证形成的稳定处置流程",
  "content_markdown": "...",
  "confidence": 0.92,
  "sensitivity": "internal",
  "evidence_refs": [
    {"type": "memory_l1", "id": "memory_uuid"},
    {"type": "session", "id": "session_uuid"}
  ],
  "content_checksum": "sha256:...",
  "metadata": {}
}
```

`event_id` 用于消息投递幂等；`tenant_id + team_id + memory_id + memory_version` 用于投影任务唯一性；`content_checksum` 用于内容一致性检查。

### 9.4 审核状态

```text
draft
  → pending_review
      ├─ approved → publishing → published
      ├─ rejected
      └─ changes_requested → draft
```

审核记录保存审核人、审核时间、审核意见、L3 版本、审核内容快照、所属团队、发布目标和操作历史。

审核界面仅展示必要的证据摘要。查看原始对话证据需要独立的记忆查看权限。

## 10. 记忆 Wiki

### 10.1 知识类型

```text
FMind 知识
├─ RAG 知识库
│  ├─ 文件上传
│  ├─ 网页
│  └─ 其他数据源
└─ Wiki 知识库
   ├─ 普通 Wiki
   └─ 记忆知识库
      └─ 审核通过的 L3
```

记忆 Wiki 的固定标识：

```json
{
  "knowledge_type": "wiki",
  "wiki_source": "memory",
  "source_type": "memory_l3",
  "tenant_id": "tenant_uuid",
  "team_id": "team_uuid",
  "memory_id": "memory_uuid",
  "memory_version": 3,
  "review_status": "approved"
}
```

该标识只能由发布管道写入，普通 Wiki 创建入口不得伪造记忆来源。

### 10.2 创建规则

每个团队首次审核通过 L3 时，自动创建一个 Wiki 类型的独立知识库：

```text
名称：<团队名称> · 记忆知识库
类型：wiki
来源：memory
归属：tenant_id + team_id
```

同一 `memory_id` 对应同一逻辑 Wiki 页面。新的 `memory_version` 创建新的 Wiki Revision，不创建同名重复页面。

### 10.3 Markdown 格式

Wiki 内部保存标准 Markdown 源文：

```markdown
---
schema: fmind.cognition/v1
title: 客户异常件处置经验
tenant_id: tenant_uuid
team_id: team_uuid
source_type: memory_l3
source_memory_id: memory_uuid
source_memory_version: 3
confidence: 0.92
sensitivity: internal
review_status: approved
reviewed_at: 2026-08-17T11:00:00Z
content_checksum: sha256:...
---

# 客户异常件处置经验

## 适用场景

...

## 稳定结论

...

## 执行方法

...

## 限制与例外

...

## 来源说明

该页面由审核通过的团队 L3 记忆生成，原始证据保留在记忆系统中。
```

### 10.4 更新规则

- L3 新版本创建待审核 Wiki 修订，审核前不影响当前已发布版本；
- checksum 未变化时跳过重复发布；
- L3 冲突时，新修订进入 `review_required`，已发布旧版保留并显示风险提示；
- L3 撤销时，页面转为 `deprecated`，不物理删除；
- 团队停用时，记忆 Wiki 转为只读并停止接收新投影；
- Wiki 人工编辑形成知识侧修订，不反写 L3；
- 新 L3 与人工修订冲突时创建待审核版本，不自动覆盖人工内容。

## 11. 持久化模型

第一阶段在 FMind PostgreSQL 中新增以下逻辑实体；具体表名可在实施计划中按现有命名规范确定。

### 11.1 MemoryEngineBinding

记录 FMind 租户、团队和 MemoryCore 作用域映射，以及服务状态和配置版本。

关键字段：`id`、`tenant_id`、`team_id`、`engine_type`、`external_team_id`、`status`、`config_version`、时间戳。

### 11.2 MemoryL3Snapshot

保存收到的 L3 内容快照，确保审核和发布不依赖 MemoryCore 临时在线状态。

关键字段：`memory_id`、`memory_version`、`tenant_id`、`team_id`、`agent_id`、`title`、`summary`、`content_markdown`、`confidence`、`sensitivity`、`evidence_refs`、`content_checksum`、`source_event_id`。

### 11.3 MemoryReviewTask

保存审核状态、审核内容快照、审核人、意见和状态变更历史。

### 11.4 MemoryWikiPublication

记录记忆版本与 Wiki 资产的血缘：

```text
memory_id + memory_version
        ↕
wiki_id + wiki_page_id + wiki_revision_id
```

同时保存发布状态、checksum、发布时间、失败阶段和最后一次错误。

### 11.5 MemoryIntegrationEvent

保存入站事件、幂等状态、处理次数和最终结果，用于审计、防重放和故障恢复。

## 12. 任务与异常恢复

投影任务状态机：

```text
received
  → validating
  → pending_review
      ├─ rejected
      ├─ changes_requested
      └─ approved
           → rendering_markdown
           → publishing_wiki
           → published
```

异常状态包括 `validation_failed`、`publish_failed`、`callback_failed`、`conflicted` 和 `revoked`。

PostgreSQL 保存事实状态，Redis/Asynq 只负责异步执行。处理规则：

- 重复事件返回已有结果；
- Markdown 生成失败从渲染阶段重试；
- Wiki 发布失败保留审核结果并从发布阶段重试；
- 发布成功但回调失败时不回滚 Wiki，只重试回调；
- 服务重启扫描未完成任务并重新入队；
- 超过重试上限进入失败队列并通知管理员；
- 人工重试记录操作者和原因；
- 所有状态转换使用乐观锁或事务条件更新，避免并发覆盖。

## 13. 前端设计

FMind 增加独立的“外部记忆”管理模块，避免与现有知识库 Agent 记忆开关混淆：

```text
外部记忆
├─ L0 原始记录
├─ L1 原子记忆
├─ L2 场景记忆
├─ L3 成熟记忆
├─ L3 审核
└─ Wiki 发布记录
```

第一阶段前端支持：

- 按租户、团队、Agent、用户、会话和层级浏览外部记忆；
- 查看 L3 内容、置信度、版本和证据摘要；
- 审核通过、驳回和要求修改；
- 选择或确认目标团队记忆 Wiki；
- 查看 L3 与 Wiki Revision 的版本关系；
- 查看任务失败原因并执行有权限的人工重试；
- 从记忆跳转 Wiki，从 Wiki 追溯来源记忆。

记忆 Wiki 页面展示 `[记忆知识库] [L3] [已审核] [版本 N]`。FMind 现有“开启记忆功能”开关的名称、作用和行为保持不变。

## 14. 部署与安全

第一阶段由 FMind Compose 统一编排：

```text
fmind-frontend
fmind-server
fmind-docreader
fmind-memory-core
postgres
redis
```

要求：

- MemoryCore 只使用 Docker 内部网络，不映射公网端口；
- 内部接口使用独立服务凭据，不使用用户 API Key；
- 生产环境缺少服务密钥时启动失败；
- 模型密钥只保存在 FMind 模型管理侧；
- 日志不得记录完整对话、模型密钥或原始敏感证据；
- 所有审核、发布、撤销和重试操作写入审计日志；
- 事件体、Markdown 和证据摘要设置大小上限；
- 外部 Agent 接口实施租户级限流、请求大小限制和幂等保护。

## 15. 查询路径

第一阶段保持两条入口和两套语义：

### 15.1 外部记忆召回

外部 Agent 调用 FMind 外部记忆 API，由 MemoryCore 使用其原生 L0–L3 召回机制返回记忆上下文。

### 15.2 Wiki 问答

用户选择普通 Wiki 或记忆 Wiki，通过 FMind Wiki 问答能力查询审核后的稳定知识。

第一阶段不自动合并两个查询结果。后续如需统一 Agent 上下文编排，必须另行设计来源权重、权限、引用和 Token 配额。

## 16. 测试策略

### 16.1 契约测试

- 对话事件和 L3 事件 Schema 兼容性；
- FMind Adapter 与 MemoryCore API 的请求和错误映射；
- Model Capability Gateway 的认证、超时和预算；
- 第二阶段 Go 引擎必须通过相同契约测试。

### 16.2 功能测试

- 外部对话形成 L0–L3；
- 原生记忆召回不依赖 Wiki；
- 未审核 L3 不出现在 Wiki；
- 审核通过后创建记忆 Wiki 页面；
- L3 新版本创建待审核修订；
- 驳回、要求修改、冲突、撤销和废弃流程；
- 普通 Wiki 和普通 RAG 知识库不受影响；
- FMind 原有记忆开关行为不变。

### 16.3 隔离与安全测试

- 不同租户和团队的数据不可越权读取；
- 服务签名、防重放和事件幂等；
- 普通用户不能伪造 `wiki_source=memory`；
- 原始证据查看权限与 Wiki 阅读权限分离；
- 日志和错误信息不泄露敏感内容。

### 16.4 故障恢复测试

- MemoryCore、FMind、Redis 或数据库短暂不可用；
- 重复事件和乱序事件；
- 发布成功但回调失败；
- 服务重启恢复未完成任务；
- 达到重试上限后的人工恢复；
- 并发审核和并发发布的互斥行为。

### 16.5 回归测试

- FMind 知识库导入、Wiki、问答和现有 Agent；
- FMind 原有 Neo4j 记忆；
- 登录、租户上下文和模型管理；
- Compose 启动、健康检查和升级回滚。

## 17. 第一阶段验收标准

满足以下条件视为第一阶段融合完成：

1. 外部 Agent 可通过 FMind 接口写入对话并调用 MemoryCore 原生召回；
2. MemoryCore 完成 L0–L3 全链路，FMind 不重写其提取算法；
3. MemoryCore 所需 Chat 和 Embedding 能力由 FMind 统一提供；
4. 成熟 L3 必须经过人工审核，未审核内容无法发布；
5. 审核通过的 L3 进入团队独立记忆 Wiki，不进入普通 Raw/RAG 知识库；
6. L3 和 Wiki Revision 可双向追溯，更新、冲突和撤销均保留历史；
7. 外部记忆召回与 Wiki 问答独立运行；
8. FMind 原有记忆功能及其开关行为不变；
9. 重复事件不会产生重复 L0、审核任务或 Wiki 页面；
10. 服务重启和常见依赖故障不会丢失已确认状态；
11. 租户和团队作用域贯穿接口、事件、任务和 Wiki；
12. MemoryCore 不直接暴露公网端口，模型密钥不在两套系统重复保存。

## 18. 第二阶段 Go 原生化原则

第二阶段不是重新设计产品，而是替换记忆引擎实现：

- 冻结外部 Agent API、事件 Schema、审核接口和 Wiki 发布契约；
- 为 L0–L3 建立可重复的行为与质量基线；
- TypeScript 与 Go 引擎影子双跑，对比提取、召回和冲突处理结果；
- 按 L0 写入、L1 提取、L2 聚合、L3 沉淀、召回逐步切换；
- 每一步保留按租户或 Agent 回退到 TypeScript 引擎的能力；
- 完成数据校验和稳定期后再停用 TypeScript MemoryCore。

Go 化可以复用 FMind 的 PostgreSQL、Redis/Asynq、模型服务、审计和可观测性，但不得把外部记忆与 FMind 原有记忆表或召回逻辑合并。

## 19. 后续实施计划边界

本设计获批后，实施计划需要进一步细化为：

1. 数据库迁移和仓储层；
2. 内部事件入口与服务认证；
3. Memory Engine Adapter 与外部 Agent API；
4. Model Capability Gateway；
5. MemoryCore 配置和接口改造；
6. L3 审核服务和异步任务；
7. 记忆 Wiki 类型、发布器和版本血缘；
8. 前端外部记忆、审核和发布页面；
9. Docker Compose、健康检查和运维配置；
10. 契约、功能、安全、恢复和回归测试；
11. 灰度启用、观测、回滚和第一阶段验收。

实施计划必须按可独立验证的纵向切片安排，不能同时大规模改造 FMind 和 MemoryCore，也不能在第一阶段提前实施 Go 原生重写。
