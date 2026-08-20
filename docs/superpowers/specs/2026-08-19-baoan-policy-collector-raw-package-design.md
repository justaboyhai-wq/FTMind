# 宝安政策采集器与 Raw Package v1 设计

## 1. 目标

在 FTMind 仓库中建设一个可脱离 FTMind 运行的独立插件项目，定时从宝安区政策法规官网发现并下载政策原文、原始网页、附件、官网分类、功能标签和显式关系，生成不可变、可校验、可重复转换的本地 Raw Package。

Raw Package 是官网事实的长期档案，不直接等同于某一版政策 Wiki。后续政策问答、申报政策、领导讲话材料等 Wiki Schema 均以 Raw Package 为输入生成，Wiki 规则变化时不需要重新抓取官网。

## 2. 范围

### 2.1 本阶段包含

- 独立 Go 模块、CLI、配置、调度器和 Docker 镜像。
- 从 `https://www.baoan.gov.cn/xxgk/fgk/` 及其政策目录完整发现政策原文和政策解读。
- 下载政策详情原始 HTML、正文内公开附件以及显式关联页面。
- 完整记录官网五个分类入口及其层级、编码、排序和数量。
- 完整记录政策页面上的解读、意见征集、政策咨询、申报等业务入口。
- 解析文号、机构、日期、体裁、官网标签、申报时间等确定性字段。
- 根据官网当前可申报信号、申报开始/截止时间和当前时间计算申报状态。
- 全量、增量、失败重试、断点续跑、完整性对账、校验和导出。
- 生成不可变 Raw Package v1 和批次账本。

### 2.2 本阶段不包含

- 不直接创建或更新 FTMind Wiki 页面。
- 不调用大模型生成标签、摘要或关系。
- 不把模型推断写入 Raw Package。
- 不删除已归档的历史政策或附件。
- 不绕过验证码、登录、站点安全限制或 robots 规则。
- 不新增 PostgreSQL、Redis、MinIO 等基础设施依赖。

## 3. 项目边界

项目目录固定为：

```text
plugins/baoan-policy-collector/
├── cmd/baoan-policy-collector/
├── internal/
│   ├── archive/
│   ├── classify/
│   ├── config/
│   ├── crawl/
│   ├── discover/
│   ├── extract/
│   ├── httpclient/
│   ├── protocol/
│   ├── reconcile/
│   ├── scheduler/
│   ├── state/
│   └── verify/
├── schema/
├── testdata/
├── Dockerfile
├── go.mod
└── README.md
```

该模块不得导入 FTMind 根模块的 application/service 或数据库实现。它只通过 Raw Package 文件契约与下游通信。允许复制极小的安全 URL 校验逻辑，但应在插件内部拥有独立测试，避免插件升级受 FTMind 主服务编译状态影响。

## 4. 总体数据流

```text
官网分类页与列表页
        ↓
入口 HTML 静态分析 + 公开请求协议目录
        ↓
分类字典快照 + URL 全量发现
        ↓
政策详情和关联入口解析
        ↓
原始 HTML、附件、关联页面下载
        ↓
确定性字段提取 + 动态申报状态计算
        ↓
临时 Raw Package 构建
        ↓
Schema、哈希和引用完整性校验
        ↓
原子发布不可变快照 + 更新 latest.json
        ↓
未来的 Policy Wiki Transformer / FTMind Importer
```

采集和 Wiki 生成必须是两条独立流水线。采集器只回答“官网当时发布了什么”；Wiki Transformer 才回答“如何面向具体应用组织这些事实”。

## 4.1 入口 HTML 与公开请求协议发现

采集器不能把浏览器逐项点击当成生产抓取方式。对每个 seed URL，必须先下载完整静态 HTML，建立页面请求目录，再使用已验证的公开 HTTP 请求直接完成分页、筛选、详情和附件采集。

静态分析范围：

