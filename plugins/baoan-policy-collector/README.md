# Baoan Policy Collector

独立下载宝安区政策法规库的 `zcfg.js`、详情 JSON、原文 HTML、附件和官网显式关系，输出不可变 Raw Package。采集器不依赖 FMind、PostgreSQL、Redis、MinIO 或浏览器。

```powershell
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector retry --failed --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector verify --all --data-dir ./baoan-policy-data
go run ./cmd/baoan-policy-collector daemon --data-dir ./baoan-policy-data
```

完整 run 会从入口 HTML 重新发现 `/zcfg.js`，不会固化当前 881 个 ID。生产下载默认每主机 1 req/s，HTML 10 MiB，单附件 100 MiB，单政策附件合计 500 MiB。
