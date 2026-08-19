# Baoan Policy Collector

独立下载宝安区政策法规库的 `zcfg.js`、详情 JSON、原文 HTML、附件和官网显式关系，输出不可变 Raw Package。采集器不依赖 FMind、PostgreSQL、Redis、MinIO 或浏览器。

```powershell
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector retry --failed --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector verify --all --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector serve-rss --data-dir ./baoan-policy-data --addr :18320 --base-url https://collector.example.com
```

The collector is an independent Go module. It discovers `zcfg.js` from the seed
HTML on every run, downloads detail JSON, original HTML, attachments, and
official relationship metadata, then writes immutable packages under
`<data-dir>/policies`. Each run also writes discovery/event ledgers and an
official-dimension dictionary snapshot under `<data-dir>/dictionaries`.
It does not require the FMind services.

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
folder that FMind's document uploader imports recursively as one transaction;
uploading every JSON sidecar would create noisy independent documents. For a
one-time document import, export one Markdown document plus its structured
sidecar per policy:

```powershell
.\scripts\export-raw.ps1 -DataDir .\baoan-policy-data -OutputDir .\raw-export
```

For ongoing synchronization, run `serve-rss` against the persistent data
directory. Configure FMind's RSS/Atom data source with
`http://<host>:18320/feed.xml` (or the public reverse-proxy URL) and set its
normal sync schedule. Each RSS item links to a readable policy page generated
from `normalized.md`; the source JSON, relation graph, manifest, and
attachments remain in the raw package for audit and later Wiki transformation.
The gateway is read-only and reflects each package's `latest.json`, so a later
incremental collector run automatically changes the next RSS sync.

完整 run 会从入口 HTML 重新发现 `/zcfg.js`，不会固化当前 881 个 ID。生产下载默认每主机 1 req/s，HTML 10 MiB，单附件 100 MiB，单政策附件合计 500 MiB。