- 页面内全部 `<a href>`、`<form action>`、`<script src>`、`<link href>`、`<iframe src>`、图片和附件 URL。
- inline script 中的公开配置、栏目 ID、分页参数、接口 base URL、JSON 数据和请求模板。
- 外部 JavaScript 中可静态识别的 `fetch`、XHR、axios、JSONP 和分页 URL 模板。
- HTML meta、canonical、data-* 属性以及页面源码中出现的栏目/分类 code。
- 首次协议核对时浏览器加载该页面实际产生的公开网络请求，包括方法、路径、query/body 参数、响应 Content-Type、分页字段和响应含义。

输出 `protocol/request-catalog.json`，每条请求定义至少包含：

- `request_id`：稳定标识。
- `discovered_from`：HTML 元素、inline script、external script 或 browser trace。
- `method`、`url_template`、允许的 host。
- query/body 参数名、类型、必填性、示例值和语义。
- 响应格式、字符集、分页字段、记录 ID 字段和错误信号。
- 用途：分类字典、政策列表、详情、解读、附件或咨询。
- 验证时间、来源文件 SHA-256 和协议版本。

请求目录中的“新请求”必须逐项解释含义，不能因为页面发起了请求就全部跟随。统计、广告、埋点、字体、图片、地图、分享和无关推荐请求标为 `ignored` 并记录理由；只有能够产生政策事实或发现政策 URL 的请求进入采集白名单。

生产执行顺序：

1. 直接 GET seed HTML 并计算结构指纹。
2. 使用当前协议目录解析静态链接和公开数据请求。
3. 直接发送分类、分页、详情和附件 HTTP 请求，不模拟点击。
4. 对照静态页面链接与请求响应记录，合并去重并做数量守恒。
5. HTML 结构指纹或请求协议发生变化时暂停使用不匹配的请求模板，将 run 标为 partial，并生成协议差异报告。

浏览器网络跟踪只用于以下情况：首次建立协议目录、HTML/JavaScript 无法静态解释请求参数、网站改版导致协议指纹失配。跟踪完成后必须把请求语义固化成版本化协议目录；日常定时任务不得依赖浏览器、坐标点击或 DOM 点击循环。

## 4.2 2026-08-19 官网请求协议实测基线

对 `https://www.baoan.gov.cn/xxgk/fgk/` 的静态脚本、浏览器已加载资源及公开响应进行了交叉核对。以下内容作为首版 `protocol/request-catalog.json` 的实测基线，而不是长期写死且无需校验的假设：

1. 全量发现入口为 `GET https://www.baoan.gov.cn/zcfg.js`。本次响应为 UTF-8 JavaScript，大小约 1.01 MB，声明 `var allData = [...]`，其中恰好包含 881 条带 `id` 的政策/解读索引记录。采集器必须归档原始脚本、计算 SHA-256，再用受限解析器提取数组；禁止直接执行远端 JavaScript。
2. 详情元数据 URL 由文章 ID 确定性生成：`/postmeta/p/{floor(id/1000000)}/{floor(id/1000)}/{id}.json`。例如 `12846556` 对应 `https://www.baoan.gov.cn/postmeta/p/12/12846/12846556.json`。页面当前展示的 7 条可申报记录分别触发 `12846556`、`12793671`、`12702507`、`12298294`、`12181232`、`11953225`、`11208928` 的详情请求。
3. `/postmeta/hot/187595.json`、`/postmeta/hot/187596.json`、`/postmeta/hot/146015.json`、`/postmeta/hot/110300.json` 返回 `articles` 聚合，用于页面“热门政策”展示，每个请求只取前 10 条。它们可作为补充发现和对账信号，但不得作为 881 条全量索引或详情真源。
4. `zcfg.js` 索引已观察到的业务字段包括：`id`、`title`、`subtitle`、`content`、`abstract`、`source`、`date`、`filterDate`、`url`、`wh`、`zt`、`tc`、`channel`、`zxdh`、`wjlx`、`sbks`、`sbjs`、`expired_time`。其中部分记录的 `content` 为空，页面搜索时才按需加载 `/zcfgCon.js`；因此正文真源仍是详情 HTML/详情 JSON，而不是假定索引永远带全文。
5. 详情 JSON 已观察到 `attachment`、`related_posts`、`gkml_data`、`url`、`gkml_url`、`EXT_wh`、`EXT_wjlb`、`EXT_fbjg`、`EXT_ztfl`、`wjlx`、`sbks`、`sbjs`、`zxdz`、`zxdh` 等字段。`gkml_data` 是 JSON 字符串，须二次解析并原样归档，其字段包含有效期、文号、发布机构、关键词、主分类、体裁分类和主题分类等。
6. 附件对象至少包含 `id`、`name`、`type`、`mime`、`size`、`url`；关联对象至少包含 `id`、`title`、`type`、`rank`、`related_classify`、`url`、状态和多种发布时间。关联关系的业务类型不能只凭标题猜测，应优先结合 `related_classify`、目标 URL 栏目、页面显示文案和目标详情元数据确定；无法确定时保留原始关联并标记 unknown。
7. 样本 `12846556` 的详情响应包含 4 个附件和 4 条显式关联，覆盖政策 PDF、3 个 Word 表格、上位政策、历史版本、文字解读和意见征集，证明详情 JSON 可以作为附件与关系发现的主入口，详情 HTML用于正文保真和交叉校验。

