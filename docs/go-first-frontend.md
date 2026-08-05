# Go-first 前端开发说明

前端只支持 Go 后端。不要新增 Java HTTP、STOMP/SockJS、浏览器可读会话 token 或旧 payload 的兼容代码。

## 数据边界

- `src/contracts/generated/`：唯一的 HTTP 与 WebSocket 类型来源；禁止手改。
- `src/transport/httpClient.ts`：Cookie session、CSRF、超时、取消和 `APIError`。
- `src/transport/realtimeClient.ts`：唯一的 WebSocket 连接与发送入口。
- TanStack Vue Query：HTTP 服务端资源，例如房间、平台、搜索、歌单、成员、邀请码和管理资源。
- Pinia：会话、当前房间上下文、实时房间快照、命令、音频和本地偏好；不得复制 Vue Query 的服务端缓存。

实时播放相关职责保持分离：

- `roomRuntimeStore` 保存服务端权威播放状态和版本。
- `roomCommandStore` 管理 mutation ID、ack/nack、超时、回滚和 resync。
- `audioPlaybackStore` 保存浏览器本地音频状态。
- `roomRealtimeCoordinator` 管理房间连接、心跳、重连、可见性恢复和房间切换。

## 会话和权限

认证仅使用后端设置的 HttpOnly Cookie。前端通过 `/api/account/me` 恢复会话，按 capability 而不是显示名、错误文本或旧角色别名决定界面可用性。私有房间访问同样只依赖 HttpOnly room-access Cookie。

## 生成契约

从仓库根目录执行：

```powershell
cd backend-go
go run ./cmd/contractgen -repo ..
go run ./cmd/contractgen -check -repo ..
```

第一条更新 `contracts/go/` 和 `music-party-web/src/contracts/generated/`；第二条用于 CI 或本地检查生成文件是否漂移。任何协议变更都应先扩展 Go contract generator，再修改前端消费方。

## 本地质量门禁

```powershell
cd music-party-web
pnpm typecheck
pnpm lint
pnpm test:run
pnpm build
pnpm test:e2e
pnpm audit --audit-level=moderate --registry=https://registry.npmjs.org

cd ../backend-go
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
```

`test:e2e` 使用 Go 测试后端，并包含入口、房间访问、会话迁移、移动端和视觉回归。更新截图基线只能在人工审查可见变化后执行：

```powershell
pnpm exec playwright test e2e/visual.spec.ts --update-snapshots --workers=1
```

## 视觉约束

Go-first 重构可以调整入口、房间流程和设置的信息架构，但不得借此改变主要视觉语言、自由布局、移动端和 Lite Mode 的既有使用方式。视觉变更应由 Playwright 基线和人工截图审查共同确认。
