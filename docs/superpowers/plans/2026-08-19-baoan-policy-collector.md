# 宝安政策采集器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `plugins/baoan-policy-collector` 建成独立采集器，直接遍历官网 `zcfg.js` 全量记录，下载详情 JSON、原文 HTML 和附件，生成不可变、可校验、可增量维护的 Raw Package。

**Architecture:** 每次先下载 seed HTML，从 `<script src>` 重新发现并校验 `zcfg.js`，再用不执行远端代码的受限解析器提取 `allData`。按 ID 公式请求详情 JSON，以它作为结构化主数据源，以原文 HTML 和附件作为保真证据；SQLite 只保存运行状态，事实全部落入 Raw Package。本阶段不接入 FTMind 数据库、前端、模型或浏览器自动化。

**Tech Stack:** Go 1.26、`net/http`、`encoding/json`、`crypto/sha256`、`html-to-markdown/v2`、`modernc.org/sqlite`、`robfig/cron/v3`、JSON Schema、`testing`/`httptest`、Docker。

---

## 简化后的边界

实测协议已经消除分页和通用爬取的不确定性：seed HTML 引用 `/zcfg.js`；该文件当前包含 881 条唯一 ID；详情 URL 可按 `/postmeta/p/{id/1000000}/{id/1000}/{id}.json` 拼接；详情 JSON 已包含正文、官方元数据、附件和关系。因此首版删除通用蜘蛛、分页器、浏览器降级、FTMind 数据源表、MinIO、管理 API、Vue 页面和模型标签推理。未来 Wiki Transformer/Importer 读取本项目的 Raw Package，单独建设。

## 文件结构

```text
plugins/baoan-policy-collector/
├── cmd/baoan-policy-collector/main.go
├── internal/archive/package.go
├── internal/collector/collector.go
├── internal/config/config.go
├── internal/detail/detail.go
├── internal/discovery/index.go
├── internal/discovery/parser.go
├── internal/httpclient/client.go
├── internal/model/model.go
├── internal/normalize/normalize.go
├── internal/scheduler/scheduler.go
├── internal/state/store.go
├── protocol/request-catalog.json
├── schema/{manifest,structured,relations,run-manifest}.schema.json
├── testdata/{seed.html,zcfg.js,detail-12846556.json,policy-12846556.html}
├── Dockerfile
├── go.mod
└── README.md
```

每个 Go 实现文件都有同目录 `_test.go`。输出目录固定为：

```text
baoan-policy-data/
├── state/collector.db
├── dictionaries/snapshots/<dictionary-snapshot-id>/
├── policies/post_<id>/{latest.json,snapshots/<snapshot-id>/}
└── runs/<run-id>/
    ├── discovery/{seed.html,index-script.js,index-records.ndjson,ids.json}
    ├── run-manifest.json
    ├── policies.ndjson
    ├── failures.ndjson
    └── changes.ndjson
```

### Task 1: 独立模块、配置和数据契约

**Files:**
- Create: `plugins/baoan-policy-collector/go.mod`
- Create: `plugins/baoan-policy-collector/internal/config/config.go`
- Create: `plugins/baoan-policy-collector/internal/config/config_test.go`
- Create: `plugins/baoan-policy-collector/internal/model/model.go`
- Create: `plugins/baoan-policy-collector/testdata/seed.html`
- Create: `plugins/baoan-policy-collector/testdata/zcfg.js`
- Create: `plugins/baoan-policy-collector/testdata/detail-12846556.json`

- [ ] **Step 1: 初始化模块和依赖**

```powershell
cd plugins/baoan-policy-collector
go mod init github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector
go get github.com/JohannesKaufmann/html-to-markdown/v2 github.com/robfig/cron/v3 modernc.org/sqlite github.com/santhosh-tekuri/jsonschema/v6
```

- [ ] **Step 2: 写默认配置失败测试**