当前页面脚本将“当前可申报”候选条件实现为：`sbks` 与 `sbjs` 均非空，且 `expired_time` 尚未到期或为空；UI 同时用 `sbjs` 显示申报截止日期与剩余天数。由于这个实现没有直接校验当前时间是否落在 `[sbks, sbjs]` 内，采集器不得把网站脚本判断当作唯一状态结论。Raw Package 必须分别保存：是否被官网列入、`sbks`、`sbjs`、`expired_time`、网站脚本规则版本、本地透明规则结果以及两者冲突。

协议健康检查至少验证：`zcfg.js` 可解析、索引记录数非零、ID 唯一、随机详情样本符合 Schema、详情路径公式成立、附件 URL 位于白名单、`gkml_data` 可二次解析。若入口由 JavaScript 数组变成其他格式、记录数相对上次完整批次异常下降、字段集合漂移或路径公式失效，本批次必须标记 partial 并停止据此做“官网已删除”判断。

静态入口 HTML 已确认包含 `<script src="/zcfg.js" type="text/javascript" charset="utf-8"></script>`，其后的 inline script 直接执行 `allData = allData.sort(sortData)`。因此 881 个 ID 并不直接内嵌在入口 HTML 正文中，而是位于入口 HTML 明确声明的初始化资源 `zcfg.js` 中。采集器每次运行必须从 seed HTML 重新发现并校验该脚本引用，再下载当次脚本提取 ID，不能把当前 881 个 ID 固化进程序。首版实测提取 881 次 ID、去重后仍为 881，重复数为 0；对索引首部、尾部以及当前可申报样本按详情路径公式抽查均返回 200。

每次 run 应额外归档 `discovery/seed.html`、`discovery/index-script.js`、`discovery/index-records.ndjson` 和 `discovery/ids.json`。`ids.json` 至少保存索引 URL、响应 SHA-256、提取时间、记录总数、排序前 ID 列表、唯一 ID 列表以及相对上一完整 run 的 `added_ids`、`changed_ids`、`missing_ids`。新增政策由 `added_ids` 自动进入详情和附件队列；`missing_ids` 只进入缺失观察，不直接删除历史资料。

## 5. 官网事实、计算结果与未来模型数据的边界

### 5.1 官网事实 `official`

以下维度存在于官网时，值必须逐字保存，模型或本地规则不得补写同一维度的值：

- `application_listing`：是否出现在官网“当前可申报政策”入口。
- `service_object`：企业政策、个人政策、其他及其官网二级分类。
- `issuing_authority`：官网当期发文机构。
- `theme`：官网当期主题分类。
- `carrier_type`：区规范性文件、部门规范性文件、其他文件、政策解读。
- `features`：文字解读、图文解读、意见征集、政策咨询等页面实际出现的入口。

“全部”属于查询节点，不赋给政策作为标签。官网新增但尚未认识的分类或功能标签必须原样进入 `unknown_official_values`，同时令本批次为 partial，不能丢弃或自动归入“其他”。

