# 云端 MVP 部署

此部署栈只包含 Keystone 前端、应用、ParadeDB PostgreSQL 和 Qdrant。

- Redis 使用阿里云 Tair。
- 文件对象存储使用阿里云 OSS。
- 新建知识库仅允许使用 OSS；旧的 MinIO/本地存储配置不会在此栈中启用。
- 向量模型在首次登录后通过 Keystone 管理界面配置为火山 AgentPlan。
- 不启动 MinIO、Redis、DocReader 或本地模型。
- 所有数据库和向量端口仅在 Compose 网络内可见；前端仅绑定到宿主机 `127.0.0.1`，由宿主机 Nginx 反向代理。

## 首次部署

1. 在 ECS 复制 `.env.example` 为 `.env`，填写 Tair、OSS 和随机密钥。`.env` 不进入 Git。
2. 确保 Tair 白名单允许 ECS 私网 IP，并为 OSS 创建仅限 `keystore001/keystone-mvp/*` 的 RAM 凭据。
3. 启动：

   ```bash
   docker compose -f docker-compose.yml up -d --build
   ```

4. 验证：

   ```bash
   docker compose -f docker-compose.yml ps
   curl -f http://127.0.0.1:18081/health
   ```

5. 通过前端完成首次管理员、OSS 连通性与火山 AgentPlan 模型配置。验证无误后，再将 Nginx 的 `keystone.boliboliworld.cn` upstream 从旧前端切换到 `127.0.0.1:18081`。

## 从本地 Docker 迁移

1. 先对云端 PostgreSQL 执行 `pg_dump -Fc` 备份，并保留在 ECS 的受限目录中。
2. 使用同一 ParadeDB 主版本从本地 PostgreSQL 导出 `pg_dump -Fc`，校验 SHA-256 后恢复到云端；恢复期间停止 `app` 与 `frontend`。
3. 使用 MinIO API 枚举源对象后再复制到 OSS。不要根据数据库中的 `minio://` 文件路径推断对象一定存在。
4. 恢复后将租户和知识库的存储后端切换为 `oss`，并保留 `STORAGE_ALLOW_LIST=oss`；以云端 `.env` 的 OSS 凭据作为唯一有效配置。
5. Qdrant 不直接复制。配置云端嵌入模型后，按当前模型维度重新建立索引。

如果源 MinIO 没有对象，数据库的历史文件引用不能凭空恢复；知识库元数据和已保存的分块仍可迁移，原始附件需重新上传。

## 凭据轮换

Tair 密码轮换后，不要把密码粘贴到 Shell 历史或 Git 中。在部署目录执行：

```bash
./update-redis-password.py
docker compose --env-file .env up -d --force-recreate app
```

该脚本只交互式更新 `.env` 中的 `REDIS_PASSWORD`，并保持该文件权限为仅所有者可读写。

## 明确限制

本阶段不运行 DocReader/MinerU。因此 PDF、Office 文档的解析与重新解析不可用；仅用于验证应用、模型配置、文本/Markdown 知识和检索问答流程。
