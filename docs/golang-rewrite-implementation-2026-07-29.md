# MusicParty Go 后端兼容重写实施台账

日期：2026-07-29；最后更新：2026-08-01

状态：实施中。本文取代 `golang-rewrite-estimation-2026-07-26.md` 作为执行台账；旧文档仅保留为历史估算。

## 不可变决策

- 在同一仓库的 `backend-go/` 中实现单进程、单 SQLite、单部署单元的 Go 后端。
- 前端不增加 Java/Go 兼容分支；REST、WebSocket、Cookie、CSRF、环境变量和 SQLite 文件均以冻结 Java 行为为准。
- 不引入微服务、gRPC、GraphQL、PostgreSQL、ORM 或生产双写。
- Go 在数据库副本上做差分验证，最终在 5–15 分钟维护窗口切换。
- 首次切换及回滚期不执行 schema migration；Java 保留 30 天或两个稳定版本。
- Docker/VPS 能力在范围内；已退役的产品能力不属于迁移范围，也不保留实现或契约。
- 普通播放、媒体代理、缓存、本地上传和转码均在范围内。
- 首发外部平台硬门槛为 Netease 和 Bilibili；本地曲库同时属于首发核心门槛。YouTube、Navidrome、Squidify 和动态 Subsonic 仅在管理员配置启用时验收，详见 `go-rewrite-launch-platform-scope-2026-08-01.md`。

## 当前协调边界

用户已确认 Spring Boot 后端修补稳定。阶段 1、阶段 2 与阶段 3 已完成，但继续保持以下边界：

- 基线提交 `95b9f8da32e2375b7183e9d6233d8c45d0d1dfcd` 和注释标签 `go-rewrite-baseline-v1` 已推送到 `origin/NRT-Base`；Java/前端基线内容自标签后保持不变。
- 后续提交 `9079233686cda893c2eafaf25d70c471aa8fe4e1` 仅将 Surefire 临时目录从干净 checkout 中不存在的 `target/tmp` 改为必然存在的 `target`；不改变 Java 业务行为或契约。对应 GitHub Quality 的前端和 181 个 Java 测试均通过。
- HTTP、WebSocket、配置与数据库契约已生成并可执行检查；契约必须以该标签为生成来源。
- 当前不修改、不暂存、不提交 Java 或前端文件。
- Go、契约、验收证据和阶段文档将在功能完成后按阶段整理提交；运行时数据库和本地工具目录不得混入这些提交。

## 已落地的 Go 基础设施

代码位于 `backend-go/`，当前包括：

