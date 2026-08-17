# 云端 MVP 部署

此部署栈包含 FMind 前端、应用、ParadeDB PostgreSQL、Qdrant 和基础 DocReader。

- Redis 使用阿里云 Tair。
- 文件对象存储使用阿里云 OSS。
- 新建知识库仅允许使用 OSS；旧的 MinIO/本地存储配置不会在此栈中启用。
- 当前向量模型通过 FMind 管理界面配置为硅基流动 `BAAI/bge-m3`（1024 维）。
- 当前问答、摘要与推荐问题模型使用火山 AgentPlan `doubao-seed-2.0-pro`；同一 AgentPlan Provider 下的模型共享一份加密 Key。
- 不启动 MinIO、Redis、MinerU/ODL 混合解析器或本地模型。
- 所有数据库和向量端口仅在 Compose 网络内可见；前端仅绑定到宿主机 `127.0.0.1`，由宿主机 Nginx 反向代理。

## 首次部署

1. 在 ECS 复制 `.env.example` 为 `.env`，填写 Tair、OSS 和随机密钥。`.env` 不进入 Git。
2. 确保 Tair 白名单允许 ECS 私网 IP，并为 OSS 创建仅限 `keystore001/fmind-mvp/*` 的 RAM 凭据。
3. 启动：

   ```bash
   docker compose -f docker-compose.yml up -d --build
   ```

4. 验证：

   ```bash
   docker compose -f docker-compose.yml ps
   curl -f http://127.0.0.1:18081/health
   ```

5. 通过前端完成首次管理员、OSS、Qdrant、硅基流动 Embedding 与火山 AgentPlan Chat 模型配置。验证无误后，再将 Nginx 的 `fmind.boliboliworld.cn` upstream 切换到 `127.0.0.1:18081`。

## 从本地 Docker 迁移

1. 先对云端 PostgreSQL 执行 `pg_dump -Fc` 备份，并保留在 ECS 的受限目录中。
2. 使用同一 ParadeDB 主版本从本地 PostgreSQL 导出 `pg_dump -Fc`，校验 SHA-256 后恢复到云端；恢复期间停止 `app` 与 `frontend`。
3. 使用 **MinIO API** 枚举和导出源对象后再复制到 OSS。不要直接复制 Docker 数据卷：MinIO 文件系统后端包含 `xl.meta`、`part.*` 等内部对象片段，并不是可供 FMind 读取的原始附件。
4. 恢复后将租户和知识库的存储后端切换为 `oss`，并保留 `STORAGE_ALLOW_LIST=oss`；以云端 `.env` 的 OSS 凭据作为唯一有效配置。
5. Qdrant 不直接复制。配置云端嵌入模型后，按当前模型维度重新建立索引。

如果源 MinIO 没有对象，数据库的历史文件引用不能凭空恢复；知识库元数据和已保存的分块仍可迁移，原始附件需重新上传。

### MinIO 到 OSS 的安全上传工具

`tools/oss-upload-tree` 用于将已经通过 MinIO API 导出的**逻辑文件目录**上传至 OSS。它只从环境变量读取 OSS 凭据，默认先执行 `HeadObject` 并跳过已存在对象；只有在使用全新、隔离的目标前缀时才可显式加 `-overwrite`。它不会删除源数据或 OSS 对象。

在有 Go 开发环境的机器上构建 Linux 静态二进制：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -o oss-upload-tree ./deploy/cloud-mvp/tools/oss-upload-tree
```

在 ECS 上执行前，加载该部署目录的 `.env`；不要将 AccessKey 写入命令行或日志：

```bash
set -a && . ./.env && set +a
./oss-upload-tree \
  -source /restricted/export/blexwiki \
  -prefix "${OSS_PATH_PREFIX}legacy-import-YYYYMMDD/blexwiki" \
  -dry-run
```

确认预演对象数和总大小正确后，去掉 `-dry-run`。若 RAM 策略不允许 `HeadObject`，请先授予该用户对 `keystore001/fmind-mvp/*` 的 `GetObject`、`PutObject` 和 `ListObjects` 权限；不要通过盲目重试或改用主账号密钥绕过权限控制。

对象数、大小和抽样下载都验证通过后，再使用 `tools/migrate-legacy-minio-refs.sql` 将数据库中的历史 `minio://blexwiki/` 引用改为新的 `oss://` 前缀。该 SQL 包含事务但默认**不提交**，并会列出剩余旧链接；必须先完成 PostgreSQL 备份、确认所有统计值为零，才允许提交。之后再在 FMind 中重建知识库索引。

如果历史知识库的规则仍将 PDF、DOC 指向 `mineru_cloud`，重建不会实际使用 DocReader。使用 `tools/switch-parser-rules-to-docreader.sql` 预演并提交规则切换：PDF/DOC 走 DocReader `builtin`，原先指向 MinerU 的 PPT/XLS 走 DocReader `markitdown`；其余规则保持原样。完成切换后再触发重新解析。

## 凭据轮换

Tair 密码轮换后，不要把密码粘贴到 Shell 历史或 Git 中。在部署目录执行：

```bash
./update-redis-password.py
docker compose --env-file .env up -d --force-recreate app
```

该脚本只交互式更新 `.env` 中的 `REDIS_PASSWORD`，并保持该文件权限为仅所有者可读写。

## 文档解析能力与限制

DocReader 在同一 Compose 网络中以 gRPC 提供基础解析，端口不映射到 ECS 公网。它支持 PDF、Office 等复杂格式的常规解析；当前按 2C4G MVP 规格限制为单任务、单页渲染进程，并使用 160 DPI。

部署栈同时保留源码构建定义：若 ECS 的镜像加速器无法获取上游 `fmind-docreader` 预构建镜像，Compose 会使用本项目的 `docker/Dockerfile.docreader` 构建同一版本。可通过 `DOCREADER_APT_MIRROR` 指定 Debian 镜像根地址，例如 `https://mirrors.aliyun.com`。

为控制构建体积和外部下载风险，MVP 构建不下载 GitHub 上的健康检查二进制，也不安装 Playwright 浏览器运行时；健康检查由已安装的 Python gRPC 组件执行。网页解析不属于当前能力范围，Python 依赖可通过 `DOCREADER_PYPI_INDEX_URL` 指定镜像。

当前仓库的 `docreader/uv.lock` 与 `pyproject.toml` 声明并非完全同步，因此云端构建会在一次性的构建层内按 `pyproject.toml` 重新解析运行时依赖，而不会修改工作区的锁文件；后续应在开发环境统一更新锁文件后再恢复严格锁定构建。

不部署 MinerU 或 OpenDataLoader 混合服务。因此扫描版、高精度版面恢复和 OCR 不是此阶段目标；这类文件后续需要单独部署 MinerU 或升级解析节点。DocReader 负责提取文本，嵌入模型负责向量化，二者需分别配置。

## 当前生产基线

当前实例、端口、模型、备份、发布、回滚和共享 ECS 注意事项统一以[部署与运维手册](../../docs/DEPLOYMENT_RUNBOOK.md)为准。历史 WireGuard/本机全栈不再是生产依赖。
