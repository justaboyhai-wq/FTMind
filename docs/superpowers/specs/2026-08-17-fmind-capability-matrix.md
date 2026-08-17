# FMind 与 PRD MVP 能力矩阵

状态含义：

- `reuse`：FMind 已有实现，先做契约和回归验收；
- `adapt`：已有实现，但需增加统一身份、外部 Agent 或来源标识；
- `new`：融合项目新增；
- `defer`：明确不进入本轮默认交付。

## 1. 97 项功能覆盖

| 能力域 | PRD 功能编号 | 判定 | 当前承载/计划任务 |
|---|---|---|---|
| ORG | FR-ORG-001–005, FR-ORG-008 | reuse/adapt | FMind Tenant、Organization、成员、角色、上下文；Task 0、Task 1、Task 2 |
| ORG | FR-ORG-006 | adapt | FMind 服务身份/API Key；增加 Connector Secret 与短期 Binding Token；Task 2、Task 3 |
| ORG | FR-ORG-007 | new | AgentBinding 与 User-Agent-Project 委托；Task 1、Task 2 |
| AGT | FR-AGT-001 | reuse | `internal/application/service/agent_service.go` 与 Agent API；Task 0 回归 |
| AGT | FR-AGT-002 | adapt | AgentBinding 接入配置和 Connector 安装参数；Task 2、Task 14 |
| AGT | FR-AGT-003 | adapt | Memory/Knowledge/Wiki/Raw/Skill 能力授权；Binding capability scopes；Task 1、Task 2、Task 13 |
| AGT | FR-AGT-004 | reuse/adapt | FMind MCP 服务、工具注册和审批；Cognition MCP 增加外部记忆工具；Task 13 |
| AGT | FR-AGT-005 | adapt | 既有模型凭据加 Connector Secret 托管、轮换和撤销；Task 2、Task 3 |
| AGT | FR-AGT-006 | adapt | Agent Context Policy 增加 Project/Task/Memory/Wiki 策略；Task 1、Task 13 |
| AGT | FR-AGT-007 | reuse/adapt | 既有 Agent Session 加绑定 session 映射；Task 1、Task 4 |
| AGT | FR-AGT-008 | new | 连接器和代理链路测试；Task 2、Task 5、Task 6 |
| AGT | FR-AGT-009–010 | reuse/adapt | MCP 调用审计、Agent 停用和恢复；Task 2、Task 13、Task 16 |
| MEM | FR-MEM-001–002 | new/adapt | MemoryCore 自动 L0 捕获和管理查看；Task 4、Task 5、Task 8、Task 14 |
| MEM | FR-MEM-003 | adapt | MemoryCore L1 提取；FMind 增加纠正/确认/撤回；Task 8 |
| MEM | FR-MEM-004 | adapt | MemoryCore L2 聚合；FMind 增加范围、证据和结果确认；Task 8 |
| MEM | FR-MEM-005 | reuse/adapt | MemoryCore L3 沉淀；FMind 接收成熟事件；Task 7、Task 10 |
| MEM | FR-MEM-006 | new | L3 审核、版本、复审和失效；Task 11 |
| MEM | FR-MEM-007–008 | adapt | 记忆搜索和上下文召回；MemoryProxy/Plugin/Provider 原生召回；Task 5、Task 6、Task 13 |
| MEM | FR-MEM-009–011 | new/adapt | 冲突、纠正、撤回、脱敏、保留期和删除传播；Task 8 |
| MEM | FR-MEM-012 | new | L3 → 记忆 Wiki，声明级证据和血缘；Task 11、Task 12 |
| RAW | FR-RAW-001–012 | reuse/adapt | FMind Knowledge/Storage/Source/ACL/Lineage；本轮记忆 Wiki 不进入 Raw；Task 0 矩阵和现有回归 |
| PAR | FR-PAR-001–008 | reuse/adapt | Docreader、解析、OCR、质量和人工队列；不由记忆 Wiki 触发；Task 0 回归 |
| RAG | FR-RAG-001–009 | reuse/adapt | Retriever、Embedding、Rerank、ACL、索引和引用；记忆 Wiki 不触发普通 RAG；Task 0 回归 |
| WIKI | FR-WIKI-001–006 | reuse/adapt | Wiki 页面、修订、链接、审核、撤回；记忆来源标识和 L3 投影；Task 11、Task 14 |
| WIKI | FR-WIKI-007–009 | adapt | 证据绑定、页面反馈、记忆 Wiki 只读/废弃；Task 11、Task 12 |
| SKL | FR-SKL-001–008 | reuse/defer | FMind Skill 读取/执行基础保留；完整 Skill 资产、提炼和跨 Agent 分发另立计划 |
| GOV | FR-GOV-001–011 | reuse/adapt | Tenant/RBAC/Audit/Policy 基础复用；Binding、Project、Asset Scope、Token 和证据新接入；Task 0–3、Task 8、Task 13 |
| OPS | FR-OPS-001–010 | reuse/adapt | Runtime task、Asynq、健康检查、审计、指标和恢复；L3 Outbox、Proxy/MemoryCore 观测新增；Task 10、Task 12、Task 15、Task 16 |

## 2. PRD 业务输入与输出覆盖

| 类型 | PRD 编号 | 本轮验证 |
|---|---|---|
| 输入 | IN-01 组织/项目/用户/Agent | Binding Context、Project/Task 字段；Task 1–3 |
| 输入 | IN-02 文件上传 | FMind 现有上传回归，不与记忆 Wiki 混用；Task 0 |
| 输入 | IN-03 外部数据源 | FMind 现有 Connector 回归；Task 0 |
| 输入 | IN-04 URL/结构化数据 | FMind 现有导入回归；Task 0 |
| 输入 | IN-05 Agent 交付物 | FMind 现有 Raw/知识入口回归；Task 0 |
| 输入 | IN-06 对话/任务轨迹 | Plugin、Provider、MemoryProxy 自动捕获；Task 4–6 |
| 输出 | OUT-01 Memory Context | MemoryProxy/Plugin/Provider + Cognition MCP；Task 5、6、13 |
| 输出 | OUT-02 RAG 问答 | FMind 既有知识库回归；不与 MemoryCore 召回合并；Task 0 |
| 输出 | OUT-03 Wiki 导航 | FMind Wiki 回归和记忆 Wiki 标识；Task 11、14 |
| 输出 | OUT-04 Raw 原件访问 | FMind 既有原件权限回归；Task 0 |
| 输出 | OUT-05 Skill 方法复用 | 本轮只保持既有 Skill 基础；完整融合另立计划 |
| 输出 | OUT-06 Agent Context Package | Memory/RAG/Wiki/Raw/Skill 分区装配；Task 13 |

## 3. MVP 红线

- 跨租户、部门、Workspace、Project、Team、User、Agent、Task 越权为 0；
- Connector Secret 不出现在 Prompt、工具参数和普通日志；
- L0 捕获成功率目标不低于 99%；
- Context Package P95 目标不高于 4 秒；
- L3 正式 Wiki 声明级证据覆盖率为 100%；
- L3 未审核不得成为记忆 Wiki 当前版本；
- Memory Wiki 不创建 Raw、Document、Chunk、Embedding 或普通 RAG 索引；
- MemoryCore 的 L0/L1 向量召回和 L2/L3 Profile write-through 必须保留，
  但与 FMind RAG collection、生命周期、权限和索引完全隔离；
- FMind 原有记忆开关和 Neo4j 路径回归不受影响；
- L3 事件必须通过 durable outbox，不能因进程崩溃丢失。