### 5.2 确定性计算 `computed`

不依赖模型、可通过输入事实重复计算的数据可进入 Raw Package：

- URL 规范化结果。
- 文件 SHA-256、大小和 MIME sniff 结果。
- 申报状态与剩余天数。
- 官方分类和记录之间的确定性关联。
- 页面显式链接产生的附件、解读、意见征集、咨询和申报关系。

每个计算值必须携带算法版本、计算时间和输入证据引用。

### 5.3 未来模型扩展 `derived_ai`

政策工具、支持方式、适用行业、申报条件等官网没有的新维度可在未来 Wiki Transformer 中由模型生成。它们不属于 Raw Package v1；模型不得修改 `official`，只能创建独立的 `derived_ai` 命名空间，并保留证据、模型和提示词版本。

## 6. 官网分类字典

每个 full run 必须生成一份官网分类字典快照，一级维度固定为：

1. 当前可申报政策。
2. 按服务对象分类。
3. 按发文机构分类。
4. 按主题分类。
5. 按文件载体分类。

已确认的服务对象层级：

```text
企业政策
├── 全部（查询节点）
├── 产业政策
├── 空间政策
├── 监督管理
└── 其他

个人政策
├── 全部（查询节点）
├── 教育
├── 医疗
├── 住房
└── 其他

其他
```

文件载体首版精确集合：

- 区规范性文件。
- 部门规范性文件。
- 其他文件。
- 政策解读。

发文机构和主题不在设计文档中手工猜测全集。采集器必须展开官网当期完整列表，记录 `source_code`、`source_label`、`parent_code`、`source_order`、官网显示数量、首次/最后发现时间和来源响应哈希。批次验收要求官网报告数量、捕获数量与去重后字典数量一致。

## 7. 功能标签与显式关系

详情页业务入口必须记录原始文案和目标，不得只保存一个归一化 tag：

| 分组 | 已知原始文案示例 | 规范 code | 关系 |
|---|---|---|---|
| interpretation | 文字解读 | `text_interpretation` | `interprets` |
| interpretation | 图文解读 | `graphic_interpretation` | `interprets` |
| interpretation | 视频解读、数字人解读 | 对应稳定 code | `interprets` |
| participation | 意见征集 | `opinion_solicitation` | `solicits_opinion_for` |
| participation | 征集结果、意见反馈 | `solicitation_result` | `reports_solicitation_for` |
| consultation | 政策咨询、我要咨询政策 | `policy_consultation` | 无目标政策时保存联系方式 |
| application | 可申报项目、申报指南 | `application_guide` | `has_application_guide` |
| utility | 下载、打印 | 对应 utility code | 不作为 Wiki 业务标签 |

完整标签数不预设。全量采集后必须输出原始标签唯一数、规范 code 唯一数、业务标签数、utility 数、unknown 数、出现政策数、出现总次数和样例 URL。`unknown` 原样归档并告警，映射更新后允许从旧 Raw 快照重新生成新索引，但不得改写旧快照。

## 8. 动态申报状态

申报状态综合使用官网入口和日期事实，统一时区为 `Asia/Shanghai`。

输入：

- 本批次是否出现在官网当前可申报列表。
- 申报开始时间。
- 申报截止时间。
- 正文中的常态化、全年、长期受理等确定性表达。
- 本批次抓取时间。

输出枚举：

- `not_started`
- `open`
- `closing_soon`
- `closed`
- `rolling`
- `official_open_date_unknown`
- `unknown`

核定顺序：

1. 官网列入且日期处于申报期，输出 `open` 或 `closing_soon`，依据为 `official_and_date`。
2. 官网列入但日期未知，输出 `official_open_date_unknown`，依据为 `official_listing`。
3. 官网列入但日期显示截止，仍输出 `open`，同时产生 `official_date_conflict` 数据质量问题。
4. 官网未列入但可靠日期处于申报期，输出 `open` 或 `closing_soon`，依据为 `date_calculation`。
5. 官网未列入且日期已截止，输出 `closed`。
6. 未列入且日期未知，输出 `unknown`，不能推断为已截止。