```go
func TestDefaultConfig(t *testing.T) {
	c := Default()
	if c.SeedURL != "https://www.baoan.gov.cn/xxgk/fgk/" { t.Fatalf("seed=%q", c.SeedURL) }
	if c.RequestInterval != time.Second || c.HTMLMaxBytes != 10<<20 || c.AttachmentMaxBytes != 100<<20 { t.Fatalf("limits=%+v", c) }
	if len(c.AllowedHosts) != 1 || c.AllowedHosts[0] != "www.baoan.gov.cn" { t.Fatalf("hosts=%v", c.AllowedHosts) }
}
```

- [ ] **Step 3: 运行失败测试**

Run: `go test ./internal/config -count=1`

Expected: FAIL，`Default` 未定义。

- [ ] **Step 4: 实现配置和模型**

```go
type Config struct {
	SeedURL, DataDir string
	AllowedHosts []string
	RequestInterval time.Duration
	HTMLMaxBytes, AttachmentMaxBytes int64
	RequestTimeout time.Duration
	RetryCount int
	IncrementalCron, FullCron string
}
func Default() Config { return Config{
	SeedURL: "https://www.baoan.gov.cn/xxgk/fgk/", DataDir: "./baoan-policy-data",
	AllowedHosts: []string{"www.baoan.gov.cn"}, RequestInterval: time.Second,
	HTMLMaxBytes: 10<<20, AttachmentMaxBytes: 100<<20, RequestTimeout: 30*time.Second,
	RetryCount: 3, IncrementalCron: "0 2 * * *", FullCron: "0 3 * * 0",
} }
```

`model.go` 定义 `IndexRecord`、`DetailRaw`、`Attachment`、`RelatedPost`、`StructuredPolicy`、`Manifest`、`RunManifest`、`Failure`、`FieldConflict`；Unix 时间保留 `int64` 原值，派生时间使用 RFC3339 字符串。

- [ ] **Step 5: 保存真实公开夹具并验证**

保存 seed 的 `/zcfg.js` 引用、至少三条真实索引记录和用户提供的完整详情 JSON；不得写入 Cookie。Run: `go test ./... -count=1`，Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add plugins/baoan-policy-collector
git commit -m "feat: scaffold Baoan policy collector"
```

### Task 2: 安全、有界、限速 HTTP 客户端

**Files:**
- Create: `plugins/baoan-policy-collector/internal/httpclient/client.go`
- Create: `plugins/baoan-policy-collector/internal/httpclient/client_test.go`

- [ ] **Step 1: 写安全边界失败测试**

```go
func TestClientRejectsUntrustedRedirect(t *testing.T) {
	private := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); defer private.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, private.URL, 302) })); defer public.Close()
	c := New(Options{AllowedHosts: []string{hostOf(public.URL)}, MaxBytes: 1024})
	_, err := c.Get(context.Background(), public.URL)
	if !errors.Is(err, ErrHostNotAllowed) { t.Fatalf("err=%v", err) }
}
func hostOf(raw string) string {
	u,err:=url.Parse(raw); if err!=nil { panic(err) }
	return u.Hostname()
}
```

同文件覆盖非 HTTP(S)、用户信息、回环/私网/链路本地、DNS 重绑定、10 次重定向、声明与流式大小超限、429、503、context 取消。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/httpclient -count=1`

Expected: FAIL，`New` 未定义。

- [ ] **Step 3: 实现响应和边界**

```go
type Response struct {
	URL string; StatusCode int; ContentType, ETag, LastModified string
	Body []byte; FetchedAt time.Time; SHA256 string
}
func digest(b []byte) string { s:=sha256.Sum256(b); return hex.EncodeToString(s[:]) }
```

发请求前和每次重定向后执行相同 URL/host/DNS/IP 校验；使用 `io.LimitReader(body,max+1)`，超限不返回部分正文。

- [ ] **Step 4: 实现限速和重试**

每主机默认 1 req/s、burst 1；只重试暂时网络错误、429、502、503、504。等待通过可注入 sleeper 测试，404/403/TLS/解析错误不重试。

- [ ] **Step 5: 验证并提交**

Run: `go test -race ./internal/httpclient -count=1`，Expected: PASS 且无 race。

```powershell
git add plugins/baoan-policy-collector/internal/httpclient
git commit -m "feat: add bounded policy source client"
```