- Go `1.26.0` module；本机工具链为 Go `1.26.5 windows/amd64`。
- Chi `v5.3.1`、coder/websocket `v1.8.15`、modernc SQLite `v1.55.0`。
- Prometheus client `v1.24.1`、x/crypto `v0.54.0`、x/sync `v0.22.0`。
- testify `v1.11.1`、go-cmp `v0.7.0`、goleak `v1.3.0`。
- 显式构造与标准库 `slog`，未使用 Gin、Viper、Wire、Fx、ORM 或 service locator。
- 当前环境变量和默认值的 typed 配置加载、解析与校验。
- `8080` 默认端口、HTTP 超时、信号处理、readiness 降级和优雅关停。
- `/actuator/health`、liveness、readiness、info、metrics 和 Prometheus 端点。
- 请求 ID、结构化访问日志、HTTP 指标，以及与当前 Java 一致的媒体 CORS、可信代理、Cookie 与 CSRF 基础。
- server/production 启动安全校验、统一错误映射、WebSocket 同源/白名单 Origin 校验。
- bcrypt 密码原语、WebSocket 信封和握手选项、有界并发辅助。
- CGO-free SQLite 打开、WAL、foreign keys、busy timeout、integrity/foreign-key 检查器。
- SQLite 单 writer 连接、默认两个只读 `query_only` 连接、容量 100 的串行事务写队列。
- 写事务 commit 后才返回；失败、取消和关闭会 rollback，队列满和关闭具有明确错误。
- 从空库运行当前 Java 初始化器生成的 `contracts/db/schema.sql`、`schema.json` 和 `schema.sha256`。
- 当前权威快照包含 23 张应用表、45 个显式 schema 对象，并记录列、外键、隐式索引和 partial index。
- schema 兼容检查按表、列、类型、NOT NULL、主键和显式索引比较；旧迁移库只豁免 SQLite `ALTER TABLE` 无法补加的历史外键元数据，不以原始建表 SQL hash 直接拒绝。
- typed repository 已覆盖冻结 schema 的全部 23 张表，包括 room、账号/身份/会话、成员/邀请、queue、history、playback、chat、房间与用户播放列表、设置、Subsonic 和本地媒体。
- `VACUUM INTO` 一致性副本工具、数据库启动接入，以及 fresh/legacy/current-copy 三类 Java→Go→Java→Go SQLite 往返均已落地。
- `contracts/http/openapi.yaml` 与 source-linked route inventory 当前覆盖 93 个 HTTP operation，包括 83 个应用映射及 Actuator、静态首页和 SPA fallback。
- WebSocket JSON Schema 当前记录 26 个 canonical 入站类型、全部斜杠别名和 19 个实际发现的出站类型；消息顺序、关闭码、字段和数组顺序均为严格契约。
- `contracts/config/environment.yaml` 从 Java `application.yml` 冻结 103 个环境变量及默认值和源码位置。
- 隔离 Java 黑盒启动器使用随机端口、临时 SQLite、本地目录和固定管理员；golden 比较器只归一化明确的时间、随机 ID、凭据和 Cookie 值。
- 非 root Dockerfile，运行时保留 FFmpeg、Python 和 yt-dlp。
- 正式 `.github/workflows/go-backend.yml`：Windows/Linux 验证、race、静态检查、Go 漏洞扫描、Docker build 和 Trivy。

阶段 4 至阶段 7 的平台、业务、房间 actor、WebSocket Hub、媒体代理和静态前端服务已经实现。移除 Radio 与 Tauri 后的最终运行时候选 `96727c13434567152bed889798f1e4803e30b5cf` 已完成阶段 8 系统验收：契约、质量门、首发真实平台、新浏览器流程、负载、故障矩阵、同硬件比较和运行时镜像扫描均有证据。阶段 9 的离线切换准备已开始，干净 Dockerfile 重建已经恢复并通过；生产数据库副本交接验证和实际切换仍是后续门禁，当前没有触碰生产环境。

阶段 4 当前落地内容：

- `/api/config`、`/api/platforms`、搜索、用户播放列表、播放列表歌曲、专辑、用户搜索、歌词和封面色端点。
- Netease、Bilibili WBI/收藏夹、YouTube Data API、Navidrome/Subsonic 和本地媒体查询实现。
- 长生命周期 HTTP client、连接/响应头/整体超时、取消传播、瞬态重试、`Retry-After`、响应体上限和无效 JSON 拒绝。
- 数据库优先的 Netease/Bilibili secret、Java AES-GCM 格式的 Subsonic credential 解密、房间绑定动态 source 和 session-token allowed-users 校验。
- fixture 测试覆盖平台映射、WBI 签名、Subsonic token auth、超时/取消、重试、无效响应和冻结 HTTP 状态。

未将 fixture 结果推断为真实上游验收。最终阶段 8 候选已经使用有效运行时凭据完成 Netease、Bilibili 和本地曲库的规定流程；YouTube、Navidrome、Squidify 和动态 Subsonic 只在该部署明确启用时加入门禁，本次目标部署未将它们加入首发门禁。

阶段 5 当前落地内容：

- 管理员 bootstrap、密码登录、Session/Cookie/CSRF、注销、资料和密码修改，以及账号状态接口。
- 一次性房间邀请 metadata 与原子兑换；invite consume、账号、profile、membership 和 session 在同一 writer 事务提交。
- 房间列表、重命名、删除、邀请、成员和 Owner 管理；保护最后一个 Owner，删除房间时迁移在线身份并清理持久化数据。
- 用户/房间播放列表 CRUD、tracks、重排、导入/导出和 liked songs；批量添加与导入使用单事务，避免部分写入。
- 公开房间播放列表要求有效 Session；私有房间额外验证 Java 兼容的 HMAC `roomAccessToken`、过期时间和 password version。
- Subsonic source 保存、排序、删除、测试与运行时注册；credential 使用 Java `enc:v1:` AES-GCM 格式，列表接口不返回密码。
- Navidrome allowed-users 修改即时刷新并持久化，进程重启时优先从 SQLite 恢复。

