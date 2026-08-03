# Go 生产切换包

## ACR 镜像同步

`.github/workflows/go-backend-release.yml` 在 GHCR 发布同一份已扫描的 Go 候选时，也会推送到 `crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party`。工作流摘要会输出 `ACR_MUSIC_PARTY_IMAGE`，该值必须是带 `@sha256:` 的不可变引用。

将该值写入 VPS 专用 `compose.go.env` 的 `MUSIC_PARTY_IMAGE`，然后按本文件既有维护窗口和 schema bridge 流程操作。不得使用 ACR `latest` 或 `java-legacy-*` 标签，它们不是 Go 后端，可能与 Go 所需的数据库形状不兼容。

本目录只准备阶段 9，不会自动连接或修改生产环境。首次切换继续遵守：5–15 分钟维护窗口、单 SQLite、无 schema migration、Java 可立即回滚。

## 一次性 21→23 schema bridge

若生产 Java 数据库仍为 21 个应用表，先在维护窗口执行一次 bridge；Go 首次启动绝不承担 schema migration。bridge 只创建快照、离线运行固定 Java initializer 并验证结果，不会启动、停止、恢复任何 Compose 服务，也不会更改 Cookie。

三道确认缺一不可：

1. 三个镜像 ref 都是不可变 digest，且 Go 和 Java bridge 必须是下列固定候选。
2. 操作人进入已公告维护窗口，并显式设置 MUSICPARTY_MAINTENANCE_CONFIRMED=YES。
3. 操作人手动停止唯一 Java 写入者，脚本再次确认 music-party-app 明确处于 stopped/exited 状态。

    export MUSIC_PARTY_IMAGE='ghcr.io/bigblackblob/musicparty-go@sha256:f16d4a26b94d28ca42e47c0d1bee8043f5f2434d2a982f375290ef49c390e759'
    export MUSIC_PARTY_JAVA_BRIDGE_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:1f08392fa699bcb655e3fd2a6eb2347432378804412b2e5521f955e48cf299e2'
    export MUSIC_PARTY_ORIGINAL_JAVA_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:<recorded-current-java-digest>'

    docker pull "$MUSIC_PARTY_IMAGE"
    docker pull "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE"
    docker pull "$MUSIC_PARTY_ORIGINAL_JAVA_IMAGE"

    # 在 Java 仍运行时记录真实文件身份，不能猜测。
    export MUSIC_PARTY_RUNTIME_UID="$(docker exec music-party-app id -u)"
    export MUSIC_PARTY_RUNTIME_GID="$(docker exec music-party-app id -g)"
    sh backend-go/deploy/schema-bridge.sh preflight

    # 仅由操作人停止唯一写入者，bridge 本身不会触碰 Compose 生命周期。
    docker compose stop music-party
    export MUSICPARTY_MAINTENANCE_CONFIRMED=YES
    sh backend-go/deploy/schema-bridge.sh apply

    # 对 apply 输出的显式快照路径复查 Go dbcheck 和 SHA-256。
    sh backend-go/deploy/schema-bridge.sh verify ./music_party/backups/go-cutover/musicparty-post-bridge-pre-go-<timestamp>.db

preflight 只接受准确的历史形状：21 个应用表，缺失的只能是 room_membership 与 room_invite。migration ledger 不按数量放行，所有记录都必须已完成，且 key 必须精确等于下列 allowlist，不能有 room_membership / room_invite 的 target key 或任何额外 key：

    legacy.queue-data.json
    legacy.rooms.json
    schema.admin_bootstrap_claim.table
    schema.local_track.original_hash_unique
    schema.local_track.product_fields
    schema.local_track.table
    schema.local_upload_access.table
    schema.room_history_track.table
    schema.room_playback_state.like_markers_json
    schema.room_playback_state.liked_user_ids_json
    schema.room_subsonic_source.table
    schema.site_setting.table
    schema.subsonic_source.owner_room_id
    schema.subsonic_source.table
    schema.user_account.table
    schema.user_binding.table
    schema.user_playlist.system_key
    schema.user_playlist.table
    schema.user_playlist_track.table
    schema.user_profile.current_room_id