### Task 3: 发现并安全解析 `zcfg.js`

**Files:**
- Create: `plugins/baoan-policy-collector/internal/discovery/index.go`
- Create: `plugins/baoan-policy-collector/internal/discovery/parser.go`
- Create: `plugins/baoan-policy-collector/internal/discovery/parser_test.go`
- Create: `plugins/baoan-policy-collector/protocol/request-catalog.json`

- [ ] **Step 1: 写发现和解析失败测试**

```go
func TestDiscoverIndexScript(t *testing.T) {
	h := []byte(`<script src="/zcfg.js" charset="utf-8"></script>`)
	u, err := DiscoverIndexScript("https://www.baoan.gov.cn/xxgk/fgk/", h)
	if err != nil || u != "https://www.baoan.gov.cn/zcfg.js" { t.Fatalf("url=%q err=%v", u, err) }
}
func TestParseAllDataWithoutExecution(t *testing.T) {
	s := []byte(`var allData=[{"id":"1","title":'含\'引号',"wjlx":["企业政策"]},];`)
	r, err := ParseAllData(s)
	if err != nil || len(r)!=1 || r[0].Title!="含'引号" { t.Fatalf("records=%+v err=%v", r, err) }
}
```

另测缺少赋值、括号未闭合、模板字符串、函数调用、未知标识符、空/重复 ID。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/discovery -count=1`

Expected: FAIL，函数未定义。

- [ ] **Step 3: 实现受限 JS 字面量解析**

```go
func ParseAllData(src []byte) ([]model.IndexRecord, error) {
	lit, err := extractAssignedArray(src, "allData"); if err != nil { return nil, err }
	b, err := normalizeJSLiteral(lit); if err != nil { return nil, err }
	var out []model.IndexRecord
	if err=json.Unmarshal(b,&out); err != nil { return nil, err }
	if err=validateUniqueIDs(out); err != nil { return nil, err }
	return out,nil
}
```

扫描器只接受对象/数组标点、数字、单双引号字符串、`true/false/null` 和尾随逗号；字符串重新用 `json.Marshal` 编码。拒绝函数、模板字符串和未知 token；禁止 `eval`，禁止正则解析嵌套对象。

- [ ] **Step 4: 实现 ID 账本比较**

```go
type IDDiff struct { Added, Changed, Missing []string }
```

每条记录规范 JSON 后计算哈希；新 ID→Added，同 ID 哈希改变→Changed，旧 ID消失→Missing；仅排序改变不算 Changed。

`request-catalog.json` 固化已验证的 seed、`zcfg.js`、详情公式、热门聚合和附件请求，逐项记录用途、host、响应格式、发现来源和验证日期；热门聚合标为补充对账而非全量真源。

- [ ] **Step 5: 验证并提交**

Run: `go test ./internal/discovery -count=1`，Expected: PASS。

```powershell
git add plugins/baoan-policy-collector/internal/discovery
git commit -m "feat: parse Baoan policy index safely"
```

### Task 4: 详情 URL 和分层 JSON 解析

**Files:**
- Create: `plugins/baoan-policy-collector/internal/detail/detail.go`
- Create: `plugins/baoan-policy-collector/internal/detail/detail_test.go`
- Modify: `plugins/baoan-policy-collector/internal/model/model.go`

- [ ] **Step 1: 写真实样本失败测试**

```go
func TestURLForID(t *testing.T) {
	got,err:=URLForID("https://www.baoan.gov.cn","12846556")
	want:="https://www.baoan.gov.cn/postmeta/p/12/12846/12846556.json"
	if err!=nil || got!=want { t.Fatalf("got=%q err=%v",got,err) }
}
func TestDecodeLayeredDetail(t *testing.T) {
	b,err:=os.ReadFile("../../testdata/detail-12846556.json"); if err!=nil { t.Fatal(err) }
	d,err:=Decode(b); if err!=nil { t.Fatal(err) }
	if d.ID!=12846556 || len(d.Attachments)!=4 || len(d.RelatedPosts)!=4 { t.Fatalf("detail=%+v",d) }
	if d.GKML.DocumentNumber!="深宝福规〔2026〕1号" { t.Fatalf("gkml=%+v",d.GKML) }
}
```

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/detail -count=1`

