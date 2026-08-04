# MusicParty 一次性永久邀请码管理：新 Session 交接

## 给新 Session 的直接指令

请在当前仓库直接实现本交接中的功能。UI 方案已经由用户选定为方案 A，无需重新制作设计稿或再次询问布局方案。

不要使用或修复 `sol-advisor`，它已被用户移除。不要连接 VPS、修改生产数据库、重新部署生产服务或读取 COOKIE / SESSDATA。

## 目标

在前端管理员设置中新增独立的“邀请码”入口，允许平台管理员：

1. 选择房间。
2. 输入可选备注并生成邀请码。
3. 立即复制完整邀请链接。
4. 查看历史邀请码的状态。
5. 撤销仍可使用的邀请码。

新生成的邀请码语义必须是：

- 单次使用。
- 永不过期，直到被使用或由管理员撤销。
- 明文 secret 只在创建响应中出现一次；列表 API 不得返回 secret。
- 已使用的邀请码不能再次兑换。
- 已撤销的邀请码不能兑换。

现有旧邀请码保持原有到期时间，不对历史数据做批量迁移。

## 当前仓库状态

- 分支：`NRT-Base`
- 当前提交：`832bb32e4618c48d1958dfe1a080d3b3addeb2e3`
- 远端：`origin/NRT-Base` 与当前提交同步。
- Go 后端已在 VPS 完成切换，但本任务不得接触 VPS。

必须保留、不得修改或提交的用户文件：

```text
music_party/data/musicparty.db
docs/fix-verification-2026-07-26.md
docs/golang-rewrite-estimation-2026-07-26.md
docs/performance-stability-optimization-plan-2026-07-26.md
docs/test-fix-report-2026-07-26.md
```

本交接文档是协调文件，除非用户明确要求，否则不要把它加入功能提交。

## 已存在的后端能力

路由已经存在，无需新增数据库表或新路由：

```text
POST   /api/rooms/{roomId}/invites
GET    /api/rooms/{roomId}/invites
DELETE /api/rooms/{roomId}/invites/{inviteId}
GET    /api/join/{secret}/metadata
POST   /api/invites/redeem
```

相关实现：

```text
backend-go/internal/httpapi/rooms.go
backend-go/internal/domain/room/service.go
backend-go/internal/domain/account/service.go
```

当前邀请：

- `max_uses=1`。
- secret 以 SHA-256 存储。
- 创建响应包含一次性 `secret`。
- 列表响应不包含 `secret`。
- 当前默认有效期为 7 天：`InviteTTL = 7 * 24 * time.Hour`。
- 权限由 `room.Service.CanManage` 控制，平台管理员和房间 OWNER 均可调用后端接口。
- 兑换成功时会同时填写 `used_at` 和 `revoked_at`，因此前端状态判断必须优先显示“已使用”，再判断“已撤销”。

## 永久邀请码的兼容实现

不要修改 `room_invite` 表结构，也不要使用 `expires_at=0`、`NULL` 或 Go 专属数据库语义。Java 回滚链仍处于保留期，这些做法可能被 Java 判断为已过期。

推荐使用 Java 和现有 SQL 比较都能正常识别的远未来时间：

```go
const PermanentInviteExpiresAt int64 = 253402300799000 // 9999-12-31T23:59:59Z
```

实现要求：

- `CreateInvite` 将新邀请码的 `expires_at` 写为该常量。
- 保留现有兑换 SQL 的 `expires_at >= now` 条件，不需要改变账户兑换事务。
- 在 `room.Invite` 增加 additive JSON 字段：

```go
Permanent bool `json:"permanent"`
```

- 创建响应中 `permanent=true`。
- 列表读取后，根据 `expires_at == PermanentInviteExpiresAt` 设置 `permanent=true`。
- 旧的有限期邀请码返回 `permanent=false`，并继续按原时间过期。
- 可以删除不再使用的 `InviteTTL`，但不要改变其他邀请或账户逻辑。

这种实现没有 schema migration，Java 若回滚也会把该值视为尚未过期。

## 已存在的前端能力

`music-party-web/src/api/auth.js` 已经包含：

```js
createInvite(roomId, label)
listInvites(roomId)
revokeInvite(roomId, inviteId)
inviteMetadata(secret)
redeemInvite(secret, displayName)
```

不要重复在 `rooms.js` 中再创建一套 API，除非同时完成清晰且必要的迁移。目前直接复用 `authApi` 最符合 YAGNI。

邀请链接入口已经由 `AuthOverlay.vue` 支持：

```text
/join/{secret}
```

生成完整链接时应基于当前公开 origin：

```js
new URL(`/join/${encodeURIComponent(secret)}`, window.location.origin).toString()
```