`closing_soon` 默认阈值为 7 个自然日，可配置。每个快照保存状态及依据；当前时间变化只生成新的派生状态记录，不修改历史 Raw 快照。

## 9. Raw Package v1 目录契约

数据根目录：

```text
baoan-policy-data/
├── state/
│   ├── collector.db
│   └── lock.json
├── dictionaries/
│   └── snapshots/<dictionary-snapshot-id>/
│       ├── official-taxonomy.json
│       ├── feature-labels.json
│       └── checksums.sha256
├── policies/
│   └── post_<id>/
│       ├── latest.json
│       └── snapshots/<snapshot-id>/
│           ├── manifest.json
│           ├── source.html
│           ├── source-detail.json
│           ├── normalized.md
│           ├── structured.json
│           ├── relations.json
│           ├── checksums.sha256
│           └── attachments/
│               └── <sha256-prefix>-<safe-original-name>
└── runs/
    └── <run-id>/
        ├── discovery/
        │   ├── seed.html
        │   ├── index-script.js
        │   ├── index-records.ndjson
        │   └── ids.json
        ├── run-manifest.json
        ├── policies.ndjson
        ├── failures.ndjson
        ├── changes.ndjson
        └── checksums.sha256
```

`snapshot-id` 使用 `{UTC时间}-{snapshot_sha256前12位}`。快照目录发布后只读；`latest.json` 是一个小型指针文件，避免 Windows 符号链接兼容问题。重复抓取且快照哈希不变时不创建新目录，只更新 run 账本中的 `unchanged`。

## 10. Raw 文件职责

### 10.1 `manifest.json`

保存包身份和证据：

- `schema_version=baoan.raw/v1`
- `package_id`、`external_id`、`snapshot_id`
- canonical/final/discovered-from URL
- HTTP 状态、ETag、Last-Modified、Content-Type、响应时间
- 抓取器版本、抓取批次、抓取时间
- 所有包内文件的相对路径、大小、SHA-256、MIME
- parser/normalizer/application-status 算法版本
- 数据质量问题和失败引用

### 10.2 `source.html`

保存详情页下载后的原始响应正文。除 HTTP content-encoding 的正常传输解码外不得修改字节；原始 charset 和响应头摘要写入 manifest。

### 10.3 `normalized.md`

保存确定性清洗后的可读正文，包括来源证据头、政策正文、附件清单和显式关联入口。不得写入模型摘要或模型标签。

### 10.4 `structured.json`

保存：

- 标题、文号、机构、发布日期、成文日期、生效/失效日期。
- 官网五维分类 assignment，保留 source code、原始名称、父子路径和证据。
- 当前可申报官网信号、申报窗口、计算状态和核定依据。
- 官网功能标签及联系方式。
- 未识别的官网字段和值。

### 10.5 `relations.json`

只保存确定性关系：附件、显式解读、意见征集/反馈、政策咨询、申报指南、网页中明确链接的修订/废止/引用。只有正文相似或标题相似但无明确证据的关系不进入 Raw v1。

### 10.6 详情 JSON 的分层解析

详情 `/postmeta/p/.../<id>.json` 是单篇政策的主结构化数据源，但必须同时归档未经改写的原始响应，例如保存为 `source-detail.json`。解析分为四层：

1. 顶层发布字段：`id`、标题、摘要、`content`、来源、关键词、URL、发布时间、更新时间等。`content` 是正文 HTML，可确定性转换为 `normalized.md`，但原始 HTML 字符串必须保留。
2. 顶层扩展字段：`EXT_*`、`zxdz`、`zxdh`、`wjlx`、`sbks`、`sbjs`，用于文号、机构、主题、文件类别、咨询方式、服务对象和申报窗口。
3. `json_ext`：其值是 JSON 字符串，二次解析后与顶层扩展字段逐字段对账。两层值一致时保留共同事实和双重证据；不一致时不得静默覆盖，应保存双方原值并产生 `duplicate_field_conflict`。
4. `gkml_data`：其值也是 JSON 字符串，二次解析后提供政府信息公开标识、有效性、有效期、文号、发布机构以及 `classify_main/genre/theme` 的代码和名称。原始字符串与解析对象均须保留。