Expected: FAIL，函数未定义。

- [ ] **Step 3: 实现 ID 和两层 JSON 解码**

```go
type Decoded struct {
	Raw []byte; ID int64; Title, ContentHTML string
	Attachments []model.Attachment; RelatedPosts []model.RelatedPost
	Extension Extension; GKML GKML; Conflicts []model.FieldConflict
}
```

ID 只接受正十进制整数。`Decode` 保存原字节，解析顶层，再二次解析 `json_ext`、`gkml_data`；未知字段保存为 `map[string]json.RawMessage`。

- [ ] **Step 4: 对账重复字段和时间**

顶层与 `json_ext` 的 `EXT_*`/`wjlx`/`sbks`/`sbjs` 逐项比较，不一致写入 `FieldConflict`。Unix 秒保留原值并生成 Asia/Shanghai RFC3339；各类发布日期、生效期、有效期和申报期不得合并。

- [ ] **Step 5: 验证并提交**

Run: `go test ./internal/detail -count=1`，Expected: 样本 4 附件、4 关系、重复扩展字段无冲突。

```powershell
git add plugins/baoan-policy-collector/internal/detail plugins/baoan-policy-collector/internal/model/model.go
git commit -m "feat: decode layered policy details"
```

### Task 5: 正文、官网分类和关系证据

**Files:**
- Create: `plugins/baoan-policy-collector/internal/normalize/normalize.go`
- Create: `plugins/baoan-policy-collector/internal/normalize/normalize_test.go`

- [ ] **Step 1: 写正文与关系失败测试**

```go
func TestNormalizeSample(t *testing.T) {
	d:=loadSampleDetail(t)
	p,err:=Policy(d,time.Date(2026,8,19,0,0,0,0,time.FixedZone("CST",8*3600)))
	if err!=nil { t.Fatal(err) }
	if !strings.Contains(p.Markdown,"第一章") || !strings.Contains(p.Markdown,"第三十条") { t.Fatal("policy structure lost") }
	if p.Official.Theme!="公安、安全、司法" || p.Official.ServiceObjects[0]!="其他" { t.Fatalf("official=%+v",p.Official) }
}
func loadSampleDetail(t *testing.T) detail.Decoded {
	t.Helper()
	b,err:=os.ReadFile("../../testdata/detail-12846556.json"); if err!=nil { t.Fatal(err) }
	d,err:=detail.Decode(b); if err!=nil { t.Fatal(err) }
	return d
}
```

