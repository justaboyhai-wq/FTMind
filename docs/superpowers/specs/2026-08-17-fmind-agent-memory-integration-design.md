# FMind 与 TencentDB Agent Memory 融合设计

- 状态：已确认，按 1.2 版规划实施
- 版本：1.2
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
5. 面向 OpenClaw、Hermes、Claude Code、OpenAI-compatible、Anthropic-compatible 及其他第三方 Agent 的统一绑定与自动接入能力。

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

审核通过的 L3 仅存入 Wiki 类型知识库，并要求目标 KnowledgeBase 的
`WikiConfig.IsMemoryWiki=true`，标识为“记忆知识库”。它不进入普通 Raw/RAG
文件知识库，不触发 Docreader、普通文件切片、Embedding 或 RAG 索引任务。
MemoryCore 自己的 L0/L1 向量/FTS/hybrid recall 以及 L2/L3 Profile write-through
仍然保留；这套 Memory VectorDB 不得与 FMind RAG collection 混用。

标准 Markdown 是 Wiki 页面的内容格式和导出格式，不代表 Raw 文件资产。

本设计以 L3 为唯一知识化入口。L2 仍按 MemoryCore 原机制生成、确认、更新和召回，但不直接发布为记忆 Wiki。该决策覆盖原 PRD 中“L2/L3 发布 Raw”的旧描述。

### 2.4 两阶段演进

第一阶段保留 TypeScript MemoryCore，通过内部服务接口接入 FMind，优先验证 L0–L3 行为和产品闭环。

第二阶段在接口契约和质量基线稳定后，将 MemoryCore 机制逐步迁移为 FMind Go 原生记忆引擎。迁移不得改变外部 Agent 接口、审核流程或 Wiki 发布契约。

### 2.5 控制面、代理数据面和记忆面的三层分工

FMind 是控制面，是组织、用户、Agent、绑定、Binding Key、权限和策略的唯一权威来源。

MemoryProxy 是外部 Agent 数据面，保留 OpenAI/Anthropic 兼容协议代理、流式转发、自动对话捕获和回答前记忆注入能力。MemoryProxy 不再拥有独立的组织和长期绑定权威，只缓存 FMind 签发的短期 Binding Context。

MemoryCore 是记忆面，负责保存 L0、自动执行 L1–L3 提取、维护记忆生命周期并提供原生召回。

外部 Agent 不主动调用 L1/L2/L3 提取接口。Plugin、MemoryProvider 或 MemoryProxy 自动捕获完整对话后写入 L0，由 MemoryCore Pipeline 根据原有轮次、时间、空闲和会话策略异步提取。

## 3. 范围

### 3.1 第一阶段包含

- MemoryCore 作为 FMind 内部受管服务运行；
- FMind 外部 Agent 接入中心、组织绑定和 Binding Key；
- OpenClaw Plugin、Hermes MemoryProvider 接入；
- MemoryProxy OpenAI/Anthropic 兼容协议代理、自动捕获和召回注入；
- 外部 Agent 对话自动写入和 L0–L3 原生召回；
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
- 用通用对话 API 代替 Plugin、Provider 或 MemoryProxy 的自动捕获；
- 由外部 Agent 主动触发 L1/L2/L3 提取；
- MemoryProxy 自己维护第二套组织、用户、权限或长期 Binding 权威。

## 4. 总体架构

