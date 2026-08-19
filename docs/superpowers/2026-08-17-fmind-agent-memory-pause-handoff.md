---
status: paused
branch: main
timestamp: 2026-08-17T21:08:39+08:00
files_modified:
  - .env.example
  - docker-compose.dev.yml
  - docker-compose.yml
  - docker/Dockerfile.app
  - docker/Dockerfile.docreader
  - docreader/parser/epub_parser.py
  - docreader/parser/pptx_media.py
  - docreader/proto/docreader.pb.go
  - docreader/proto/docreader_grpc.pb.go
  - docreader/tests/test_epub_parser.py
  - docreader/tests/test_ppt_convert.py
  - docreader/tests/test_archive_safety.py
  - docreader/utils/archive.py
  - frontend/.dockerignore
  - frontend/Dockerfile
  - frontend/index.html
  - frontend/src/assets/theme/fmind.css
  - frontend/src/components/ModelEditorDialog.vue
  - frontend/src/components/menu.vue
  - frontend/src/views/auth/Login.vue
  - frontend/public/sf-logo-alone.png
  - frontend/src/assets/img/brand/sf-logo-alone.png
  - miniprogram/pages/settings/settings.js
  - miniprogram/utils/config.js
  - tests/miniprogram/miniprogram.test.js
  - prototype/
---

# FMind-Agent Memory 融合暂停交接与执行提示词

## 1. 暂停声明

- 暂停时间：2026-08-17 21:08（Asia/Shanghai）。
- 当前目标仍是 `in-progress`，未标记完成或失败。
- 用户要求停止继续消耗实现 Token。此后 Codex 只做只读审计，剩余实现交给其他编程工具。
- 已终止正在运行的全仓 Go 编译；该命令是被主动终止，不代表测试失败。
- 已中断 OpenClaw/Hermes 直连绑定实现代理。它留下未提交的中间代码，必须在原地审查和续做，禁止直接丢弃。

## 2. 两个代码库

| 代码库 | 路径 | 分支 | 当前 HEAD |
|---|---|---|---|
| FMind | `E:\worktest\FMind` | `main` | `1b4e6f2d` |
| TencentDB Agent Memory | `E:\worktest\TencentDB-Agent-Memory` | `feat/server_team` | `b616404` |

两个目录都有重要状态。执行工具必须分别运行 `git status --short`，不得只处理 FMind。

## 3. 冻结的架构决定

1. FMind 是统一的知识库、租户、部门、团队、用户、权限与审核管理底座。
2. TencentDB Agent Memory 保留原有 L0-L3 提取、沉淀、召回和群体记忆机制，不改其核心评分/提取算法。
3. 记忆召回与知识库问答是两条独立路径：
   - L0-L3 原始记忆继续进入 MemoryCore 自己的记忆存储和向量召回空间。
   - 只有通过人工审核的 L3 标准 Markdown 才发布到 FMind 的专用 Memory Wiki。
4. Memory Wiki 不是 RAG 知识库。必须阻止普通文件、URL、Raw、Knowledge、Chunk、Embedding、Graph/Vector/Keyword ingest。
5. Memory Wiki 与普通 Wiki/RAG 知识库并存，不覆盖、不混用。
6. 同一种向量数据库技术可以复用，但记忆向量与知识库向量必须使用不同数据库、collection/class/index 或租户命名空间，绝不能处于同一个向量空间。Embedding/Rerank 模型可统一，索引与权限边界不可统一。
7. FMind 原有“给知识库 Agent 使用的内部记忆”保持不动；本次外部 Agent Memory 不复用或改造该功能。
8. 外部 Agent 不能自行声明 tenant/team/user/agent/task。FMind AgentBinding 是唯一身份与权限权威。
9. Connector Secret 只用于换取短期 Binding Token；不得进入模型提示词、日志、普通 session metadata 或上游厂商请求。
10. Binding Token 必须短期、可在线复验，并绑定 `external_agent`、connector type、tenant/team/user/agent/task、policy version、capability scopes 与 asset scopes。
11. L3 发布必须经过审核。Memory Wiki 按 tenant+team 隔离，审核和发布由管理员/审核员执行。
12. 业务镜像应从本地源码构建。Docker 仍会下载 Node/Go/Python 等基础镜像和依赖，这与“业务代码不下载预制镜像”不冲突。