关系测试精确覆盖：2→`related_document`、3→`text_interpretation`、4→`graphic_interpretation`、5→`video_interpretation`；7 保留原 code，并按官网当前标题规则记录申报公告、操作规程或意见征集显示标签。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/normalize -count=1`

Expected: FAIL，`Policy` 未定义。

- [ ] **Step 3: 实现确定性 HTML→Markdown**

使用 `html-to-markdown/v2`，删除空段和重复空白但保留章、条、列表、表格。证据头包含 ID、标题、文号、机构、来源 URL、抓取时间和详情 SHA-256；不得生成模型摘要或模型标签。

- [ ] **Step 4: 实现官网事实和申报状态**

逐字保存 `source`、`EXT_fbjg`、`EXT_ztfl`、`EXT_wjlb`、`EXT_fglb`、`wjlx` 以及 GKML 分类代码/名称。申报状态同时记录官网是否列入、`sbks`、`sbjs`、`expired_time`、日期计算和冲突，不把官网脚本结论当成唯一事实。

同一次完整 run 聚合并发布官方字典快照：服务对象、发文机构、主题、文件载体、功能关系 code；每项保存官网原值、出现次数、来源响应 SHA-256 和首次/最后发现时间，未知值原样保留并令 run 至少为 partial。

- [ ] **Step 5: 验证并提交**

Run: `go test ./internal/normalize -count=1`，Expected: PASS，关系均携带原 code、标题、URL 和规则版本。

```powershell
git add plugins/baoan-policy-collector/internal/normalize
git commit -m "feat: normalize policy facts and relations"
```

### Task 6: 附件与不可变 Raw Package

**Files:**
- Create: `plugins/baoan-policy-collector/internal/archive/package.go`
- Create: `plugins/baoan-policy-collector/internal/archive/package_test.go`
- Create: `plugins/baoan-policy-collector/schema/manifest.schema.json`
- Create: `plugins/baoan-policy-collector/schema/structured.schema.json`
- Create: `plugins/baoan-policy-collector/schema/relations.schema.json`
- Create: `plugins/baoan-policy-collector/schema/run-manifest.schema.json`

- [ ] **Step 1: 写原子发布失败测试**

```go
func TestPublishIsImmutableAndIdempotent(t *testing.T) {
	root:=t.TempDir()
	pkg:=Package{ExternalID:"12846556",DetailRaw:[]byte(`{"id":12846556}`),SourceHTML:[]byte(`<p>正文</p>`),Markdown:"# 正文\n",Structured:json.RawMessage(`{"id":"12846556"}`),Relations:json.RawMessage(`[]`)}
	a,err:=Publish(root,pkg); if err!=nil { t.Fatal(err) }
	b,err:=Publish(root,pkg); if err!=nil { t.Fatal(err) }
	if a.SnapshotID!=b.SnapshotID || b.Created { t.Fatalf("a=%+v b=%+v",a,b) }
}
```

另测声明/实际大小或 MIME 不符、单附件 100 MiB、单政策附件合计 500 MiB、路径穿越、写入中断、checksum 不符、旧快照拒绝覆盖、`latest.json` 原子替换。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/archive -count=1`

Expected: FAIL，`Publish` 未定义。

- [ ] **Step 3: 实现包内容**

每个快照写 `manifest.json`、`source.html`、`source-detail.json`、`normalized.md`、`structured.json`、`relations.json`、`checksums.sha256`、`attachments/`。附件名为 `<sha256前12位>-<安全原名>`，manifest 同时记录声明/实际 MIME、大小和 SHA-256。

```go
type Package struct {
	ExternalID string
	DetailRaw, SourceHTML []byte
	Markdown string
	Structured, Relations json.RawMessage
	Attachments []model.DownloadedAttachment
}
type PublishResult struct { SnapshotID string; Created bool }
```

- [ ] **Step 4: 实现 staging 和原子发布**

先写 `.staging/<run>/<post>`，关闭句柄后校验 JSON Schema、相对路径和全部哈希，再 rename。相同快照哈希记 `unchanged`，不同内容新建快照，绝不修改旧目录。

- [ ] **Step 5: 验证并提交**

Run: `go test ./internal/archive -count=1`，Expected: PASS，故障注入不产生半包。

```powershell
git add plugins/baoan-policy-collector/internal/archive plugins/baoan-policy-collector/schema
git commit -m "feat: publish immutable policy packages"
```

### Task 7: SQLite 状态、全量/增量和重试

**Files:**
- Create: `plugins/baoan-policy-collector/internal/state/store.go`
- Create: `plugins/baoan-policy-collector/internal/state/store_test.go`
- Create: `plugins/baoan-policy-collector/internal/collector/collector.go`
- Create: `plugins/baoan-policy-collector/internal/collector/collector_test.go`

- [ ] **Step 1: 写 checkpoint/retry 失败测试**

```go
func TestFailedDetailRemainsRetryable(t *testing.T) {
	s:=openTestStore(t)
	if err:=s.RecordFailure(context.Background(),model.Failure{RunID:"r1",URL:"https://www.baoan.gov.cn/x",Stage:"detail"}); err!=nil { t.Fatal(err) }
	x,err:=s.ListRetryable(context.Background(),10)
	if err!=nil || len(x)!=1 || x[0].RunID!="r1" { t.Fatalf("items=%+v err=%v",x,err) }
}
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s,err:=Open(filepath.Join(t.TempDir(),"collector.db")); if err!=nil { t.Fatal(err) }
	t.Cleanup(func(){ _=s.Close() })
	return s
}
```

SQLite 只建 `runs`、`records`、`failures`、`locks`；正文事实不得只存在数据库。