阶段 5 的播放列表 `/enqueue` 已接入阶段 6 `RoomRuntime`；Passkey、管理员 elevation 和离线恢复在当前 Java 基线中仍未完成，Go 不宣称已经提供这些安全保证。

阶段 6 当前落地内容：

- 每个活跃房间由一个 `RoomRuntime` goroutine 独占队列和播放状态；HTTP/WS 写操作进入同一个容量 100、默认等待 5 秒的命令队列，跨房间可并行。
- 首次播放和切歌将 queue、playback state 与可选 history 在单个 SQLite writer transaction 中提交；提交失败不会更新 actor 内存或发布 ACK/广播。
- 实现 queue version、mutation ID 去重、index/queue-id 两种 reorder、patch/state 广播、动态 progress、暂停位置冻结、自动切歌和房间空闲驱逐。
- WebSocket Hub 每连接一个写循环和容量 256 的发送队列；可靠消息 FIFO，五类状态消息 latest-only 合并，队列满时关闭慢客户端。
- `/ws?room-id=...` 保持 Cookie/query Session 鉴权、1008、点号/斜杠别名、初始 state/identity/presence/rooms、resync、ping/pong、queue ACK/NACK、chat/history、user bind/rename 和 room create/delete。
- 私有房间创建写入 Java 兼容 bcrypt hash/password version；guest mutation、pause/skip/shuffle lock 和 seek 点播者权限在 dispatcher/runtime 中执行。
- `enqueue` 通过统一 `PlayableResolver` 从 Netease、Bilibili、YouTube、Subsonic 或本地 repository 解析权威名称、艺术家和时长，不再信任客户端元信息；解析结果使用 5 分钟、最多 2048 项的有界缓存。
- 本地测试覆盖同房间并发写、mutation 去重、进度/自动切歌/暂停、空闲驱逐、Hub FIFO/latest-only/慢客户端、真实 WebSocket 鉴权/alias/enqueue/resync，以及 fuzz 和 goleak。

阶段 6 的 Java/Go WebSocket golden、原前端浏览器流程、300 连接、30 分钟状态收敛和真实元信息入队已在阶段 8 通过。生产替换仍取决于阶段 9 的生产数据库副本交接、切换审批和维护窗口，而不是由本地测试自动推断。

阶段 7 当前落地内容：

- 本地上传使用请求体大小上限、文件名清理、SHA-256 重复检测和受根目录约束的路径；源文件先落盘，再将 typed 状态写入冻结 SQLite schema。
- FFmpeg 转码使用有界 worker/queue、`CommandContext`、30 分钟上限和完整进程树终止；输出先写 `.part`，成功后原子重命名。
- 完成转码后使用 ffprobe 获取时长，并尝试提取内嵌封面；失败不会把不完整媒体标记为完成。
- `/api/local` 的管理、上传权限、媒体和封面端点已接入；媒体读取验证 Session，拒绝路径穿越，并支持单 Range、`206`、`416`、取消传播和 CORS 暴露头。
- Netease、Bilibili、YouTube/yt-dlp、Navidrome 和房间 Subsonic stream/cover proxy 已接入，保留 Range、Content-Type、Content-Length、Content-Range 和 Accept-Ranges。
- 播放状态按平台生成与前端兼容的代理 URL；真实 Netease 流已验证 `206 audio/mpeg`、浏览器音频加载、暂停、切歌和 seek 状态同步。
- 下载缓存采用有界提交队列、平台共享并发上限、瞬态重试、`.part` 原子提升、重启后按确定文件名重新发现和 LRU 空间回收。
阶段 7 的有效故障矩阵覆盖 Range/后缀 Range/416、路径穿越、上游取消与状态映射、缓存原子写/LRU/重试/去重/队列满/关闭清理、本地上传重复检测/大小限制、转码完成与删除、转码超时清理、SQLite BUSY 和异常事务恢复。本机真实 FFmpeg 测试生成短 WAV、经正式 Transcoder 编码为 OGG/Opus，并验证 `OggS` 和 `.part` 清理。破坏性的真实磁盘满注入仍保留为环境验收项。

