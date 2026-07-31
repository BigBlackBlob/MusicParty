# Go 生产切换包

本目录只准备阶段 9，不会自动连接或修改生产环境。首次切换继续遵守：5–15 分钟维护窗口、单 SQLite、无 schema migration、Java 可立即回滚。

## 硬门槛

开始维护窗口前必须全部满足：

- 阶段 8 尚未关闭的真实平台、完整 Netease 流和干净镜像重建验收已有书面证据，或由负责人明确接受对应风险。
- `.github/workflows/go-backend-release.yml` 从计划切换的提交发布成功，记录 `ghcr.io/...@sha256:...`，禁止使用移动标签。
- VPS 上已拉取并检查该 digest，现有 Java digest、配置文件路径、数据目录属主和剩余磁盘空间已记录。
- 维护公告、操作人、观察人、回滚决策人和窗口开始/结束时间已确定。
- 不改动 `.env` 中的凭据；切换日志不得输出其内容。

## 维护窗口

以下命令都在仓库根目录执行。它们不会替用户决定何时停生产服务。

```sh
export MUSIC_PARTY_IMAGE='ghcr.io/OWNER/musicparty-go@sha256:...'
docker pull "$MUSIC_PARTY_IMAGE"
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
4. 将当前数据库另存为故障证据，再将已验证快照复制回原数据库路径；保持原属主和权限。
5. 将 `MUSIC_PARTY_IMAGE` 恢复为记录的 Java digest，使用原 Compose 启动。
6. 验证 Java readiness、登录、WebSocket、队列、播放和数据库完整性。

Java 镜像与构建链至少保留 30 天或两个稳定版本。稳定期内不得运行仅 Go 可识别的 schema migration。