- [ ] **Step 2: 写编排失败测试**

`httptest` 提供 seed、3 条索引、3 个详情和一个先503后成功的附件。首次 run 应 partial 且其余政策发布；retry 后收敛；第二次不变增量应 `created=0 updated=0 unchanged=3`。

- [ ] **Step 3: 运行失败测试**

Run: `go test ./internal/state ./internal/collector -count=1`

Expected: FAIL，store/collector 未定义。

- [ ] **Step 4: 实现状态机和顺序**

```go
const (
	RunDiscovering="discovering"; RunFetching="fetching"; RunPublishing="publishing"
	RunReconciling="reconciling"; RunSuccess="success"; RunPartial="partial"; RunFailed="failed"
)
```

固定流程：归档 seed/index → 解析 ID → 写 discovery 账本 → 详情 → 原文 HTML → 附件 → normalize → publish → run 账本 → 完整 run 才 missing 对账。单条失败不中止其他记录。

- [ ] **Step 5: 实现增量和三轮缺失**

详情、正文、附件清单或附件哈希变化均产生新快照。ETag/Last-Modified 仅优化传输。连续三次完整 full run 缺失才标记 `source_removed_candidate`，不删除历史包。

- [ ] **Step 6: 验证并提交**

Run: `go test -race ./internal/state ./internal/collector -count=1`，Expected: PASS 且无 race。

```powershell
git add plugins/baoan-policy-collector/internal/state plugins/baoan-policy-collector/internal/collector
git commit -m "feat: orchestrate incremental policy collection"
```

### Task 8: CLI、cron、Docker 和 README

**Files:**
- Create: `plugins/baoan-policy-collector/cmd/baoan-policy-collector/main.go`
- Create: `plugins/baoan-policy-collector/cmd/baoan-policy-collector/main_test.go`
- Create: `plugins/baoan-policy-collector/internal/scheduler/scheduler.go`
- Create: `plugins/baoan-policy-collector/Dockerfile`
- Create: `plugins/baoan-policy-collector/README.md`

- [ ] **Step 1: 写 CLI 失败测试**

对 `run(args,stdout,stderr)` 断言：无命令退出2；`collect --full`、`collect --incremental`、`retry --failed`、`verify --all`、`export-manifest --run r1`、`daemon` 可解析；成功0、运行失败1、配置错误2、partial 3。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./cmd/baoan-policy-collector ./internal/scheduler -count=1`

Expected: FAIL，`run`/scheduler 未定义。

- [ ] **Step 3: 实现命令和 JSON 摘要**

摘要字段固定为：`run_id/status/index_count/unique_ids/created/updated/unchanged/failed/attachments_declared/attachments_saved/data_dir`；日志不写正文、附件内容或完整 query。

- [ ] **Step 4: 实现调度和退出**

默认 Asia/Shanghai 每日02:00增量、周日03:00全量；进程锁阻止重叠。SIGTERM 停止新请求，完成当前文件并保存 checkpoint，60秒内退出。

- [ ] **Step 5: Docker 与 README**

多阶段构建静态二进制，非 root 运行，只挂载 `/data`，不开放端口。README 给出 Windows、Docker、全量、增量、重试、校验、备份和恢复命令，明确不依赖 FTMind 七容器。

- [ ] **Step 6: 验证并提交**

```powershell
go test ./... -count=1
go build ./cmd/baoan-policy-collector
docker build -t fmind/baoan-policy-collector:test .
git add plugins/baoan-policy-collector
git commit -m "feat: package policy collector CLI"
```

Expected: 测试、编译、镜像构建成功。

### Task 9: 协议验收、首轮全量和数据质量报告

**Files:**
- Create: `plugins/baoan-policy-collector/internal/collector/live_test.go`
- Create: `plugins/baoan-policy-collector/scripts/verify-run.ps1`
- Modify: `plugins/baoan-policy-collector/README.md`

- [ ] **Step 1: 默认端到端验收**

Run: `go test -race ./... -count=1`

Expected: PASS；默认测试只访问 `httptest`。

- [ ] **Step 2: 实现显式 live smoke test**

`live_test.go` 使用 `//go:build live`，只验证 seed 200、发现官方索引、ID 非空唯一、首中尾3个详情符合 Schema、附件 host 在白名单；不下载全部附件。