所有 Unix 时间戳按“秒”保存原值，并另行生成 `Asia/Shanghai` ISO-8601 表示；不得只保留格式化日期。`first_publish_time`、`display_publish_time`、`publish_time`、`create_time`、`updated`、成文日期、生效日期、有效期、申报起止时间语义不同，不得合并成单一 `date`。

`attachment[]` 直接进入附件下载队列，同时核对声明的 `id/name/type/mime/size/url`、实际响应 MIME、实际字节数和 SHA-256。`related_posts[]` 原样进入关系证据层；官网当前脚本把 `related_classify=2` 显示为“相关文件”、`3` 显示为“文字解读”、`4` 显示为“图文解读”、`5` 显示为“视频解读”，而 `7` 还会根据标题显示为申报公告、操作规程或意见征集等。由于 `7` 的细分包含官网前端标题规则，Raw 中必须同时保存数值代码、标题、目标 URL、官网显示标签和规则版本，不能只保存归一化关系。

## 11. 状态库与不可变档案

`state/collector.db` 是可变的运行状态，不属于档案：

- URL 发现状态和分页游标。
- ETag、Last-Modified 和最近成功哈希。
- 失败队列、重试次数和下次重试时间。
- 完整 run 的 seen set。
- 调度锁和上次成功时间。

SQLite 丢失后允许通过现有 Raw Package 重建基础状态；因此 state 不能保存 Raw 中没有的唯一业务事实。

快照发布过程：

1. 写入数据根目录内的 `.staging/<run-id>/<package-id>`。
2. 关闭所有文件句柄。
3. 校验 JSON Schema、相对路径、文件大小和 SHA-256。
4. 确认所有 manifest 引用都位于包目录内。
5. 原子 rename 到最终 snapshot 目录。
6. 原子替换 `latest.json`。
7. 记录 run 账本和 SQLite 成功状态。

任一步失败时保留失败记录，清理该包的 staging 目录，不发布半套快照。

## 12. CLI 与调度

```text
baoan-policy-collector collect --full
baoan-policy-collector collect --incremental
baoan-policy-collector retry --failed
baoan-policy-collector verify --all
baoan-policy-collector export-manifest --run <run-id>
baoan-policy-collector daemon
```

`daemon` 读取标准 5 段 cron 表达式，默认每天 Asia/Shanghai 02:00 执行增量、每周日 03:00 执行 full reconciliation。单实例文件锁避免重叠执行；进程收到 SIGTERM/Windows service stop 时停止发起新请求，完成当前文件写入并保存 checkpoint。

Docker 仅运行同一二进制并挂载 `/data` 和只读配置，不引入额外服务。原生 Windows 运行和 Docker 运行必须生成完全相同的 Raw Package 路径与 JSON 内容。

## 13. 全量、增量与对账

### 13.1 全量

- 下载 seed HTML，输出静态资源/请求统计和版本化 request catalog 差异。
- 只执行 request catalog 中标为 policy-data 的公开请求，跳过 ignored 请求。
- 抓取五维官网字典。
- 遍历全部列表分页和分类入口。
- 合并 canonical URL 和 `post_<id>` 去重。
- 抓取详情、功能入口和附件。
- 与上次完整 run 比较 seen set。
- 输出新增、变化、不变、缺失、失败和字典变化。

### 13.2 增量

- HTTP 缓存头只作为优化，内容哈希是变化判据。
- 失败 URL 始终进入 retry set，不能推进其成功水位。
- 正文、官网标签、功能入口、附件清单或附件哈希变化均生成新快照。
- 单页失败不阻止其他页面归档，但 run 标为 partial。

### 13.3 缺失

只有完整成功的 full run 才参与缺失判断。连续三次完整 full run 未发现同一政策后，将状态记为 `source_removed_candidate`，不删除本地快照。后续重新出现时恢复 active 并记录变化事件。

## 14. 网络和资源安全