apply 使用 Java 镜像内的 Python SQLite backup API 创建不可覆盖的 pre-bridge 和 post-bridge/pre-Go 一致快照，因此停 Java 前留下的 WAL/SHM 不会被当成裸 musicparty.db 复制。Java 写入前，pre-bridge 快照还必须由不可变 Go `/app/dbcheck` 以退出码 1 报告唯一已知的不兼容状态：integrity 为 `["ok"]`、无外键违规、21 个应用表、schemaCompatible=false，且 schemaDifferences 仅为 `table.room_invite` 与 `table.room_membership` 的 `missing required table`。Java initializer 后 ledger 必须是上述 allowlist 加上两个 target key，以及已批准的 `schema.user_account.platform_admin_role` ADMIN→PLATFORM_ADMIN 兼容 migration，共 23 个完成 key。该兼容 migration 不放宽对既有 COOKIE、SESSDATA 或 user session 行的前后指纹保护。post-bridge 快照必须由同一不可变 dbcheck 精确报告 23 个应用表、schemaCompatible=true 和空 schemaDifferences。

SQLite 的 WAL SHM 协调有一个窄例外：对仍在 data 目录中的 live 数据库，桥接的 Python 读取任务会把目录 bind 为可写，供 SQLite 协调 SHM；连接仍为 mode=ro，并立即设置 PRAGMA query_only=ON，绝不执行数据库写入。因此不会修改 DB/WAL 内容、COOKIE 或 session 行。已经由 backup API 关闭并一致化的 pre/post 快照始终以只读 bind 和 mode=ro&immutable=1 打开。

临时 Java task container 使用记录的 Java UID/GID、--network none、只读根文件系统和临时 /tmp。唯一例外是 Java initializer 的私有 /tmp tmpfs 带有 exec，以便 SQLite JDBC 解压其原生 `.so`；这一权限只存在于离线、cap-drop、no-new-privileges 的 Java task，Python SQLite 与 Go dbcheck task 的 /tmp 始终为 noexec。该 bridge 仍使用普通应用 Java 镜像的 bridge mode，而非专用 migration 镜像，这是已记录的残余风险，故固定 digest、断网、最小权限和完整快照 gate 均不可省略。脚本私下确认 Java 的 SQLite schema initialized 完成标记及两个 migration ledger 条目后才优雅停止它；若完成标记前容器退出会立即报安全的通用错误，且不输出原始日志或环境内容。

Windows 的 Git-Bash 本地 rehearsal 也受支持：脚本会将 bind source 解析为 C:/... 形式，并仅在 MINGW/MSYS 下为 Docker 调用设置 MSYS_NO_PATHCONV=1，避免 Docker CLI 改写容器内的 /bin/sh、/app 或 /data 路径。VPS 生产操作仍是 Linux，保持原有 pwd -P 和 Docker 调用行为。

Python SQLite task 的源码经容器 stdin 传入，不作为 Docker 参数传递，避免 Git-Bash 与 Docker Desktop 改写 SQL 中的引号。

bridge 会在前后比较已有 netease.cookie、bilibili.sessdata 设置行和已有 user session。比较材料及内部指纹仅短暂存在于进程内，输出只有安全行数和 unchanged=true，绝不显示 Cookie、SESSDATA、session、密码哈希、行内容或内部指纹。Netease 或 Bilibili Cookie 到期不是 bridge 期间改变存储值的理由；应在 bridge 之外按正常设置流程处理。

bridge 完成后仍按下面的正常切换流程运行 cutover.sh。`schema-bridge.sh verify <snapshot>` 只接受上述精确的 pre-bridge 或 post-bridge 语义状态，并输出 `snapshot-kind=pre-bridge|post-bridge`；若 Go 验证失败，先停止 Go 写入者，运行 schema-bridge.sh verify <pre-bridge-snapshot>，再按回滚步骤恢复 Java。bridge 不会自动恢复数据库或启动 Java。所有 task 使用已在本机检查的固定 digest 并传入 `--pull=never`。原始数据库、WAL/SHM、快照、原始命令日志都不是仓库证据，绝不能提交；仓库只保留不含原始数据的声明式摘要。