Run: `go test ./internal/collector -tags=live -run TestLiveProtocol -count=1`

Expected: PASS；失败只输出阶段和字段差异。

- [ ] **Step 3: 10条 dry-run**

```powershell
go run ./cmd/baoan-policy-collector collect --full --max-items 10 --data-dir ./test-output
go run ./cmd/baoan-policy-collector verify --all --data-dir ./test-output
```

Expected: 状态 `sampled`，10个包校验通过，不执行 missing 对账。

- [ ] **Step 4: 首轮完整采集与验证**

```powershell
go run ./cmd/baoan-policy-collector collect --full --data-dir ./baoan-policy-data
powershell -File scripts/verify-run.ps1 -DataDir ./baoan-policy-data
```

Expected: `index_count == unique_ids`，详情成功率≥98%；附件全部成功归档或进入带 URL/阶段/原因的失败账本；partial 不参与删除判断。

- [ ] **Step 5: 验证幂等增量**

Run: `go run ./cmd/baoan-policy-collector collect --incremental --data-dir ./baoan-policy-data`

Expected: 官网不变时 `created=0 updated=0`；新增 ID 自动进入 `added_ids` 和下载队列。

- [ ] **Step 6: 输出数据质量报告**

报告包含索引总数/唯一数、政策原文/政策解读数、机构/主题/载体/服务对象唯一值及次数、附件声明/成功/失败、关系 code、申报状态、未知官网值和字段冲突；只统计官网事实。

- [ ] **Step 7: 最终检查并提交**

```powershell
rg -n 'TBD|FIXME|以后补充|待实现' .
git diff --check
go vet ./...
go test ./... -count=1
git add plugins/baoan-policy-collector
git commit -m "test: verify Baoan policy raw collection"
```

Expected: 占位词扫描无输出，diff、vet、测试通过。

## 完成定义

- 每次从 seed HTML 发现 `zcfg.js`，不写死当前 881 个 ID。
- 解析不执行远端 JavaScript；协议漂移、空/重复 ID 令 run 变为 partial。
- 每条成功记录保存原始详情 JSON、HTML、Markdown、结构化字段、关系、附件和哈希。
- `json_ext`、`gkml_data` 二次解析，字段冲突不覆盖。
- 官网标签逐字保真；首版不调用模型。
- 全量、增量、失败重试、断点续跑和三轮缺失均有测试。
- 快照不可变，可用 SHA-256 和 JSON Schema 独立校验。
- Windows、Linux、Docker 可独立运行，不依赖 FTMind、数据库服务或浏览器。
- 本阶段不直接导入 Wiki；Raw Package 稳定后再制定 Wiki Schema/Importer 计划。

## 实施审计（2026-08-19）

- [x] 独立 Go 模块、受限 HTTP、`zcfg.js` 安全解析、详情/正文/附件采集已实现。
- [x] 官网事实字段、动态申报状态、官方关系 code、原始 URL 和原始响应已落盘。
- [x] 每次索引采集生成带源 SHA-256 的官方维度字典快照（服务对象/机构/主题/载体）。
- [x] JSON Schema 已通过 `schema/embed.go` 嵌入并在发布与校验阶段实际执行；SHA-256 校验同步执行。
- [x] SQLite 运行状态、失败队列、锁、增量统计、三次缺失确认和定向 retry 已实现。
- [x] CLI、Asia/Shanghai scheduler、SIGTERM 停止、Dockerfile、校验脚本和质量报告已实现。
- [x] 本地端到端采集、增量 unchanged、2 条 dry-run、官网 seed/index/首中尾详情 live smoke 已验证。
- [ ] Windows 当前没有 gcc，因此 `go test -race` 无法在本机执行；普通测试、`go vet` 和构建已通过。
- [ ] Docker 构建所需 Go 依赖下载受本机 TLS/代理证书错误影响；Dockerfile 本身已准备，需在网络证书正常的 Linux/服务器环境构建。