本轮本机二进制冒烟确认 FFmpeg 8.1.2 可执行 libopus/OGG 编码，ffprobe 可读取 lavfi 输入。隔离运行时镜像包含 yt-dlp 2026.7.4；最终候选已经保存 Netease 完整 CDN 音频、Bilibili 搜索/收藏夹/音频和本地曲库上传转码的真实证据。YouTube、Navidrome 和 Subsonic 未在目标部署启用，因此不属于本次首发门禁。

## 当前验证证据

在 `backend-go/` 中已通过：

```text
gofmt
go mod tidy
go test ./...
go vet ./...
staticcheck v0.7.0 ./...
govulncheck v1.6.0 ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...
```

`go test ./...` 包含真实子进程集成测试：构建服务、随机端口启动、等待探针、检查 readiness/Prometheus/metrics，并结束进程。

阶段 1 Java 黑盒已连续执行一次 capture 和一次全新临时库 compare，结果一致：

```text
42 个 HTTP 场景，覆盖基础/邀请及用户播放列表 CRUD、重排、导入失败、导出、喜欢歌曲和空列表入队边界
11 个 WebSocket 场景，覆盖 1008、resync、ping、identity、presence、queue NACK、Unicode chat/public chat、history、reconnect
HTTP status、字段缺失、数组顺序、错误体、Cookie 属性、选定 header 和 WS 顺序均严格比较
```

媒体与服务补充矩阵执行 35 个定向 Java 测试，0 failure、0 error、0 skipped，覆盖本地上传/待转码/重复检测、Range/proxy/fallback、播放列表和 queue ACK/NACK。真实上游冒烟与完整二进制媒体压力矩阵仍属于阶段 4/7/8，而不是由阶段 1 的 fixture 结果推断。

真实进程冒烟结果：

```text
GET /actuator/health             200, status=UP
GET /actuator/health/readiness   200, status=UP
GET /actuator/prometheus         200, 包含 Go collector
GET /actuator/metrics            200
GET /missing                     404
```

Go `dbcheck` 对本轮数据库副本的最终结果均为：

```json
{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":23}
```

实际完成的往返矩阵为：

1. Java 从空库初始化出的 23 表数据库。
2. `SqliteSchemaInitializerTests` 产生、由旧 schema 升级的数据库；其 schema 快照保存在 `contracts/db/fixtures/legacy-upgraded/`。
3. `music_party/data/musicparty.db` 的只读一致性副本：原副本为 20 表，由 Java 初始化器升级到 23 表后，再由 Go 全 repository 写入、Java repository 读取并写回，最后由 Go 重新读取。

三类数据库最终均通过 `integrity_check`、foreign-key check 和业务内容检查。原始 `music_party/data/musicparty.db` 未被本轮 Go 测试写入；可能含真实数据的临时副本已删除且未进入版本库。

canonical fresh schema hash 为 `9e9eddae8c30016fc19ac0300124498473734bfe0e0d3845780158a28cacbf8b`；legacy-upgraded fixture hash 为 `ba921627b6ff03eab9bfd53f0999d6fbb743127f31ab250af5224c47703eecb6`。hash 用于识别快照，不替代语义兼容检查。

SQLite 专项测试还覆盖单 writer 下 50 个并发写入无丢失更新、BUSY、rollback、取消、队列满、关闭、panic rollback，以及进程在未提交 WAL 事务中被强制终止后的恢复。

Linux race 已通过：

```text
golang:1.26.5-bookworm
go test -race -count=1 ./...
全部通过
```

Docker 验证已通过：

```text
Linux/amd64 image build 成功
只读根文件系统启动成功
cap-drop=ALL
no-new-privileges
uid=10001(appuser), gid=10001(appgroup)
health/readiness=UP
FFmpeg 8.0.1
Python 3.12.13
yt-dlp 2026.07.04
Trivy Critical: 0
```

阶段 8 追加证据：

