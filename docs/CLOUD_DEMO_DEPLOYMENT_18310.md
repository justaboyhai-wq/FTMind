# FMind 云服务器测试/演示部署手册

本文用于将 FMind 部署到已有业务服务器，目标是测试和演示，不覆盖服务器上原有的哪吒及其他项目。

## 1. 服务器与端口规划

已对服务器 115.191.64.43 做过只读检查：

| 项目 | 检查结果 |
| --- | --- |
| 系统 | Ubuntu，x86_64 |
| CPU | 16 vCPU |
| 内存 | 约 30 GiB，可用约 12 GiB |
| 磁盘 | 394 GiB，剩余约 138 GiB |
| Swap | 当前未配置 |
| FMind 前端 | 18310 |
| FMind App 宿主机端口 | 18311，仅绑定 127.0.0.1 |

已确认 18310、18311 当前没有监听，也没有现有容器映射。哪吒使用的 18005、18006、18007、18278、18288、18289 等端口不使用、不修改。

访问地址：

    http://115.191.64.43:18310/login

如果以后配置域名和 HTTPS，建议由 Nginx 对外开放 80/443，再反向代理到 127.0.0.1:18310。

## 2. 首次部署范围

首次只启动核心服务：

- frontend：网页前端
- app：FMind API、任务队列和业务逻辑
- docreader：PDF/Office/PPTX 文档解析
- postgres：业务数据库和默认向量存储
- redis：任务队列和流式状态

默认命令只启动 5 个核心服务。当前本机 FMind 还运行了 Memory 逻辑服务；它由 `memory-core` 和 `memory-proxy` 两个容器组成，因此如果按容器数量统计会多 2 个容器。

本机 FMind 服务对应关系如下：

| 逻辑服务 | Compose 服务/容器 | 默认部署 |
| --- | --- | --- |
| 前端 | `frontend` | 启动 |
| API/任务 | `app` | 启动 |
| 文档解析 | `docreader` | 启动 |
| PostgreSQL | `postgres` | 启动 |
| Redis | `redis` | 启动 |
| Agent Memory | `memory-core` + `memory-proxy` | 默认关闭 |

本机的 `foxonto-*`、`foxonto-mysql`、`foxonto-redis` 等是另一套项目，不属于 FMind，不会部署到 FMind Compose 中。

首次不启用 MinIO、Neo4j、Weaviate、Qdrant/Milvus/Doris、SearXNG、Langfuse、Dex、MCP 等其他可选服务，避免额外资源占用和端口冲突。

如果你希望云端与本机 FMind 完全一致，使用 Memory profile 启动 7 个容器（6 个逻辑服务）：

    docker compose --env-file .env --profile memory build
    docker compose --env-file .env --profile memory up -d
    docker compose --env-file .env --profile memory ps

Memory Proxy 默认只绑定 `127.0.0.1:8096`，不需要占用 18310 之后的公网端口；不要把它改成公网监听。

## 3. 云安全组

建议只开放：

    22/tcp      SSH，最好限制为办公出口 IP
    18310/tcp   FMind 演示前端
    80/tcp      可选，域名 HTTP
    443/tcp     可选，域名 HTTPS

不要开放 18311、5432、6379、50051、7474、7687、9000、9001、6333、6334。服务器当前 UFW 未启用，云平台安全组仍必须配置。

## 4. 系统准备

以下命令在服务器执行，不会修改哪吒：

    apt-get update
    apt-get install -y git curl openssl ca-certificates

如果尚未安装 Docker：

    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
    docker version
    docker compose version

服务器当前没有 Swap。建议增加 4 GiB，降低首次源码构建和大型文档解析时 OOM 的风险：

    swapon --show
    fallocate -l 4G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab

如果已经存在 Swap，不要重复创建。

## 5. 获取代码

使用独立目录，不要放入哪吒目录：

    mkdir -p /opt/fmind
    cd /opt/fmind
    git clone <FMind仓库地址> .
    git checkout <已验证的提交或分支>

如果使用压缩包上传，解压后确认目录中存在 docker-compose.yml、docker/Dockerfile.app、docker/Dockerfile.docreader、frontend、config、docreader。

## 6. 创建 .env

    cd /opt/fmind
    cp .env.example .env
    chmod 600 .env

编辑 .env，至少确认以下值：

    FRONTEND_PORT=18310
    APP_PORT=127.0.0.1:18311
    APP_BACKEND_PORT=8080

    DB_DRIVER=postgres
    DB_HOST=postgres
    DB_PORT=5432
    DB_USER=postgres
    DB_NAME=FMind
    DB_PASSWORD=替换为随机强密码

    REDIS_ADDR=redis:6379
    REDIS_DB=0
    REDIS_PASSWORD=替换为随机强密码

    RETRIEVE_DRIVER=postgres
    DOCREADER_ADDR=docreader:50051
    DOCREADER_TRANSPORT=grpc

    JWT_SECRET=替换为随机值
    TENANT_AES_KEY=替换为随机值
    SYSTEM_AES_KEY=替换为随机值
    CRYPTO_MASTER_KEY=替换为随机值
    CRYPTO_SALT=替换为随机值

    DOCREADER_MARKITDOWN_MAX_WORKERS=1
    DOCREADER_PDF_RENDER_MAX_WORKERS=1
    FMIND_MODEL_MAX_CONCURRENCY=2
    FMIND_ASYNQ_CORE_CONCURRENCY=1

