# Baoan Policy Collector

独立下载宝安区政策法规库的 `zcfg.js`、详情 JSON、原文 HTML、附件和官网显式关系，输出不可变 Raw Package。采集器不依赖 FTMind、PostgreSQL、Redis、MinIO 或浏览器。

```powershell
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector retry --failed --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector verify --all --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector export-raw --data-dir ./baoan-policy-data --output-dir ./raw-export
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data --incremental-cron "0 2 * * *" --full-cron "0 3 * * 0"
go run ./cmd/baoan-policy-collector serve-rss --data-dir ./baoan-policy-data --addr :18320 --base-url https://collector.example.com
```

## Existing FTMind knowledge migration

The migration command is intentionally dry-run by default. It loads canonical
packages, validates the `post_<id>` mapping against the target knowledge base
when `DATABASE_URL` is available, writes a complete rollback manifest, and
updates only title/file name/metadata/tag relations when `--apply` is used.
Knowledge IDs, chunks, vectors and physical storage paths are not rewritten.

```powershell
go run ./cmd/baoan-policy-collector migrate `
  --kb-id e3cfbfb3-a90e-49c0-82c2-f95e7f595d54 `
  --feed-url http://115.191.64.43:18320/feed.xml `
  --data-dir .\baoan-policy-data `
  --db-url "$env:DATABASE_URL" `
  --datasource-id "$env:BAOAN_DATASOURCE_ID" `
  --dry-run `
  --rollback-file .\baoan-policy-migration-rollback.json

go run ./cmd/baoan-policy-collector migrate `
  --kb-id e3cfbfb3-a90e-49c0-82c2-f95e7f595d54 `
  --feed-url http://115.191.64.43:18320/feed.xml `
  --data-dir .\baoan-policy-data `
  --db-url "$env:DATABASE_URL" `
  --datasource-id "$env:BAOAN_DATASOURCE_ID" `
  --apply `
  --rollback-file .\baoan-policy-migration-rollback.json
```

`--apply` stops on missing or duplicate policy IDs and runs in one PostgreSQL
transaction. Keep the rollback manifest and a database backup until RSS first
sync has been verified.

To restore the captured names/metadata after pausing the RSS source:

```powershell
go run ./cmd/baoan-policy-collector migrate --rollback `
  --rollback-file .\baoan-policy-migration-rollback.json `
  --db-url "$env:DATABASE_URL"
```

The collector is an independent Go module. It discovers `zcfg.js` from the seed
HTML on every run, downloads detail JSON, original HTML, attachments, and
official relationship metadata, then writes immutable packages under
`<data-dir>/policies`. Each run also writes discovery/event ledgers and an
official-dimension dictionary snapshot under `<data-dir>/dictionaries`.
It does not require the FTMind services.

Validation commands:

```powershell
go test ./...
go vet ./...
go test -tags=live ./internal/collector -run TestLiveProtocol
.\scripts\verify-run.ps1 -DataDir .\baoan-policy-data
.\scripts\quality-report.ps1 -DataDir .\baoan-policy-data
```

Use `daemon` for the built-in scheduled mode. It handles Ctrl+C/SIGTERM and
skips overlapping runs. For production, run the binary as a systemd service or
container and mount the data directory as persistent storage.

## Raw export and RSS gateway

`<data-dir>` is already the immutable `baoan.raw/v1` package store. It is not a
folder that FTMind's document uploader imports recursively as one transaction;
uploading every JSON sidecar would create noisy independent documents. For a
one-time document import, export exactly one `baoan.canonical-md/v1` Markdown
document per policy:

```powershell
.\scripts\export-raw.ps1 -DataDir .\baoan-policy-data -OutputDir .\raw-export
```

The exporter marks its output directory as managed and refuses to mix with an
unmanaged non-empty directory. Re-running it replaces only generated Markdown
files, preventing stale JSON sidecars or deleted policies from remaining in the
knowledge-base upload set.

For ongoing synchronization, run `serve-rss` against the persistent data
directory. Configure FTMind's RSS/Atom data source with
`http://<host>:18320/feed.xml` (or the public reverse-proxy URL) and set its
normal sync schedule. Raw export and RSS both use the same canonical assembler.
Each document contains official dimensions, application dates/status, source
URLs, official relations, archived/missing attachment inventory, original
policy body, snapshot ID and audit path. Source JSON, HTML, downloaded files and
checksums remain in the raw package for audit and later Wiki transformation.
The gateway is read-only and reflects each package's `latest.json`, so a later
incremental collector run automatically changes the next RSS sync.
`/healthz` assembles every latest package and returns HTTP 503 if any document
cannot be rendered; `/feed.xml` also fails closed instead of silently omitting a
bad package.

## Standalone Docker deployment

The project ships as a two-service stack: `collector` performs scheduled writes
and `rss` serves a read-only view over the same `./data` bind mount. Copy an
existing small data set into `./data` before deployment if the feed must be
available immediately; later incremental jobs continue from its SQLite state.

```bash
cp .env.example .env
# Set BAOAN_RSS_BASE_URL to the externally reachable HTTPS URL.
docker compose up -d --build
docker compose logs -f collector rss
```

Only RSS port 18320 is bound, and it defaults to loopback. Put it behind the
server's existing Nginx, then configure FTMind with the proxied `/feed.xml` URL.
The collector has no public port. Cron expressions use Asia/Shanghai.

完整 run 会从入口 HTML 重新发现 `/zcfg.js`，不会固化当前 881 个 ID。生产下载默认每主机 1 req/s，HTML 10 MiB，单附件 100 MiB，单政策附件合计 500 MiB。