- Netease 实际歌曲 `28816031` 解析为 `Cling Cling`、`Perfume`、257693 ms，代理 Range 返回 `206 audio/mpeg`，浏览器 `readyState=4` 且 seek 状态从 30000 ms 收敛到 30001 ms。
- 无 Cookie 上游只提供约 30 秒试听，因此未把完整 4:17 seek 当作已验证；试听片段内真实 seek 已通过。
- 两个浏览器标签在后端快速重启后恢复 track 与音频 ready state；约 1000 首真实元信息队列可滚动到底，500/1000 阶段 ACK P95 分别为 18.81/36.15 ms。
- 候选 `96727c1` 使用一次性授权运行时 Cookie 重新验证 Netease 完整音频源，Range 返回 `bytes 0-65535/10310052`；验收脚本新增 1,000,000 字节最小源大小门槛，会拒绝约 481 KB 的试听片段。Bilibili 真实搜索返回 10 条、收藏夹返回 14 个非空集合，音频 Range 返回 `bytes 0-65535/17713611`，入队和 seek 均通过。凭据、账号和收藏夹名称未写入证据，一次性容器已删除。
- 本地曲库使用临时 4 秒 WAV 完成上传、FFmpeg OGG 转码、搜索、入队、Range、seek、删除和引用清理；删除后媒体返回 410，临时音频和数据库未进入 Git。
- 候选 `96727c1` 的 10 连接 + 20 房间 + 1000 队列、100 连接和 300 连接三组均运行 30 分钟且 stderr 为空。1000 队列 ACK P95 为 13.37 ms、最大 20.29 ms、状态收敛为 23.85 ms；100/300 连接关闭后 goroutine 均从 218/618 回落到 18。20 个保留房间使第一组恢复后 goroutine 为 39，符合房间 actor 生命周期设计。
- 候选 `96727c1` 完成了新的可视化浏览器验收：管理员登录、进入 Lounge、WebSocket 在线状态、真实 Bilibili 当前播放、Netease 搜索渲染和入队后的队列更新均通过。登录前 401、Wake Lock 权限拒绝、单次封面提色 400、浏览器主动中止媒体请求及无 handler 的 `player.progress` debug 日志均按非阻塞现象记录；媒体字节正确性由独立 Range/seek 验收覆盖。
- 有效媒体故障矩阵覆盖 Range、取消、Retry-After、缓存/LRU/去重/队列满、SQLite BUSY/异常回滚、FFmpeg 超时、`.part` 清理及路径限制。
- 同硬件比较中，Java/Go 启动分别为 7073.81/348.43 ms，RSS 为 252723200/20766720 bytes，HTTP P95 为 25.03/2.83 ms，300 WS pong P95 为 374.39/148.21 ms；两端均建立全部连接且 0 error。
- 候选镜像 `sha256:bbb8eb103050995e2f85157464bbef406eec8a1a206d52c59d01a18ca0ea002b` 已从干净 Dockerfile 构建，并以 UID/GID 10001、只读 rootfs、cap-drop ALL、no-new-privileges 启动；健康检查为 `UP`，静态首页返回 200，Trivy CRITICAL=0。
- 阶段 9 准备期间 Docker Hub 访问恢复，`backend-go/Dockerfile` 已从头成功构建 `musicparty-go:stage9-prep`；正式镜像包含非 root 主进程以及 `/app/dbsnapshot`、`/app/dbcheck` 运维二进制，Compose 合并配置验证通过。该本地标签不是已发布的生产候选 digest。
- 阶段 9 本机隔离候选 `669b6ef` 已通过新版 `dbcheck`、`cutover.sh preflight/snapshot/verify`、Go→Java→Go repository 往返、Go `DB_INIT_SCHEMA=false` 只读 rootfs 启动和 Java 容器回滚启动。演练同时确认 Go/Java 必须共用 Java 容器实测 UID/GID；Go Compose 和 preflight 已强制这一条件。证据清单位于 `backend-go/acceptance/stage9/final-candidate-669b6ef/manifest.json`，本机 registry digest 不是远端发布 digest，未触碰 VPS 或生产数据库。
- 候选更新 Axios、PostCSS 和 Vite 后，显式使用官方 npm registry 执行的 `npm audit` 与 `pnpm audit` 均报告 0 漏洞。本机默认 npmmirror 不实现 audit API，因此正式证据记录了带 `--registry=https://registry.npmjs.org` 的可复现命令。