```text
FMind Control Plane
    ├─ 组织 / 用户 / Agent
    ├─ AgentBinding / Binding Key
    ├─ 接入策略 / 审计 / 状态
    └─ L3 审核 / 记忆 Wiki
            │ Binding Context / 策略同步
            ▼
外部 Agent接入数据面
    ├─ OpenClaw Plugin（Hook）
    ├─ Hermes MemoryProvider
    ├─ Claude Code Hook 或 Anthropic Proxy
    └─ MemoryProxy
       ├─ OpenAI-compatible
       └─ Anthropic-compatible
            │ 自动捕获 L0 / 原生记忆召回
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

FMind 是唯一管理入口和控制面。MemoryProxy 是外部 Agent 可访问的数据入口；MemoryCore 仅开放内部网络服务，不直接向公网暴露管理接口。

## 5. 组件职责

### 5.1 FMind

FMind 负责：

- 外部 Agent 接入鉴权和身份上下文；
- Agent 类型、实例、组织归属、接入策略和 Binding Key 生命周期；
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

### 5.3 MemoryProxy 与框架连接器

MemoryProxy 负责：

- OpenAI-compatible 与 Anthropic-compatible 请求和流式响应代理；
- 从模型协议中恢复用户消息、Assistant 内容、工具调用和会话标识；
- 使用 Binding Key 解析 FMind Binding Context；
- 回答前从 MemoryCore 召回并注入相关记忆；
- 回答完成后异步写入完整 L0 Turn；
- 跟踪在途写入、优雅停机、有限重试和断路保护；
- 缓存短期 Binding Context，但不成为身份权威。

OpenClaw Plugin 和 Hermes MemoryProvider 负责在框架生命周期中完成相同的捕获与召回：回答前召回注入，回答后捕获完整 Turn。它们不执行 L1–L3 提取。

### 5.4 不纳入最终架构的组件

- MemoryKnowledge Wiki：由 FMind Wiki 完整替代；
- MemoryPanel：管理能力整合进 FMind 前端；
- MemoryCore 独立模型供应商配置界面：由 FMind 模型管理替代。
- MemoryProxy 原有 BindingRepo 作为长期身份权威：迁移为 FMind AgentBinding，Proxy 只保留有 TTL 的运行缓存。

## 6. 身份与作用域

所有外部记忆必须通过 FMind AgentBinding 获得稳定作用域：

```text
FMind tenant_id ↔ MemoryCore team_id
FMind team_id   ↔ 记忆 Wiki 归属
FMind user_id   ↔ MemoryCore user_id
FMind agent_id  ↔ MemoryCore agent_id
workspace_id    ↔ 工作空间
project_id      ↔ 项目范围
task_id         ↔ 当前任务
session_id      ↔ 外部 Agent 会话
binding_id      ↔ 一次可审计的外部 Agent 绑定
```

关联必须使用稳定内部 ID。邮箱、名称和显示标签不得作为关联主键。

所有查询、事件、审核任务和 Wiki 投影至少包含 `tenant_id`、`team_id` 与 `binding_id`。捕获和召回上下文同时保留 `department_id`、`workspace_id`、`project_id` 和 `task_id`。权限计算遵循“用户权限 ∩ 项目范围 ∩ Agent 能力 ∩ 资产策略”。

## 7. 外部 Agent 接入中心与自动提取

### 7.1 AgentBinding

每个绑定关联 `tenant_id`、`department_id`、`team_id`、`user_id`、`agent_id`、连接器类型和策略。客户端提交的同名身份字段均不可信，运行时身份只能由 Binding Key 解析。

Binding Key 是安装阶段的 Connector Secret，只在创建时显示一次，数据库只保存哈希。它只能保存在插件、Provider、代理或受控密钥存储中，不得进入 Agent Prompt、工具上下文或业务日志。运行时 Connector Secret 先换取短期 Binding Token，再访问 MemoryProxy/MemoryCore。Secret 支持到期、轮换、撤销、来源限制、最后使用时间和审计。停用绑定立即停止新的捕获与召回，但不删除历史记忆。

### 7.2 接入类型

- `openclaw_plugin`：`before_prompt_build` 召回注入，`agent_end` 捕获 Turn，`before_message_write` 清除注入标签；
- `hermes_provider`：`prefetch` 同步召回，`sync` 异步捕获，session flush 触发阶段处理；
- `openai_proxy`：MemoryProxy 透明代理 OpenAI-compatible 协议；
- `anthropic_proxy`：MemoryProxy 透明代理 Anthropic-compatible 协议，Claude Code 优先使用该方式或经验证的 Hook；
- `generic_sdk`：仅供无法安装 Plugin 且无法走协议代理的框架、历史补录和测试，不作为主要接入方式。

### 7.3 绑定解析

连接器携带 Connector Secret。MemoryProxy/Plugin 首次使用时调用 FMind 内部 token exchange/introspection，将 Secret 换取最长五分钟的短期 Binding Token 与签名 Binding Context：

```json
{
  "binding_id": "binding_uuid",
  "tenant_id": 1,
  "department_id": "department_uuid",
  "team_id": "team_uuid",
  "workspace_id": "workspace_uuid",
  "project_id": "project_uuid",
  "task_id": "task_uuid",
  "user_id": "user_uuid",
  "agent_id": "agent_uuid",
  "role_ids": ["role_uuid"],
  "capability_scopes": ["memory:recall", "memory:capture"],
  "asset_scopes": ["team:team_uuid"],
  "capture_enabled": true,
  "recall_enabled": true,
  "l3_wiki_enabled": true,
  "policy_version": 4,
  "expires_at": "2026-08-17T10:05:00Z"
}
```

MemoryProxy 可在 Token 有效期内缓存 Context。FMind 通过短 TTL、策略版本和主动撤销事件使缓存失效。高风险操作必须实时 introspection。外部请求体中的组织和权限字段一律忽略。

### 7.4 记忆治理

MemoryCore 保持原有提取策略，FMind 增加治理入口：L1 可由授权用户纠正、确认或撤回；L2 可确认适用范围、结果与证据；L3 由团队管理员或审核员审核。冲突记忆并列显示，不静默覆盖；替代、失效、撤销和删除均保留审计与证据影响记录。

L0 捕获前执行敏感信息规则；L0 支持用户可见、按保留期清理和合法删除。删除或撤回事件必须传播到 MemoryCore 召回状态及其派生候选，已发布记忆 Wiki 转入复核，不直接物理删除。

### 7.5 自动捕获和提取

```text
Agent 完成一轮对话
→ Plugin/Provider Hook 或 MemoryProxy 拼装完整 Turn
→ 按 Binding Context 写入 MemoryCore L0
→ MemoryCore notifyPipeline
→ Buffer / Scanner / Worker
→ 原有策略自动提取 L1
→ 聚合 L2
→ 演化 L3
```

Agent 不调用提取 API。通用 Conversation API 仅作为连接器内部传输、补录和测试协议。

### 7.6 自动召回

```text
Agent 构造提示词或发起模型请求
→ Plugin/Provider/MemoryProxy 读取 Binding Context
→ 直接调用 MemoryCore 原生召回
→ 格式化并注入 Prompt
→ 过滤注入标签，避免再次被捕获
```

FMind 业务服务不进入每次模型推理热路径。FMind 只处理绑定控制、策略同步、审计和管理查询。

### 7.7 管理、MCP 与兼容接口

FMind 提供 AgentBinding 创建、轮换、停用、连接测试、状态和审计接口，并提供 L0–L3 管理与治理。Conversation 写入接口保留给连接器、SDK、历史导入和自动化测试，不能暴露“提取 L1/L2/L3”操作。

首批 Cognition MCP 固定为 `memory_get_context`、`memory_search`、`memory_capture_turn`、`memory_confirm_candidate`、`knowledge_search`、`wiki_get_page`、`document_read` 和 `context_assemble`。自动捕获仍走 Hook/Provider/Proxy；MCP 的 capture 仅用于显式工具接入和补录。所有 MCP 调用透传最终用户、项目、Agent、任务和 trace ID。

`context_assemble` 只定义统一 Context Package 契约，不合并记忆与 Wiki 的存储或检索路径。返回内容保留来源类型、权限决策、冲突标识、Token 配额和所使用的 Memory/Wiki/Chunk ID。

连接器内部对话事件的最小字段：

```json
{
  "event_id": "evt_uuid",
  "binding_id": "binding_uuid",
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

`event_id` 是写入幂等键。相同事件重复提交必须返回原处理结果，不得生成重复 L0。身份字段必须由 Binding Context 填充，不能信任 Agent 请求体。

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

### 11.2 AgentBinding 与 AgentBindingKey

`AgentBinding` 记录组织、部门、Workspace、Project、用户、Agent、连接器、捕获/召回/L3 Wiki 策略、能力范围、资产范围、状态与 policy version。

`AgentBindingKey` 记录 key prefix、密钥哈希、到期时间、轮换关系、撤销时间、最后使用时间和来源限制。明文 Key 不落库。

### 11.3 BindingToken 与 ContextPackage

`BindingToken` 是短期签名凭证，绑定主体、动作范围、项目、Agent、policy version 和过期时间。服务端只保存撤销与审计所需的 token ID，不保存可重放明文。

`ContextPackage` 是运行时组合合同，分别装载 Memory、RAG、Wiki、Raw Citation 和 Skill 结果，并记录每类 Token 预算、来源、权限决定、冲突与降级信息。

### 11.4 AgentBindingSession

替代 MemoryProxy 原长期 BindingRepo，记录 `binding_id + external_session_id` 到内部 user/team/agent/task 的稳定映射。MemoryProxy Redis 只保存可丢失缓存。

### 11.5 MemoryL3Snapshot

保存收到的 L3 内容快照，确保审核和发布不依赖 MemoryCore 临时在线状态。

关键字段：`memory_id`、`memory_version`、`tenant_id`、`team_id`、`agent_id`、`title`、`summary`、`content_markdown`、`confidence`、`sensitivity`、`evidence_refs`、`content_checksum`、`source_event_id`。

### 11.6 MemoryReviewTask

保存审核状态、审核内容快照、审核人、意见和状态变更历史。

### 11.7 MemoryWikiPublication

记录记忆版本与 Wiki 资产的血缘：

```text
memory_id + memory_version
        ↕
wiki_id + wiki_page_id + wiki_revision_id
```

同时保存发布状态、checksum、发布时间、失败阶段和最后一次错误。

### 11.8 WikiClaimEvidence

保存 Wiki 修订中声明或段落到 L3/L2/L1/L0 证据的定位关系。正式记忆 Wiki 发布要求所有事实性声明均绑定至少一个有效证据引用。

### 11.9 MemoryIntegrationEvent 与 Outbox

保存入站事件、幂等状态、处理次数和最终结果，用于审计、防重放和故障恢复。MemoryCore 的 L3 生命周期事件必须先写 durable outbox，再由独立 worker 投递 FMind，避免 L3 提交成功但事件丢失。

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
├─ Agent 接入中心
│  ├─ Agent 绑定
│  ├─ Binding Key
│  ├─ 连接测试
│  └─ 捕获/召回状态
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
- L1 纠正、确认和撤回；
- L2 适用范围、结果和证据确认；
- 冲突、替代、失效和删除影响查看；
- 查看任务失败原因并执行有权限的人工重试；
- 从记忆跳转 Wiki，从 Wiki 追溯来源记忆。

记忆 Wiki 页面展示 `[记忆知识库] [L3] [已审核] [版本 N]`。FMind 现有“开启记忆功能”开关的名称、作用和行为保持不变。

## 14. 部署与安全

第一阶段由 FMind Compose 统一编排：

```text
fmind-frontend
fmind-server
fmind-docreader
fmind-memory-proxy
fmind-memory-core
postgres
redis
```

要求：

- MemoryCore 只使用 Docker 内部网络，不映射公网端口；
- MemoryProxy 仅暴露受 Binding Key 保护的模型协议代理入口，管理接口只在内部网络；
- 内部接口使用独立服务凭据，不使用用户 API Key；
- 生产环境缺少服务密钥时启动失败；
- 模型密钥只保存在 FMind 模型管理侧；
- 日志不得记录完整对话、模型密钥或原始敏感证据；
- 所有审核、发布、撤销和重试操作写入审计日志；
- Connector Secret 不进入 Agent Prompt；运行请求仅使用短期 Binding Token；
- L0 捕获执行脱敏、保留期和合法删除策略；
- 事件体、Markdown 和证据摘要设置大小上限；
- Binding Key 实施哈希存储、轮换、撤销、来源限制和租户级限流；
- 外部 Agent 捕获实施请求大小限制、会话并发限制和幂等保护。

## 15. 查询路径

第一阶段保持两条入口和两套语义：

### 15.1 外部记忆召回

Plugin、Provider 或 MemoryProxy 根据 Binding Context 直接调用 MemoryCore 原生 L0–L3 召回，并在外部 Agent 回答前注入。FMind 不进入推理热路径。

### 15.2 Wiki 问答

用户选择普通 Wiki 或记忆 Wiki，通过 FMind Wiki 问答能力查询审核后的稳定知识。

第一阶段不自动合并两个查询结果。后续如需统一 Agent 上下文编排，必须另行设计来源权重、权限、引用和 Token 配额。

## 16. 测试策略

### 16.1 契约测试

- 对话事件和 L3 事件 Schema 兼容性；
- FMind Adapter 与 MemoryCore API 的请求和错误映射；
- Binding Key introspection、Context 签名、缓存和撤销协议；
- OpenClaw、Hermes、OpenAI Proxy 与 Anthropic Proxy 的连接器契约；
- Model Capability Gateway 的认证、超时和预算；
- 第二阶段 Go 引擎必须通过相同契约测试。

### 16.2 功能测试

- 外部对话形成 L0–L3；
- 外部 Agent 无需调用提取接口即可自动捕获并触发 Pipeline；
- OpenClaw/Hermes Hook 和 OpenAI/Anthropic Proxy 均能捕获完整 Turn；
- 回答前召回注入，回答后捕获，注入内容不会重复进入 L0；
- 原生记忆召回不依赖 Wiki；
- 未审核 L3 不出现在 Wiki；
- 审核通过后创建记忆 Wiki 页面；
- L3 新版本创建待审核修订；
- 驳回、要求修改、冲突、撤销和废弃流程；
- 普通 Wiki 和普通 RAG 知识库不受影响；
- FMind 原有记忆开关行为不变。

### 16.3 隔离与安全测试

- 不同租户和团队的数据不可越权读取；
- 伪造请求体身份不能覆盖 Binding Context；
- Key 哈希、轮换、撤销和缓存失效行为正确；
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

### 16.6 指标与性能门槛

- L0 捕获成功率不低于 99%；
- L1/L2 人工采纳率具备可统计事件，试点目标不低于 70%；
- Context Package P95 不高于 4 秒；
- 权限越权测试结果为 0；
- 正式记忆 Wiki 声明级证据绑定率为 100%；
- 每次调用贯穿 trace_id、request_id、job_id、principal 和 memory/wiki version。

## 17. 第一阶段验收标准

满足以下条件视为第一阶段融合完成：

1. 外部 Agent 可通过 OpenClaw Plugin、Hermes Provider、OpenAI Proxy 或 Anthropic Proxy 完成绑定；
2. 连接器在回答前自动召回、回答后自动捕获，Agent 不调用记忆提取接口；
3. MemoryCore 完成 L0–L3 全链路，FMind 不重写其提取算法；
4. FMind 是组织、用户、Agent、Binding Key 和策略的唯一权威；
5. MemoryProxy 不再保存独立长期组织绑定，只缓存短期 Binding Context；
6. MemoryCore 所需 Chat 和 Embedding 能力由 FMind 统一提供；
7. 成熟 L3 必须经过人工审核，未审核内容无法发布；
8. 审核通过的 L3 进入团队独立记忆 Wiki，不进入普通 Raw/RAG 知识库；
9. L3 和 Wiki Revision 可双向追溯，更新、冲突和撤销均保留历史；
10. 外部记忆召回与 Wiki 问答独立运行；
11. FMind 原有记忆功能及其开关行为不变；
12. 重复事件不会产生重复 L0、审核任务或 Wiki 页面；
13. 服务重启和常见依赖故障不会丢失已确认状态；
14. 租户、部门、团队、用户、Agent 和 binding 作用域贯穿捕获、召回、审核和 Wiki；
15. MemoryCore 不直接暴露公网端口，模型密钥不在两套系统重复保存。

## 18. 第二阶段 Go 原生化原则

第二阶段不是重新设计产品，而是替换记忆引擎实现：

- 冻结 Binding Context、连接器协议、MemoryCore API、事件 Schema、审核接口和 Wiki 发布契约；
- 为 L0–L3 建立可重复的行为与质量基线；
- TypeScript 与 Go 引擎影子双跑，对比提取、召回和冲突处理结果；
- 按 L0 写入、L1 提取、L2 聚合、L3 沉淀、召回逐步切换；
- 每一步保留按租户或 Agent 回退到 TypeScript 引擎的能力；
- 完成数据校验和稳定期后再停用 TypeScript MemoryCore。

Go 化可以复用 FMind 的 PostgreSQL、Redis/Asynq、模型服务、审计和可观测性，但不得把外部记忆与 FMind 原有记忆表或召回逻辑合并。

## 19. 后续实施计划边界

本设计获批后，实施计划需要进一步细化为：

1. 数据库迁移和仓储层；
2. FMind AgentBinding、Binding Key 与组织身份；
3. introspection、Binding Context 签名、缓存和撤销；
4. MemoryProxy 身份权威迁移及 OpenAI/Anthropic 自动捕获；
5. OpenClaw Plugin 与 Hermes Provider 绑定接入；
6. Memory Engine Adapter 与管理/兼容 API；
7. Model Capability Gateway；
8. MemoryCore 配置和 L3 事件改造；
9. L3 审核服务和异步任务；
10. 记忆 Wiki 类型、发布器和版本血缘；
11. 前端 Agent 接入中心、外部记忆、审核和发布页面；
12. Docker Compose、健康检查和运维配置；
13. 契约、功能、安全、恢复和回归测试；
14. 灰度启用、观测、回滚和第一阶段验收。

实施计划必须按可独立验证的纵向切片安排，不能同时大规模改造 FMind 和 MemoryCore，也不能在第一阶段提前实施 Go 原生重写。