## 4. 已完成并提交的工作

### 4.1 FMind

- `06ea23ee`：组织域 AgentBinding 持久化、原子密钥轮换、RBAC、严格 scope、JWT、日志脱敏。
- `17385779`：Binding Token 权威 verify endpoint。
- `a910cd28`：受 Binding Context 约束的 Cognition MCP。
- `aeea1f19`：应用源码构建使用可达 Go proxy。
- `0fdd35b8`：新增 `memory` Compose profile，MemoryCore/Proxy 从相邻源码构建。
- `94989a65`：把凭证绑定到 `external_agent` 路由，阻止跨 Agent 上游越权。
- `f71b21cb`：完整 L3 可信事件、审核、发布、不可变 Wiki revision、撤销、并发 fencing、团队隔离与零 RAG Memory Wiki。
- `98ed9478`：外部记忆管理前端页面、绑定管理、L3 审核/发布、Memory Wiki 标识。
- `1b4e6f2d`：前端与 Go API 契约对齐。
- `b135035a`：AgentBinding pepper/token signing secret 配置说明。

### 4.2 TencentDB Agent Memory

- `7342b47`、`483c2d`、`12c9a52`、`4a67cd6`：MemoryProxy 绑定权威、入口凭证、远端 verify、资产收窄、capture/recall gate、bridge capability、每请求复验与安全重试。
- `5a05eeb`：MemoryCore `/v3` 数据面在线验证 FMind Binding Token，并将权威上下文传入隔离与任务载荷。
- `915d6f8`：L3 成熟/更新/撤销事件进入持久 outbox，HMAC 投递 FMind，含重试、死信和指标。
- `20b5820`：`external_agent` 贯穿 MemoryCore/Proxy 路由和 authority schema。
- `b616404`：MemoryCore/Proxy 源码镜像、lockfile、`npm ci`、固定基础镜像、非 root、严格环境展开与管理路由鉴权。

## 5. 已得到的验证证据

### FMind 后端

以下 Linux/CGO 命令在 `f71b21cb` 后通过：

```text
go test ./internal/types
        ./internal/application/repository
        ./internal/application/service/memorywiki
        ./internal/application/service
        ./internal/database
        ./internal/handler
        ./internal/middleware
        ./internal/router
        ./internal/mcpserver/cognition
        ./internal/agent/tools -count=1
```

10 个包组全部 `ok`。Memory Wiki 独立最终审计结论为“无剩余 P0/P1”。真实 PostgreSQL 迁移曾完成 000070→000072→000073→down73→down72 验证。

### 前端

- 外部记忆契约测试：5/5 通过。
- `npm run type-check`：通过。
- 之前的生产构建：通过。
- 最近一次全量前端测试：177 通过、2 失败。两项是术语规则：新英文 `scopeNote` 使用了 `tenant`，以及文档中的中文 tenant 术语。必须修到全绿，不能当作已完成。

### MemoryCore / MemoryProxy

- MemoryProxy：16 files / 136 tests 全绿，`tsc --noEmit` 通过。
- MemoryCore：相关 6 files / 52 tests 全绿，`npm run build:plugin` 通过。
- 两张源码镜像均构建成功，`/health` 正常，容器用户为 `app`。
- Core 镜像包含 FMind integration；Proxy 镜像内 SQLite 实际 open/exec 成功。
- 两份生产/开发 Compose 的 base 与 `memory` profile 都通过 `docker compose config --quiet`。

### 未完成的验证

`docker ... go test ./... -run '^$' -count=1` 在用户要求暂停时被终止，尚无全仓编译结论。必须重新完整执行。

## 6. 当前未提交状态

### 6.1 FMind：保留，不要混入融合提交

