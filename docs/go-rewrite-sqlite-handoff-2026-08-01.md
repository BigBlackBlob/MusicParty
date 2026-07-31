# MusicParty Java → Go SQLite 最终交接验收方案

日期：2026-08-01

状态：已批准，等待最终 Go 候选完成后执行。

## 目的

使用 VPS 正在运行的 Java 服务所持有的真实 SQLite 数据库的一致副本，完成一次 Java → Go 交接验证。该流程证明 Go 能接管真实数据，同时保留首次切换期间回滚 Java 的能力。

这不是日常测试，也不进入普通 Java、Go 或远端 CI。完整流程只在最终阶段 8 候选完成后执行一次；正式生产切换当天只重新生成最新快照并执行完整性检查，不重复完整往返。

## 数据库位置

当前 Compose 挂载关系为：

```text
VPS 主机：<Compose 部署目录>/music_party/data/musicparty.db
应用容器：/app/data/musicparty.db
```

在 VPS 上使用以下命令取得实际主机目录，禁止猜测部署路径：

```sh
docker inspect music-party-app \
  --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{println .Source}}{{end}}{{end}}'
```

## 创建运行中数据库的一致副本

禁止在 Java 运行时直接只复制 `musicparty.db`，因为最新事务可能仍在 `musicparty.db-wal`。当前 Java 镜像包含 Python 3，应使用 Python SQLite backup API 创建一致副本：

```sh
SNAPSHOT="musicparty-handoff-$(date -u +%Y%m%dT%H%M%SZ).db"

docker exec \
  -e SNAPSHOT="$SNAPSHOT" \
  music-party-app \
  python3 -c '
import os
import sqlite3

target = "/app/data/" + os.environ["SNAPSHOT"]
source = sqlite3.connect("file:/app/data/musicparty.db?mode=ro", uri=True)
destination = sqlite3.connect(target)
source.backup(destination)
print("snapshot:", target)
print("integrity:", destination.execute("PRAGMA integrity_check").fetchall())
print("foreign_keys:", destination.execute("PRAGMA foreign_key_check").fetchall())
destination.close()
source.close()
'
```

创建后在 VPS 主机计算 SHA-256，并通过 SSH/SCP 下载。下载完成后在本机重新计算 SHA-256；两端必须一致。

数据库包含 Session、密码哈希、平台 Cookie 和加密凭据。副本必须保存在仓库外，不得进入 Git、聊天附件、云盘或验收日志。验收证据只能记录文件大小、SHA-256、表数量和检查结果。

## 一次性交接流程

下载的文件保留为只读原件。复制出单独的工作副本，所有写入只发生在工作副本。

1. 对原件和工作副本记录 SHA-256、文件大小、`integrity_check`、`foreign_key_check` 和应用表数量。
2. 使用冻结 Java 基线启动工作副本，读取已有账号、房间、队列或播放列表，并通过正常应用接口写入一条可识别的交接测试记录；随后停止 Java。
3. 使用最终 Go 候选启动同一工作副本，必须设置 `DB_INIT_SCHEMA=false`。Go 读取真实数据及 Java 测试记录，再通过正常应用接口写入一条 Go 交接测试记录；随后停止 Go。
4. 再次启动冻结 Java 基线，只读取并确认 Go 写入的记录，以证明首次 Go 切换期间仍可回滚 Java。Java 不需要再次写入。
5. 最后由 Go `dbcheck` 执行完整性、外键和应用表检查，并确认 Go 仍可读取双方写入的记录。

最小写入次数固定为 Java 一次、Go 一次。Java 最后的回查只用于验证回滚，不形成长期 Java 测试负担。

## 失败边界

出现以下任一情况即停止交接，不允许 Go 修改生产 schema：

- 下载前后 SHA-256 不一致；
- `integrity_check` 不是 `ok`；
- 存在外键异常；
- 最终 Go 候选在 `DB_INIT_SCHEMA=false` 下拒绝当前 schema；
- Go 无法读取 Java 测试记录；
- Java 无法读取 Go 测试记录；
- 任一应用通过正常接口写入失败；
- 表数量或关键数据在没有明确操作的情况下发生变化。

如果真实数据库尚未达到冻结 Java 基线 schema，应先在隔离副本中确认差异，再决定是否需要让冻结 Java 基线在生产切换前完成既有初始化。Go 首次生产启动不得承担 schema migration。

## 正式切换当天

完整交接验收通过后，正式切换当天只需要：

1. 停止唯一 Java 写入者或使用批准的一致性快照流程取得最新数据库快照。
2. 记录快照 SHA-256，并执行 `dbcheck`。
3. 使用 `DB_INIT_SCHEMA=false` 启动不可变 digest 的 Go 镜像。
4. 验证 readiness、登录、房间、WebSocket、队列和普通播放。
5. 保留切换前快照和 Java 镜像，直到 Go 通过初始稳定观察期。

生产切换不重复完整 Java → Go → Java 往返；该往返只对最终候选与真实数据库副本执行一次。

## 正式证据

最终只提交不含数据库内容的文本或 JSON 摘要，至少记录：

```text
baselineTag
baselineCommit
candidateCommit
snapshotTimestamp
snapshotBytes
snapshotSha256
applicationTableCount
integrityResult
foreignKeyResult
javaWriteResult
goReadJavaResult
goWriteResult
javaReadGoResult
finalDbcheckResult
```

真实数据库原件和工作副本均不进入版本库。
