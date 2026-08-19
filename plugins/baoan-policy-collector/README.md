# Baoan Policy Collector

独立下载宝安区政策法规库的 `zcfg.js`、详情 JSON、原文 HTML、附件和官网显式关系，输出不可变 Raw Package。采集器不依赖 FMind、PostgreSQL、Redis、MinIO 或浏览器。

```powershell
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector retry --failed --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector verify --all --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data
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

完整 run 会从入口 HTML 重新发现 `/zcfg.js`，不会固化当前 881 个 ID。生产下载默认每主机 1 req/s，HTML 10 MiB，单附件 100 MiB，单政策附件合计 500 MiB。