以下是此前用户任务或已有工作副本，当前未提交：

- DeepSeek 示例/模型源：`.env.example`、`frontend/src/components/ModelEditorDialog.vue`。
- FMind 品牌、顺丰本地 Logo、favicon、蓝色主题、登录页：首页相关 frontend 文件与两个 Logo 资源。
- DocReader protobuf、安全解压/压缩炸弹防护及测试。
- Docker app/docreader 源码构建与 Compose 端口/安全调整。
- 小程序 API key 掩码/存储调整及测试。
- `prototype/` 原型目录。

这些改动未纳入本轮融合提交。禁止 `git reset --hard`、`git checkout -- .`、`git clean`、`git add -A`。如需处理，必须分成独立审计与独立提交。

### 6.2 TencentDB Agent Memory：直连插件中间态

OpenClaw/Hermes 直连绑定实现被中断，当前未提交文件：

```text
M  MemoryCore/openclaw-plugin/index.ts
?? MemoryCore/openclaw-plugin/src/fmind-binding.ts
?? MemoryCore/openclaw-plugin/src/fmind-binding.test.ts
M  MemoryCore/hermes-plugin/memory/memory_tencentdb/__init__.py
M  MemoryCore/hermes-plugin/memory/memory_tencentdb/client.py
?? MemoryCore/hermes-plugin/memory/memory_tencentdb/fmind_binding.py
?? MemoryCore/hermes-plugin/tests/test_fmind_binding.py
?? MemoryCore/hermes-plugin/tests/__pycache__/
```

这些文件是进行中的实现，不保证编译或测试通过。先审查后续做，不能丢弃重写。`__pycache__` 是测试生成物，最终提交前应确认后安全排除。

## 7. 剩余任务，按优先级

### P0：完成直连 OpenClaw/Hermes 权威绑定

1. 审查现有中间态，不从零重写。
2. OpenClaw 的 `before_prompt_build`、`agent_end` 和所有记忆工具；Hermes 的 prefetch、sync、flush/工具，都必须在每次数据面操作前取得 FMind 权威上下文。
3. Connector Secret 支持安全环境变量引用，不得写日志或提示词。
4. FMind introspect/verify 使用 2.5 秒超时、禁止 redirect、默认只允许 HTTPS；本机开发 HTTP 需显式开关和精确 host allowlist。
5. 严格验证响应 schema、expiry、`external_agent`、connector type、policy version、capability 与 asset scope。
6. 向 MemoryCore `/v3` 请求只发送 `X-FMind-Binding-Token`，不转发 Connector Secret。
7. recall/capture 必须分别受 capability 和 flag 双重 gate；撤销、过期、身份冲突立即 fail closed。
8. Binding Token 不能进入模型文本、普通 session、调试记录或日志。
9. FMind 关闭时保留 legacy 模式兼容。
10. 补齐成功、403/撤销、过期、identity 冲突、scope gate、secret 泄漏、legacy 回归测试；完成 TypeScript/Python 测试和插件构建后单独提交。

### P0：恢复全仓门禁

1. 修复前端 2 个术语测试并跑全量 `npm test`、type-check、build。
2. 重新完成 FMind Linux/CGO `go test ./... -run '^$' -count=1`。
3. 对 MemoryCore/Proxy 再跑全量测试、TypeScript 检查和源码镜像构建。
4. 对 OpenClaw/Hermes 插件跑独立测试与 build/import smoke。

### P0：端到端最小闭环

使用临时、至少 32 字节的强密钥启动 `memory` profile，验证：

1. FMind、MemoryCore、MemoryProxy 健康检查。
2. 无凭证、错误 Connector Secret、撤销 Token 均被拒绝。
3. 管理员创建绑定，只显示一次 Connector Secret；轮换后旧 secret 失效。
4. 外部 Agent recall 写入/读取 MemoryCore 的记忆向量空间，不进入 FMind RAG 表/collection。
5. L3 matured 事件进入 `pending_review`；未审核不可发布。
6. approve 后发布到对应 tenant+team 的专用 Memory Wiki。
7. Memory Wiki 中不存在 Raw/Knowledge/Chunk/Embedding；普通文件/URL/manual ingest 被拒绝。
8. revoke 后页面归档，外部 Cognition 精确读取和 context assemble 均拒绝。
9. 跨租户、跨团队、跨 external_agent、跨 KB asset ID 均拒绝。
10. 清理时只删除本次测试创建的容器、网络、卷和数据。

