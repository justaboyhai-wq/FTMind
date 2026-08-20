# FTMind 外部 Agent 记忆融合：PRD MVP 对齐说明

- 基线：`E:\worktest\时空世界整体架构\release\时空世界-产品说明及PRD详细设计书-V1.0.html`
- 对齐版本：2026-08-17
- 适用范围：FTMind 外部 Agent 记忆融合子项目

## 1. 已确认的产品决策

### 1.1 L3-only 知识化

原 PRD 中“L2/L3 → Raw → 知识库”的描述被本项目最新决策覆盖：

```text
L0–L3 原生记忆链路
    ├─ L0/L1/L2/L3：继续由 MemoryCore 存储、治理和召回
    └─ 仅审核通过的 L3：进入 FTMind 记忆 Wiki
```

L2 不直接发布为记忆 Wiki，也不触发普通 Raw/RAG、Docreader、Chunk、Embedding 或索引任务。
Memory 动态数据整体不进入 FTMind RAG 体系；L0/L1 及 L2/L3 profile 只保留在
MemoryCore 自己的存储与召回向量库中，并与 FTMind RAG collection 完全隔离。

普通 Raw 资产、文件知识库和 RAG 仍按 FTMind 现有能力及 PRD 独立运行；后续如需要将 L3 作为正式原件或 RAG 数据源，另立发布策略，不属于本轮默认路径。

### 1.2 两套记忆不合并

- FTMind 现有 Episode/Entity/Neo4j 记忆服务于 FTMind 内部知识库 Agent，保持不变；
- MemoryCore 通过 Plugin、Provider 或 MemoryProxy 服务外部 Agent；
- 外部 Agent 不主动调用 L1/L2/L3 提取接口；
- 连接器捕获完整对话写入 L0，MemoryCore 原有 Pipeline 异步完成 L1–L3。

### 1.3 组织身份统一

绑定身份由 FTMind 组织体系解析，包含：

```text
Tenant → Department → Workspace/Project → Team
       → User → Agent → Task → Session → Binding
```

外部请求体中的租户、部门、团队、用户和 Agent 字段不具备权威性。Connector Secret 仅用于交换短期 Binding Token；Token 绑定主体、动作、项目范围和过期时间，不进入 Agent Prompt 或工具上下文。

## 2. PRD 功能分层

### 本轮新增或适配

- `FR-ORG-006/007`：服务身份、用户-Agent 委托和绑定 Key；
- `FR-AGT-002/003/005/006/008`：接入配置、能力授权、凭据管理、上下文策略和连接测试；
- `FR-MEM-001/003/004/005/006/007/008/009/010/011/012`：外部 L0–L3、治理、召回、审核和 L3 记忆 Wiki；
- `FR-GOV-*` 中与作用域、审计、令牌和证据有关的能力；
- `FR-WIKI-*` 中与记忆来源、审核、版本和证据追溯有关的能力；
- PRD 首批 Cognition MCP 的 Memory、Wiki、Document 和 Context Package 契约。

### 直接复用并验收

- FTMind Agent 注册、MCP 服务、工具审批、Skill 读取/执行基础；
- FTMind Knowledge Base、Retriever、Embedding、Rerank、Parser、Docreader；
- FTMind Wiki 页面、修订、审计和恢复任务；
- FTMind Tenant、成员、角色、API Key、审计和组织上下文；
- FTMind 现有模型服务和 Asynq/Redis 任务治理。

### 明确不在本轮默认实现

- 正式 Ontology、双时间事实、GIS 和复杂规则引擎；
- 将 L2 直接发布为 Wiki 或 Raw；
- 改造 FTMind 内部记忆；
- MemoryKnowledge Wiki 和 MemoryPanel 的独立产品入口；
- Skill 资产完整生命周期和跨 Agent 方法分发。Skill 现有读取/执行能力保留；若纳入本轮，必须另开子计划并增加独立验收。

## 3. 端到端验收闭环

最小真实闭环为：

```text
一个租户
→ 一个部门/团队
→ 两个用户
→ 一个外部 Agent
→ 一个 Connector Binding
→ 自动捕获 L0
→ MemoryCore 自动 L1/L2/L3
→ L1/L2 治理
→ L3 人工审核
→ 团队记忆 Wiki
→ 外部 Agent 记忆召回
```

必须验证：跨用户/项目权限、Binding Token 过期与撤销、捕获失败重试、L3 事件不丢失、Wiki 声明级证据、FTMind 原有记忆不受影响。

## 4. 与原 PRD 的变更记录

| 原 PRD 约定 | 本轮基线 | 原因 |
|---|---|---|
| L2/L3 发布 Raw | 仅审核 L3 发布记忆 Wiki | 保持记忆召回与知识问答两条独立路径 |
| Agent 直接使用长期 API Key | Connector Secret 换短期 Binding Token | 满足组织绑定、撤销和最小权限 |
| Memory 通过 MCP/API/Connector 均可录入 | 自动捕获优先，MCP capture 仅补录/测试 | 不让外部 Agent 负责记忆提取时机 |
| MemoryPanel/MemoryKnowledge 独立管理 | FTMind 统一管理 | 避免第二套组织和 Wiki 权限 |
| Team/Project/Agent 作用域 | Tenant/Department/Workspace/Project/Team/User/Agent/Task | 与 FTMind 组织权限和绑定中心对齐 |
