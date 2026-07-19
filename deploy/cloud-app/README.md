# 云端 App / 本地组件部署

本目录提供“云端 App + 本地组件”的迁移模板：云端部署 Keystone `frontend` 和 `app` 两个容器；PostgreSQL、Redis、MinIO、Qdrant、DocReader 和模型服务运行在本地网络，并通过 WireGuard 或等价专网以固定私网地址提供服务。

> 当前生产实例采用更保守的 **云端前端代理 + 本机完整 App** 模式，而不是本模板的双 App 模式。这样可避免复制现有密钥、重复消费 Asynq 队列和本机/云端状态漂移。请先阅读 [部署与运维手册](../../docs/DEPLOYMENT_RUNBOOK.md)，未完成迁移评审前不要启动第二个连接同一 Redis/PostgreSQL 的 App。

## 前置条件

1. 云端 ECS 与本地网络已建立 WireGuard；所有 `10.76.0.x` 示例地址需替换为实际地址，且不得与 VPC 或 LAN 网段重叠。
2. 本地服务防火墙仅允许云端 WireGuard 地址访问所需端口：PostgreSQL `5432`、Redis `6379`、MinIO `9000`、Qdrant gRPC `6334`、DocReader gRPC `50051` 和模型 API。
3. 本地 MinIO/S3 是必需项。不要配置 `STORAGE_TYPE=local`，因为云端 App 与本地 DocReader 不共享文件系统。
4. 生产环境应通过宿主机或独立反向代理将 `443` 转发到 `127.0.0.1:8080`，并设置 `APP_EXTERNAL_URL` 为 HTTPS 域名。

## 部署

在服务器上取得此仓库后：

```bash
cd keystone/deploy/cloud-app
cp .env.example .env
chmod 600 .env
# 编辑 .env，写入真实 VPN 地址和密钥
docker compose config -q
docker compose build
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:8080/health
```

在尚未创建真实 `.env` 前，可使用下列命令检查 Compose 模板：

```bash
ENV_FILE=.env.example docker compose --env-file .env.example config -q
```

`docker compose build` 会从当前仓库源码构建前端和 App，因此部署的是当前提交，而非不受控的 `latest` 镜像。

## 发布与回滚

发布前在服务器执行 `git fetch origin && git checkout <approved-commit>`，再重新执行 `docker compose build && docker compose up -d`。回滚时切换回上一已验证提交并重复构建/启动。不得把 `.env`、WireGuard 私钥、数据库密码或模型 API Key 提交到 Git。