### P1：独立只读安全审计

- 检查日志/错误/调试缓存是否包含 Connector Secret、Binding Token、API key 或完整 Markdown evidence。
- 检查所有路由是否在 service 层再次做 tenant/team/role 校验。
- 检查 Memory Wiki 所有读写入口、Agent tool、share、folder、issue 和 ingest 绕过。
- 检查 MemoryCore outbox 重放、重复 event、版本竞争、publish/revoke 崩溃窗口。
- 检查 compose 仅在必要端口对外暴露，生产不允许明文 FMind endpoint。

### P2：已知非阻断项

1. Memory Wiki Start 与 revoke 的锁顺序不同，真实 PostgreSQL 高并发可能发生可重试死锁。
2. `OccurredAt` 纳秒与 PostgreSQL 微秒精度可能使原样重放误报冲突。
3. 已发布旧版本 publication 在新版成为页面 head 后重放，当前安全返回冲突，但体验不完全幂等。
4. 000073 trigger 未覆盖原始 SQL 把软删除 knowledge/chunk 的 `deleted_at` 恢复为 NULL。
5. 000072 down migration 将部分 128 字符列缩回 36，长 ID 会阻塞回滚。
6. Memory Wiki publisher 中仍有少量中文乱码展示字符串。
7. MemoryCore/Proxy 仍通过 `tsx` 直接运行 TypeScript 源码，可后续改为编译产物启动。

## 8. 建议验收命令

### FMind

```powershell
cd E:\worktest\FMind
git status --short
docker run --rm -e GOPROXY=https://goproxy.cn,direct `
  -v E:\worktest\FMind:/workspace `
  -v fmind-audit-gomod:/go/pkg/mod `
  -v fmind-audit-gobuild:/root/.cache/go-build `
  -w /workspace golang:1.26-bookworm `
  go test ./... -run '^$' -count=1

cd frontend
npm test
npm run type-check
npm run build

cd ..
docker compose -f docker-compose.yml config --quiet
docker compose -f docker-compose.yml --profile memory config --quiet
docker compose -f docker-compose.dev.yml config --quiet
docker compose -f docker-compose.dev.yml --profile memory config --quiet
```

### TencentDB Agent Memory

```powershell
cd E:\worktest\TencentDB-Agent-Memory
git status --short

cd MemoryProxy
npm test -- --run
npx tsc --noEmit

cd ..\MemoryCore
npm run build:plugin
# 再按各插件 package 配置运行 OpenClaw TypeScript 测试。
# Hermes 使用仓库声明的 Python 测试入口运行 tests/test_fmind_binding.py。

cd ..
docker build -f MemoryCore/Dockerfile MemoryCore
docker build -f MemoryProxy/Dockerfile MemoryProxy
```

## 9. 可直接复制给其他编程工具的执行提示词

```text
你是本项目的主实现工程师。请继续完成 FMind 与 TencentDB Agent Memory 的融合，但先阅读并严格遵守：

1. E:\worktest\FMind\docs\superpowers\2026-08-17-fmind-agent-memory-pause-handoff.md
2. E:\worktest\FMind\docs\superpowers\specs\2026-08-17-fmind-agent-memory-integration-design.md
3. E:\worktest\FMind\docs\superpowers\plans\2026-08-17-agent-memory-integration.md

两个仓库：
- E:\worktest\FMind，branch main
- E:\worktest\TencentDB-Agent-Memory，branch feat/server_team

开始前必须分别执行 git status --short 和 git log -15。两个工作树都不是干净的。禁止 reset --hard、checkout --、clean、覆盖用户改动、git add -A。所有提交只包含本任务精确文件。