## 硬门槛

开始维护窗口前必须全部满足：

- 阶段 8 最终候选证据清单已通过，且清单中的 `candidateSha` 是发布 ref 的祖先；若发布 ref 只增加证据和文档提交，工作流会验证这一边界。
- `.github/workflows/go-backend-release.yml` 从计划切换的提交发布成功，记录 `ghcr.io/...@sha256:...`，禁止使用移动标签。
- VPS 上已拉取并检查该 digest，现有 Java digest、配置文件路径、数据目录属主和剩余磁盘空间已记录。
- 已从仍在运行的 Java 容器记录数字 UID/GID。Go Compose 会继续使用这组身份，避免 SQLite 文件在 Go 与 Java 间切换时因属主不同变成只读。
- 维护公告、操作人、观察人、回滚决策人和窗口开始/结束时间已确定。
- 不改动 `.env` 中的凭据；切换日志不得输出其内容。
- `dbcheck` 必须同时报告 `integrity=["ok"]`、空的外键违规、`applicationTables=23` 和 `schemaCompatible=true`；只有完整冻结 schema 才能继续。

## 维护窗口

以下命令都在仓库根目录执行。它们不会替用户决定何时停生产服务。

```sh
export MUSIC_PARTY_IMAGE='ghcr.io/OWNER/musicparty-go@sha256:...'
docker pull "$MUSIC_PARTY_IMAGE"

# 在停止 Java 前记录它实际使用的文件身份。当前镜像通常是 100:101，
# 但切换时必须以 VPS 实测值为准，不要手填猜测值。
export MUSIC_PARTY_RUNTIME_UID="$(docker exec music-party-app id -u)"
export MUSIC_PARTY_RUNTIME_GID="$(docker exec music-party-app id -g)"
sh backend-go/deploy/cutover.sh preflight

# 进入公告过的维护窗口后停止唯一写入者。
docker compose stop music-party
export MUSICPARTY_MAINTENANCE_CONFIRMED=YES
sh backend-go/deploy/cutover.sh snapshot

docker compose -f docker-compose.yml -f backend-go/deploy/compose.go.yml up -d --no-deps music-party
docker compose -f docker-compose.yml -f backend-go/deploy/compose.go.yml ps
curl --fail --silent --show-error http://127.0.0.1:8848/actuator/health/readiness
```

必须使用原前端完成管理员登录、普通成员登录、房间进入、WebSocket 重连、点歌、暂停、seek、切歌、聊天、播放列表和至少一个已验收平台的实际播放。观察日志、readiness、HTTP 5xx、WebSocket 断开、SQLite BUSY、goroutine 和进程 RSS。任何 schema 修改都应立即停止切换。

## 回滚

达到任一条件立即回滚：readiness 连续失败、登录或房间授权不可用、队列/播放状态不收敛、数据库检查失败、核心平台不可播放，或 10 分钟内无法解释的错误持续出现。

1. 停止 Go 容器，确保没有 SQLite 写入者。
2. 保存 Go 日志和故障时间段指标。
3. 用快照替换数据库前，再运行 `cutover.sh verify <snapshot-path>` 并保留 SHA-256。不要覆盖唯一快照。
4. 将当前数据库另存为故障证据，再把已验证快照内容覆盖回现有 `musicparty.db`。不要先删除目标文件；覆盖现有文件可以保留 Java/Go 共用的属主和权限。恢复后用 `stat` 确认 UID/GID 仍等于 `MUSIC_PARTY_RUNTIME_UID:MUSIC_PARTY_RUNTIME_GID`。
5. 将 `MUSIC_PARTY_IMAGE` 恢复为记录的 Java digest，使用原 Compose 启动。
6. 验证 Java readiness、登录、WebSocket、队列、播放和数据库完整性。

Java 镜像与构建链至少保留 30 天或两个稳定版本。稳定期内不得运行仅 Go 可识别的 schema migration。