不要记录 secret、写入 localStorage、写入日志或放入历史列表状态。

## 选定的 UI：方案 A

管理员设置导航新增独立项：

```text
账号
通用设置
本地曲库
音源管理
邀请码
管理员
在线成员
```

建议新增：

```text
music-party-web/src/components/InviteManager.vue
```

并在 `SettingsCenter.vue` 中：

- 仅当 `user.isAdmin` 时显示“邀请码”导航。
- `activeSection === 'invites'` 时渲染 `InviteManager`。
- 不要把邀请码逻辑继续塞入已经很大的 `AdminSettingsPanel.vue`。

### 页面内容

1. 标题“邀请码”和简短说明。
2. 房间选择器，默认当前 `roomStore.currentRoomId`。
3. 可选备注输入，最大 64 个字符。
4. “生成永久邀请码”按钮。
5. 创建成功后显示一次性安全提示、只读完整链接和“复制链接”按钮。
6. 历史列表展示：备注、状态、有效期、操作。
7. 永久邀请的有效期显示“永久”。
8. 仅“可使用”的邀请码显示“撤销”；撤销前使用项目已有风格的 `window.confirm`。
9. 具备加载、空列表、失败、复制失败和按钮 busy 状态。

### 状态优先级

按以下顺序判断：

```text
usedAt != null                -> 已使用
revokedAt != null             -> 已撤销
permanent                     -> 可使用 / 永久
expiresAt < 当前时间          -> 已过期
其他                          -> 可使用
```

因为兑换事务会同时设置 `used_at` 与 `revoked_at`，“已使用”必须排在“已撤销”之前。

### 安全和可访问性

- 一次性链接区域使用 `aria-live="polite"`。
- 输入和房间选择器必须有可访问名称。
- 不要通过颜色单独传达状态，状态必须有文字。
- secret 只保存在组件内存的临时 ref 中；切换房间时清空。
- 刷新页面后无法恢复 secret，这是预期的安全行为。
- 不把 secret 放进 toast 文本。
- 使用现有 CSS token、纯黑/深色风格和紧凑的信息密度。
- 移动端导航和管理页必须可用，不新增持续重绘动画。

## 建议文件范围

后端：

```text
backend-go/internal/domain/room/service.go
backend-go/internal/domain/room/service_test.go
```

前端：

```text
music-party-web/src/components/SettingsCenter.vue
music-party-web/src/components/InviteManager.vue
music-party-web/src/api/auth.js                 # 仅在现有方法确有缺陷时修改
music-party-web/src/i18n/zh.js
music-party-web/src/i18n/en.js
music-party-web/src/utils/invites.js            # 可选，承载纯状态/URL逻辑
music-party-web/src/utils/invites.test.js       # 若新增上述 helper
```

避免修改 `AdminSettingsPanel.vue`，除非复用样式时确有必要且改动很小。

## 测试要求

### Go

至少覆盖：

- 新邀请码使用固定的永久时间并返回 `permanent=true`。
- 列表能区分永久新邀请码与旧的有限期邀请码。
- 永久邀请码仍只能兑换一次。
- 管理员撤销后无法兑换。
- 无管理权限的会话不能创建、列表或撤销。

执行：

```sh
cd backend-go
go test ./internal/domain/room ./internal/domain/account
```

如时间允许，再运行：

```sh
go test ./...
```

### 前端

至少覆盖纯逻辑：

- 状态优先级，特别是同时存在 `usedAt` 和 `revokedAt` 时显示“已使用”。
- 永久邀请显示“永久”。
- 完整邀请 URL 使用当前 origin 和编码后的 secret。
- 设置导航仅向管理员显示邀请码入口。

执行：

```sh
cd music-party-web
pnpm test:run
pnpm lint
pnpm build
```

如果本机只安装了 npm，也必须遵循现有 lockfile/项目实际包管理状态，不要无故重写锁文件。

### 浏览器验证

使用本地或隔离测试服务验证：

1. 管理员能看到独立“邀请码”导航。
2. 创建后完整链接只出现一次且能复制。
3. 刷新后列表中没有 secret。
4. 永久邀请显示“永久”。
5. 撤销后状态更新且链接不可兑换。
6. 已使用邀请显示“已使用”，不能撤销或再次兑换。
7. 桌面和移动宽度均无溢出。

不要使用 VPS 或生产数据库完成这些验证。

## 完成标准

- 功能实现和测试通过。
- 不包含数据库、COOKIE、SESSDATA、session、日志或凭据。
- 不修改用户已有脏文件。
- 不触碰生产环境。
- 最终报告实际修改文件、测试结果和仍需部署的新镜像步骤。
- 除非用户在新 session 明确要求，否则不要自动提交、推送、发布镜像或部署 VPS。