当前只剩三项主任务：
A. 在 TencentDB Agent Memory 中完成被中断的 OpenClaw/Hermes 直连 FMind AgentBinding 集成。保留并审查现有未提交中间代码；每个 recall/capture/tool 操作使用 FMind 权威 Binding Context 与短期 Token；严格 HTTPS/超时/schema/external_agent/connector_type/scope/expiry；Connector Secret 和 Binding Token 不进 prompt/log/session；撤销立即 fail closed；legacy 模式兼容。严格 TDD，完成插件测试和构建后单独提交。
B. 修复 FMind 前端全量测试剩余的 2 个术语失败，完成 npm test/type-check/build；重新完成 Linux/CGO go test ./... 编译门禁。
C. 用源码构建的 memory Compose profile 做真实 E2E：绑定创建/轮换/撤销、MemoryCore recall/capture、L3 matured→pending review→approve→Memory Wiki publish→revoke；断言 Memory Wiki 零 RAG、跨租户/团队/Agent/asset 均拒绝。

冻结规则：MemoryCore 的 L0-L3 记忆向量空间与 FMind RAG 向量空间必须隔离；只有审核通过的 L3 Markdown 发布到专用 Memory Wiki；FMind 原有内部 Agent 记忆不改；业务服务从源码构建。

实现要求：
- 先写失败测试，再最小实现，再跑全量回归。
- 不修改 MemoryCore 的原有提取/评分算法。
- 不把静态配置中的 team/user/agent 当作 FMind 模式权威。
- 不把 Connector Secret 发送给 MemoryCore 或模型厂商。
- 不在普通日志、错误响应、inspection、prompt、session metadata 中保存 Secret/Token。
- 遇到数据库/网络瞬时错误返回可重试错误；权限、冲突、非法事件返回永久错误。
- 每个逻辑切片独立提交，提交前 git diff --check，并列出精确测试证据。

完成时输出：
1. 两个仓库的 commit SHA 与精确文件清单。
2. 所有测试、构建、Compose、E2E 的命令与退出码。
3. 尚未解决的 P0/P1/P2；没有也必须明确写“无”。
4. git status --short，说明哪些剩余改动属于用户此前工作，未被触碰。

不要声称完成，除非上述 A/B/C 全部验证通过。
```

## 10. 后续交给 Codex 的只读审计提示词

```text
请进入只读审计模式。禁止修改、格式化、提交、启动会改变持久数据的流程。

先读：
E:\worktest\FMind\docs\superpowers\2026-08-17-fmind-agent-memory-pause-handoff.md

然后审计其他编程工具提交的两个仓库 diff。重点验证：
1. OpenClaw/Hermes 是否真正每次使用 FMind Binding 权威，而不是静态 team/user/agent。
2. Secret/Token 是否可能进入日志、提示词、session、厂商请求或缓存泄漏。
3. recall/capture/cognition/wiki 是否严格执行 capability 与 asset scope 交集。
4. 撤销、过期、轮换、policy version 变化是否立即 fail closed。
5. L3 是否必须审核、只进入专用 Memory Wiki，并保持零 RAG。
6. tenant/team/external_agent/KB/page/document 是否存在 IDOR 或共享绕过。
7. outbox 重放、并发 publish/revoke、崩溃恢复、immutable revision 是否成立。
8. 两张业务镜像是否确由当前源码构建，Compose 是否能健康启动。

只报告有精确文件/行号/复现步骤的发现，按 P0/P1/P2 排序。执行测试可以，但不得修复。最后给出 PASS 或 FAIL 门禁，并列出缺失的验收证据。
```

## 11. 恢复入口

实现工具应从本文件第 6、7、9 节开始。Codex 后续只按第 10 节做审计，不再承担主实现。

## 12. Cloud Code CI 多智能体执行协议

以下提示词可直接复制给 Cloud Code CI。Cloud Code 作为唯一总调度器，负责创建/回收子智能体、分配不重叠文件范围、合并提交、运行最终验证，并在指定位置写入待审计 Markdown 报告。

```text
你是 Cloud Code CI 的总调度器。请直接执行 FMind-Agent Memory 融合任务，不要只给计划。