生成随机值：

    openssl rand -hex 32
    openssl rand -base64 32

DB_PASSWORD、REDIS_PASSWORD 是服务密码，不等同于 FMind 网页登录账号密码。网页管理员账号在首次打开登录页后创建。真实密钥不能提交到 Git。

## 7. 校验、构建和启动

先只检查配置：

    cd /opt/fmind
    docker compose --env-file .env config --quiet

从当前源码构建镜像并启动：

    docker compose --env-file .env build
    docker compose --env-file .env up -d
    docker compose --env-file .env ps

首次构建会下载 Go、Node、Python 和系统依赖，服务器当前 16 vCPU/30 GiB，适合直接基于源码构建，不需要预先下载 FMind 镜像。不要未经确认执行 docker system prune 或 docker system prune -a，服务器已有其他项目。

## 8. 启动验证

查看服务和日志：

    docker compose --env-file .env ps
    docker compose --env-file .env logs --tail=100 app
    docker compose --env-file .env logs --tail=100 docreader

检查 App：

    curl -fsS http://127.0.0.1:18311/health

检查前端：

    curl -I http://127.0.0.1:18310/login

浏览器访问：

    http://115.191.64.43:18310/login

确认端口：

    ss -ltnp | grep -E ':(18310|18311)\b'
    docker ps --format 'table {{.Names}}\t{{.Ports}}'

预期前端监听 0.0.0.0:18310，App 仅监听 127.0.0.1:18311；PostgreSQL、Redis、DocReader 不应出现在公网监听列表。

## 9. 首次登录和模型配置

1. 打开 http://115.191.64.43:18310/login。
2. 创建或使用 FMind 管理员账号。
3. 在模型配置中添加 DeepSeek Provider，填写 API 地址、API Key 和模型名称。
4. 保存后执行一次简单对话。
5. 上传一个小型 PDF 或 DOCX，确认上传、解析、索引、检索、回答链路。

远程 DeepSeek API 不需要 GPU。只有运行本地大模型时才需要 GPU 和更大显存。

## 10. 日常运维

查看资源：

    free -h
    df -h /
    docker stats --no-stream
    docker system df

更新代码：

    cd /opt/fmind
    git fetch origin
    git checkout <已验证的提交或分支>
    git pull --ff-only origin <分支>
    docker compose --env-file .env config --quiet
    docker compose --env-file .env up -d --build
    docker compose --env-file .env ps

只重启 App：

    docker compose --env-file .env up -d --force-recreate app

停止 FMind（不删除数据卷）：

    docker compose --env-file .env stop

不要执行：

    docker compose down -v
    docker system prune -a

这些命令可能删除 FMind 数据卷或影响服务器上其他项目。

## 11. 最小备份

    mkdir -p /opt/fmind/backups
    stamp=$(date +%Y%m%d-%H%M%S)
    docker compose --env-file .env exec -T postgres pg_dump \
      -U postgres -d FMind -Fc \
      > "/opt/fmind/backups/fmind-$stamp.dump"
    sha256sum "/opt/fmind/backups/fmind-$stamp.dump"
    chmod 600 /opt/fmind/.env

同时安全保存 .env。不要把 .env、数据库 dump、API Key 或登录密码提交到 Git。

## 12. 常见问题

### 前端打开但 API 失败

    docker compose --env-file .env logs --tail=200 app frontend
    curl -fsS http://127.0.0.1:18311/health

重点检查 APP_BACKEND_PORT=8080。它是容器内 App 端口，不要改成 18311。

### 文档解析内存不足

保持 DOCREADER_MARKITDOWN_MAX_WORKERS=1、DOCREADER_PDF_RENDER_MAX_WORKERS=1、FMIND_MODEL_MAX_CONCURRENCY=1，并避免同时上传多个大文件。

### 端口冲突

    ss -ltnp | grep -E ':(18310|18311)\b'
    docker ps -a --format '{{.Names}}\t{{.Ports}}'

如果冲突，只修改 .env 中的 FRONTEND_PORT 或 APP_PORT，继续选择 18310 之后的空闲端口，并同步修改云安全组。

### 磁盘不足

    docker system df
    df -h /

先确认具体占用，不要直接清理共享 Docker 缓存或哪吒相关镜像。

## 13. 回滚

    cd /opt/fmind
    git status --short
    git checkout <上一个已验证提交>
    docker compose --env-file .env up -d --build
    docker compose --env-file .env ps

回滚代码不等于回滚数据库。涉及数据库迁移时，必须先恢复备份并确认兼容性。

## 14. 完成检查表

- [ ] 哪吒容器和配置未被停止、删除或修改
- [ ] 18310 已加入云安全组
- [ ] 18311 仅绑定 127.0.0.1
- [ ] PostgreSQL、Redis、DocReader 未对公网监听
- [ ] .env 已替换随机密钥并设置为 600
- [ ] docker compose config --quiet 通过
- [ ] App /health 返回成功
- [ ] 浏览器可以打开 /login
- [ ] DeepSeek 模型调用成功
- [ ] 小型 PDF/DOCX 上传解析成功
- [ ] 已创建数据库备份