- 默认每个主机 1 并发、1 请求/秒，遵守 Retry-After。
- 只允许 HTTPS/HTTP 官方白名单主机。
- 每次请求和重定向后阻止回环、私网、链路本地和 DNS 重绑定。
- TLS 校验必须开启。
- HTML 默认上限 10 MiB，单附件默认 100 MiB，单政策附件合计默认 500 MiB。
- 超限文件记录 URL、声明大小和失败原因，不写部分文件。
- 文件名清洗后使用哈希前缀，禁止路径穿越和绝对路径。
- 日志不记录正文、附件内容、Cookie 或完整 query 参数。

## 15. 配置

配置文件只包含非秘密参数：数据目录、seed URL、官方主机白名单、请求间隔、大小上限、cron、时区、重试次数、closing-soon 天数和是否启用浏览器降级。

浏览器降级默认关闭。启用时只允许生成一次网络协议观察结果，不允许作为长期逐页点击采集器；观察到的新 host 和请求模板必须经过白名单与语义审核后才能进入下次生产 run。

官网当前无需认证。若未来增加认证，秘密只能由环境变量或挂载 secret 提供，禁止写入配置样例、Raw Package 和日志。

## 16. 完整性账本

每次 run 输出：

- 每个官网分类维度的 reported/captured/unique 数量。
- 发现详情数、成功详情数和解析失败数。
- 声明附件数、成功附件数、超限数和失败数。
- 原始功能标签数、规范 code 数和 unknown 数。
- 新增、变化、不变、缺失候选数。
- 官网当前可申报数、日期计算可申报数、两者交集和冲突数。
- 各失败原因及 URL 引用。

样本上限模式不执行缺失对账，也不能标为完整成功。

## 17. 测试策略

- 单元测试：URL、安全客户端、分页、DOM/JSON 解析、日期、标签映射、状态机、哈希、路径和 Schema。
- 协议测试：静态 HTML/JavaScript 请求枚举、请求用途分类、参数类型、分页终止、响应 Schema、协议指纹变化和 ignored 请求隔离。
- 夹具契约测试：真实公开网页的脱敏/原始夹具，覆盖五维分类、原文、文字/图文解读、咨询和附件。
- 端到端测试：本地 `httptest` 模拟分页、重定向、429、503、附件变化、字典变化和三轮缺失。
- 故障注入：写盘失败、磁盘空间不足、进程中断、SQLite 锁、损坏 staging 和哈希不一致。
- 幂等测试：相同 full run 不产生新快照；单字段变化只产生一个新快照。
- 跨平台测试：Windows 本地路径与 Linux Docker 挂载目录生成一致的相对路径和规范 JSON。

测试不得依赖生产官网实时可用性；真实官网 smoke test 单独运行，不进入默认 CI。

## 18. 完成标准

- 独立模块可以在没有 FTMind app、PostgreSQL、Redis 和 MinIO 的环境运行。
- 可一次性完整下载政策原文、原始 HTML、附件和显式关联页面。
- 能对入口 HTML 和加载协议生成完整请求目录，说明每个数据请求的用途，并通过直接 HTTP 请求完成生产采集，不依赖逐项点击。
- 可按计划自动增量运行，失败可重试，重启可续跑。
- Raw Package 全部通过 JSON Schema、路径和 SHA-256 校验。
- 五个官网分类入口 reported/captured 数量一致。
- 全量详情页观察到的业务功能标签 unknown 数为 0；新标签能够保真归档并告警。
- 官网分类值不被本地规则或模型补写。
- 动态申报状态同时保留官网信号、时间输入、计算结果和冲突。
- 历史快照不可变，Wiki Schema 升级能够仅依靠本地 Raw Package 重放。
- 采集器不改变现有 FTMind 七容器，也不占用新的公网端口。

## 19. 后续接口

下一阶段单独建设 `Policy Wiki Transformer`：读取 `baoan.raw/v1`，按照版本化 Wiki Schema 生成 Wiki 页面、标签、目录和关系。该阶段可以引入模型创建官网没有的新维度，但必须写入 `derived_ai`，不能覆盖 Raw Package 中的 `official` 和 `computed`。