项目路径：
- FMind：E:\worktest\FMind
- TencentDB Agent Memory：E:\worktest\TencentDB-Agent-Memory

首先完整阅读：
- E:\worktest\FMind\docs\superpowers\2026-08-17-fmind-agent-memory-pause-handoff.md
- E:\worktest\FMind\docs\superpowers\2026-08-17-fmind-agent-memory-integration-design.md
- E:\worktest\FMind\docs\superpowers\plans\2026-08-17-agent-memory-integration.md
- E:\worktest\时空世界整体架构\release\时空世界-产品说明及PRD详细设计书-V1.0.html

执行前分别保存两个仓库的：
git status --short
git log --oneline -15

严禁：
- git reset --hard
- git checkout -- .
- git clean
- 删除未知文件
- git add -A
- 覆盖用户以前的 Logo、主题、DeepSeek、DocReader、小程序、Docker 或其他未提交修改

你必须启用多智能体并行执行；如果当前 CI 不支持并行，则按同样边界顺序执行。每个智能体必须使用独立 worktree/branch，或严格文件锁。禁止两个智能体同时修改同一文件。所有子智能体完成后先回报给你，由你统一审查和合并，不能自行把代码直接写回主工作树。

固定角色和文件边界：

Agent 0：Cloud Code 总调度器
- 负责读取设计、拆分任务、创建 worktree、分配任务、收集测试、合并提交。
- 不直接修改业务代码。
- 为每个子任务记录 branch、commit、文件范围和测试结果。

Agent 1：OpenClaw Binding 实现者
- 只允许修改：E:\worktest\TencentDB-Agent-Memory\MemoryCore\openclaw-plugin\**
- 继续审查并完成已有 fmind-binding.ts、fmind-binding.test.ts 和 index.ts 中间态。
- 实现 before_prompt_build、agent_end、memory tools 的 FMind Binding 权威校验。
- 验证 external_agent、connector_type、expiry、policy version、capability、asset scope。
- 只向 MemoryCore 发送短期 X-FMind-Binding-Token，不发送 Connector Secret。
- 完成 OpenClaw 测试、TypeScript 检查和插件构建。
- 如需修改共享 MemoryCore 文件，只能报告给 Agent 0，不得自行越界修改。

Agent 2：Hermes Binding 实现者
- 只允许修改：E:\worktest\TencentDB-Agent-Memory\MemoryCore\hermes-plugin\**
- 继续审查并完成已有 fmind_binding.py、client.py、__init__.py 和 tests 中间态。
- 实现 prefetch、sync、flush、capture、recall 和工具调用的 Binding 权威校验。
- 保留 legacy 模式。
- 完成 Python 测试、导入 smoke、secret 脱敏和 scope 测试。
- 如需修改共享 MemoryCore 文件，只能报告给 Agent 0，不得自行越界修改。

Agent 3：FMind 应用验证者
- 只允许修改：FMind frontend 测试和术语相关文件；默认只读。
- 复现并修复之前全量 npm test 的 2 个 terminology 失败。
- 执行 npm test、npm run type-check、npm run build。
- 在 Linux/CGO 环境执行 FMind go test ./... -run '^$' -count=1。
- 不修改后端融合逻辑，不处理其他用户未提交文件。

Agent 4：源码构建与 E2E 验证者
- 默认只读，不修改业务代码。
- 验证四个 Compose config 命令、MemoryCore/MemoryProxy 源码 Docker build、health、non-root、HTTPS fail-closed。
- 用临时强密钥完成 AgentBinding 创建/轮换/撤销、MemoryCore capture/recall、L3 review/publish/revoke。
- 断言 Memory Wiki 不产生 Raw、Knowledge、Chunk、Embedding、Graph、Vector、Keyword 数据。
- 断言跨 tenant、team、external_agent、asset scope 全部拒绝。
- 发现缺陷时只能向 Agent 0 提交带复现步骤的报告，不能自行改共享文件。