`govulncheck` 报告调用路径漏洞为 0；扫描同时识别出 1 个 required module 中的不可达漏洞，但 MusicParty 导入的包和调用路径均不受影响。

## 阶段状态

| 阶段 | 状态 | 退出条件 |
| --- | --- | --- |
| 0 Java 基线 | 已冻结 | 基线提交和注释标签已推送，成为共享 Java 兼容基线 |
| 1 可执行契约 | 已完成 | 93 个 HTTP operation、WS 双向 schema、103 个环境变量、SQLite contract、隔离 Java capture/compare 和补充媒体测试证据均已落地 |
| 2 Go 基础设施 | 已完成 | Windows 测试、静态检查、Linux race/交叉构建、Docker/Trivy 和正式 CI 均有证据 |
| 3 SQLite 兼容 | 已完成 | 23 表 repository、单写者/只读连接模型、fresh/legacy/current-copy 往返、异常中断恢复、integrity 和 foreign-key 检查均已通过 |
| 4 无状态 HTTP/平台 | 已完成首发验收 | fixture 差分全部通过；最终候选的凭据化 Netease、Bilibili 和本地曲库真实流程通过。条件平台未在目标部署启用，不加入本次门禁 |
| 5 账号/房间/播放列表 | 已实现，系统差分通过 | HTTP/持久化和实时 enqueue 已接入，冻结 HTTP/WS golden 通过 |
| 6 实时核心 | 已完成本地验收 | actor、权威元信息、原子持久化、Hub/背压、全部冻结命令、race/集成、浏览器重连及 10/100/300 WS 30 分钟矩阵通过 |
| 7 缓存/下载/流媒体 | 已完成首发验收 | 代理 URL、Netease/Bilibili 真实 Range/音频/seek、本地上传转码、缓存和有效故障矩阵通过；真实磁盘满仍是非阻塞环境注入项 |
| 8 系统验收 | 已完成 | 候选 `96727c1` 的 Java 176 tests、Go 全质量门、前端 30 files/97 tests/lint/build、npm/pnpm 0 漏洞、首发三类真实媒体、新浏览器流程、1000 队列、10/100/300 WS 30 分钟、媒体故障矩阵、同硬件比较、干净 Dockerfile 重建、只读非 root 运行和 Trivy 均通过 |
| 9 生产切换 | 本机候选演练完成，未触碰生产 | 本地不可变 digest、发布质量门、冻结 schema 校验、停机快照、共享 UID/GID、Go 启动和 Java 回滚均已演练；仍等待 VPS 数据库隔离副本交接、正式远端 digest 和维护窗口审批 |
| 10 Java 退役 | 未开始 | Go 稳定 30 天或两个版本后执行 |

## 恢复 Java 基线时的顺序

1. 确认 Spring Boot 修补 Agent 已结束，记录最终 HEAD 和完整工作树归属。
2. 重跑 Maven、前端测试/构建、lint、Node 脚本语法检查。
3. 使用临时 SQLite、固定管理员和隔离端口运行认证 WebSocket 10/100/300 连接矩阵。
4. 保存启动时间、RSS、HTTP P95、WebSocket ACK/Pong P95，而不是引用本轮临时测量。
5. 对脱敏数据库副本执行 integrity、foreign-key、文件大小和 SHA-256 检查。
6. 只暂存明确归属且已验证的文件，排除运行时数据库、Agent 工具目录和临时锁文件。
7. 推送现有基线提交和 `go-rewrite-baseline-v1` 标签（已完成）。
8. 从该标签重新验证 `contracts/`，之后所有 Java 严重修复必须同步更新契约。

## Definition of Done

完成标准仍是批准计划中的完整条件：原前端无分支使用 Go、全部冻结契约通过、SQLite 可原地切换和回滚、范围内业务与媒体能力可用、race/泄漏/安全/负载/浏览器验收通过、生产稳定期达标，并最终退役 Java 构建链。代码存在本身不构成完成证据。