Agent 5：最终安全审计者
- 只读，禁止修改和提交。
- 审计所有已合并 diff、日志、错误响应、prompt、session metadata、上游请求、scope、revoke、rotation、outbox、并发和崩溃窗口。
- 按 P0/P1/P2 给出精确文件、行号和复现方式。

实现规则：

1. 严格 TDD：失败测试 → 最小实现 → 定向测试 → 全量测试 → 构建 → diff check。
2. 保持 L0-L3 提取算法不变。
3. 保持 MemoryCore 记忆向量空间与 FMind RAG 向量空间隔离。
4. 只有审核通过的 L3 才能进入专用 Memory Wiki。
5. Connector Secret 和 Binding Token 不得进入日志、prompt、session、vendor 请求。
6. 撤销、过期、scope 冲突、policy version 不匹配必须 fail closed。
7. 每个子智能体独立提交，Agent 0 只合并经过测试的精确提交。
8. 合并前执行 git diff --check，禁止混入用户已有工作区修改。

最终验收至少包含：

- OpenClaw recall/capture/tool 测试
- Hermes prefetch/sync/flush/capture/recall 测试
- FMind frontend 全量测试、type-check、build
- FMind Linux/CGO 全仓编译
- MemoryCore 全量测试与插件构建
- MemoryProxy 全量测试与 tsc
- 两张源码镜像构建和 health smoke
- Compose 四种 config 检查
- AgentBinding 创建/轮换/撤销 E2E
- L3 pending_review → approve → Memory Wiki publish E2E
- revoke 后 Wiki 归档和 Cognition 拒绝读取
- 零 RAG 数据断言
- 跨租户、跨团队、跨 Agent、跨资产拒绝断言

每个子智能体必须反馈：
- 任务名称
- 实际修改文件
- branch 和 commit SHA
- 运行的每条命令和退出码
- 测试结果
- 构建结果
- 未解决 P0/P1/P2
- 是否生成临时容器、卷、测试数据、__pycache__

你必须在全部任务结束后生成 Markdown 报告。优先写入：
C:\Users\Airson\Desktop\FMind-Agent-Memory-Audit-Report.md

如果桌面路径不可写，则写入：
E:\worktest\FMind\audit-reports\FMind-Agent-Memory-Audit-Report.md

报告必须包含：

# FMind-Agent Memory 执行与待审计报告
## 执行时间和 Cloud Code 运行环境
## 子智能体调度表
## 两个仓库的 commit 和精确文件清单
## 每个测试/构建/E2E 命令及退出码
## AgentBinding 与外部 Agent 安全验证
## L3 审核与 Memory Wiki 验证
## 零 RAG 与向量空间隔离验证
## Compose/镜像/健康检查验证
## 未解决问题（P0/P1/P2）
## 两个仓库最终 git status --short
## 用户原有未提交文件清单
## 给后续 Codex 只读审计者的交接结论

报告必须明确写出：
- 是否改变 L0-L3 提取算法
- 是否改变 FMind 原有知识库 RAG 路径
- 是否保留 legacy Agent 模式
- Connector Secret 是否出现在日志、prompt、session 或 vendor 请求
- revoke/expiry/policy version 是否即时生效
- Memory Wiki 是否可能进入普通 RAG
- 是否仍有 P0/P1

报告写完后，不要删除报告，不要清理用户已有工作树，只清理本次创建且已确认安全的临时容器/卷/测试数据。

最后只向调用方返回：
1. 报告绝对路径
2. 两个仓库最终 commit SHA
3. P0/P1/P2 摘要
4. 是否建议交给 Codex 做只读审计
```

Cloud Code 完成后，后续只需要把它返回的 `FMind-Agent-Memory-Audit-Report.md` 路径交给 Codex。Codex 不再直接参与实现，只读取该报告和对应提交进行审计。
