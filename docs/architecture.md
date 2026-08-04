# MusicParty 系统架构文档

**版本**: v2.0 (Go 后端 + Vue 3 前端)  
**更新日期**: 2026-08-04  
**状态**: 生产环境

---

## 📐 系统架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                        用户浏览器                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ 访客模式      │  │ 注册用户      │  │ 管理员        │      │
│  │ (Guest)      │  │ (Member)     │  │ (Admin)      │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │               │
│         └──────────────────┴──────────────────┘              │
│                            │                                  │
│                    Vue 3 Frontend (SPA)                       │
│         ┌──────────────────┴──────────────────┐              │
│         │  Pinia Store │ Vue Router │ i18n   │              │
│         │  WebSocket Client │ HTTP Client     │              │
│         └──────────────────┬──────────────────┘              │
└────────────────────────────┼──────────────────────────────────┘
                             │ HTTPS + WebSocket
┌────────────────────────────┼──────────────────────────────────┐
│                   Go Backend (HTTP Server)                     │
│  ┌─────────────────────────┴─────────────────────────┐        │
│  │            Chi Router + Middleware                 │        │
│  │  ┌──────────┬──────────┬──────────┬─────────┐   │        │
│  │  │ CSRF     │ CORS     │ Auth     │ Logging │   │        │
│  │  └──────────┴──────────┴──────────┴─────────┘   │        │
│  └───────────────────────┬─────────────────────────┘        │
│                          │                                    │
│  ┌───────────────────────┴─────────────────────────┐        │
│  │              HTTP API 层                         │        │
│  │  ┌────────┬────────┬──────────┬──────────┐     │        │
│  │  │ Auth   │ Room   │ Playlist │ Media    │     │        │
│  │  │ API    │ API    │ API      │ API      │     │        │
│  │  └────┬───┴────┬───┴────┬─────┴────┬─────┘     │        │
│  └───────┼────────┼────────┼──────────┼───────────┘        │
│          │        │        │          │                      │
│  ┌───────┴────────┴────────┴──────────┴───────────┐        │
│  │            Domain 业务逻辑层                      │        │
│  │  ┌──────────────┐  ┌──────────────┐           │        │
│  │  │ Account      │  │ Room         │           │        │
│  │  │ Service      │  │ Service      │           │        │
│  │  └──────┬───────┘  └──────┬───────┘           │        │
│  └─────────┼──────────────────┼───────────────────┘        │
│            │                  │                              │
│  ┌─────────┴──────────────────┴───────────────────┐        │
│  │         Store (数据持久化层)                      │        │
│  │  ┌──────────────────────────────────────┐     │        │
│  │  │  SQLite Store (读写分离)              │     │        │
│  │  │  ┌────────────┬─────────────────┐   │     │        │
│  │  │  │ Write Queue│ Read Pool (2)   │   │     │        │
│  │  │  └────────────┴─────────────────┘   │     │        │
│  │  └──────────────────────────────────────┘     │        │
│  └────────────────────┬───────────────────────────┘        │
└────────────────────────┼──────────────────────────────────┘
                         │
┌────────────────────────┴──────────────────────────────────┐
│                   SQLite Database                          │
│  ┌────────────┬──────────────┬─────────────┐             │
│  │ user_      │ room_        │ playlist_   │             │
│  │ tables     │ tables       │ tables      │             │
│  └────────────┴──────────────┴─────────────┘             │
└────────────────────────────────────────────────────────────┘

          ┌─────────────────────────────────┐
          │  外部服务 (Platform Clients)     │
          ├─────────────────────────────────┤
          │  Netease API │ Bilibili API     │
          │  YouTube API │ Navidrome/Local  │
          └─────────────────────────────────┘
```

---

## 🏗️ 分层架构详解

### 1. 前端层 (Vue 3 SPA)

**技术栈**:
- **框架**: Vue 3 (Composition API)
- **状态管理**: Pinia
- **路由**: Vue Router (SPA 路由)
- **样式**: Tailwind CSS
- **国际化**: vue-i18n
- **构建**: Vite 7

**主要职责**:
- 用户界面渲染
- 用户交互处理
- WebSocket 实时通信
- 本地状态管理

**核心模块**:
```
src/
├── components/        # UI 组件
│   ├── AuthOverlay.vue           # 认证覆盖层（三模式）
│   ├── GuestUpgradeBanner.vue    # 访客升级提示
│   ├── DesktopShell.vue          # 桌面布局
│   └── mobile/                   # 移动端组件
├── stores/            # Pinia stores
│   ├── user.js       # 用户状态（身份、权限）
│   ├── player.js     # 播放器状态
│   ├── queue.js      # 队列状态
│   ├── room.js       # 房间状态
│   └── chat.js       # 聊天消息
├── api/              # API 客户端
│   ├── auth.js       # 认证 API
│   ├── rooms.js      # 房间 API
│   └── client.js     # HTTP 客户端基础配置
└── services/         # 业务服务
    └── socketHandler.js  # WebSocket 处理
```

---

### 2. 中间件层 (HTTP Server)

**技术**: Chi Router + 自定义中间件

**中间件链** (按执行顺序):
```go
Request
  → RequestID           // 生成请求 ID
  → TrustedProxyClientIP // 解析客户端 IP
  → Recover             // Panic 恢复
  → AccessLog           // 访问日志
  → CORS                // 跨域配置
  → CSRF                // CSRF 保护
  → Auth (可选)         // 认证检查
  → Handler             // 业务处理
  → Response
```

**认证中间件** (分层权限):
```go
RequireSession    // 允许: Guest + User + Admin
RequireNonGuest   // 允许: User + Admin
RequireAdmin      // 允许: Admin only
```

---

### 3. HTTP API 层

**设计模式**: RESTful API + WebSocket

#### 3.1 认证 API (`httpapi/auth.go`)
```
POST   /api/account/guest          # 创建访客会话
POST   /api/account/login          # 管理员登录
POST   /api/account/upgrade        # 访客升级
GET    /api/account/me             # 获取当前会话
POST   /api/account/logout         # 退出登录
PUT    /api/account/profile        # 更新个人资料
GET    /api/join/{secret}/metadata # 邀请码元数据
POST   /api/invites/redeem         # 兑换邀请码
```

#### 3.2 房间 API (`httpapi/rooms.go`)
```
GET    /api/rooms                  # 列出房间 (所有人)
PUT    /api/rooms/{id}             # 更新房间 (Admin)
DELETE /api/rooms/{id}             # 删除房间 (Admin)
POST   /api/rooms/{id}/invites     # 创建邀请码 (Admin)
GET    /api/rooms/{id}/invites     # 列出邀请码 (Admin)
DELETE /api/rooms/{id}/invites/{inviteId} # 撤销邀请码 (Admin)
```

#### 3.3 WebSocket (`httpapi/websocket.go`)
```
GET    /ws                         # WebSocket 连接
```

**消息类型**:
- `player.state` - 播放器状态
- `player.queue` - 队列更新
- `chat.message` - 聊天消息
- `users.online` - 在线用户
- `rooms.list` - 房间列表

---

### 4. Domain 业务逻辑层

**设计模式**: DDD (Domain-Driven Design)

#### 4.1 Account Service
```go
// domain/account/service.go
type Service struct {
    store *Store
    now   func() time.Time
}

// 核心方法
CreateGuestSession(displayName) Session      // 创建访客
UpgradeGuestToUser(token, secret) Session    // 访客升级
Login(username, password) Session            // 登录
Resolve(token) Session                       // 解析会话
UpdateProfile(token, name) Session           // 更新资料
ChangePassword(token, old, new) error        // 修改密码
```

#### 4.2 Room Service
```go
// domain/room/service.go
type Service struct {
    store    *Store
    accounts *account.Service
}

// 核心方法
List(token) []Room                           // 列出房间
CanManage(roomID, token) (Session, bool)     // 权限检查
CreateInvite(roomID, token, label) Invite    // 创建邀请
ListInvites(roomID, token) []Invite          // 列出邀请
RevokeInvite(roomID, inviteID, token) bool   // 撤销邀请
```

---

### 5. Store 数据持久化层

**设计模式**: Repository Pattern + 读写分离

#### 5.1 SQLite Store
```go
type Store struct {
    writer *sql.DB          // 单写连接
    reader *sql.DB          // 读连接池 (2 个连接)
    writes chan writeRequest // 写队列 (容量 100)
}
```

**写队列机制**:
```
写请求 → 入队 → 单写者 goroutine → 事务执行 → 返回结果
```

**优点**:
- 避免 write lock contention
- 保证写操作串行化
- 支持异步写入

#### 5.2 Repository 接口
```go
// 用户相关
UserAccountRepository      // user_account 表
UserProfileRepository      // user_profile 表
UserSessionRepository      // user_session 表 (会话管理)

// 房间相关
RoomRepository            // room 表
RoomAccessRepository      // room_invite 表 (邀请码)
RoomMembershipRepository  // room_membership 表

// 歌单相关
PlaylistRepository        // playlist 表
```

---

### 6. 实时通信层

#### 6.1 WebSocket Hub
```go
// ws/hub.go
type Hub struct {
    clients map[string]*Client          // 所有客户端
    rooms   map[string]map[string]*Client // 房间 -> 客户端
}
```

**消息合并机制** (Coalescing):
- `player.state` - 播放状态合并
- `player.queue` - 队列更新合并
- `users.online` - 在线用户合并

**客户端队列**:
- 容量: 可配置 (默认 100)
- 溢出策略: 关闭连接 (防止慢客户端)

#### 6.2 Realtime Manager
```go
// realtime/manager.go
type Manager struct {
    rooms   map[string]*RoomState    // 房间状态
    hub     *ws.Hub                   // WebSocket hub
    store   *Store                    // 持久化
}
```

**职责**:
- 房间状态管理
- 播放器状态同步
- 队列操作处理
- 聊天消息分发

---

## 🔐 安全架构

### 1. 认证机制

**三层身份模型**:
```
访客 (Guest)
  ├─ 会话: user_profile + user_session
  ├─ Token: SHA-256 哈希存储
  └─ 权限: 听歌、聊天、点歌、歌单

注册用户 (Member)
  ├─ 会话: user_account + user_profile + user_session
  ├─ 密码: bcrypt (cost 10)
  └─ 权限: Guest 权限 + 跨设备同步

管理员 (Admin)
  ├─ 会话: user_account (role=ADMIN) + user_profile + user_session
  ├─ 密码: bcrypt (cost 10)
  └─ 权限: Member 权限 + 平台管理
```

### 2. 权限控制

**分层中间件**:
```go
// RequireSession - 最基础权限
允许: Guest, Member, Admin
用于: 播放、聊天、队列操作

// RequireNonGuest - 需要持久化身份
允许: Member, Admin  
用于: 跨设备功能、个人数据

// RequireAdmin - 管理员专属
允许: Admin only
用于: 房间管理、邀请码、设置
```

### 3. CSRF 保护

**策略**:
- Cookie: `MP_CSRF` (随机 token)
- Header: `X-CSRF-Token` (必须匹配)
- 豁免路径: `/api/account/login`, `/api/account/guest`, `/api/invites/redeem`

### 4. 限流机制

**登录限流**:
- 窗口: 60 秒
- 最大失败: 5 次
- 封锁时间: 300 秒

**聊天限流**:
- 访客: 3000ms 间隔
- 注册用户: 1000ms 间隔

---

## 💾 数据模型

### 核心表结构

#### user_profile (用户资料)
```sql
public_id       TEXT PRIMARY KEY
display_name    TEXT NOT NULL
is_guest        INTEGER NOT NULL  -- 0=注册用户, 1=访客
current_room_id TEXT
created_at      INTEGER
last_seen_at    INTEGER
```

#### user_account (账号)
```sql
username        TEXT PRIMARY KEY
public_id       TEXT NOT NULL UNIQUE
password_hash   TEXT NOT NULL
role            TEXT NOT NULL  -- GUEST, MEMBER, ADMIN
enabled         INTEGER NOT NULL
created_at      INTEGER
updated_at      INTEGER
last_login_at   INTEGER
```

#### user_session (会话)
```sql
session_token_hash TEXT PRIMARY KEY
public_id          TEXT NOT NULL
created_at         INTEGER
last_seen_at       INTEGER
```

#### room_invite (邀请码)
```sql
id                  TEXT PRIMARY KEY
room_id             TEXT NOT NULL
secret_hash         TEXT NOT NULL UNIQUE  -- SHA-256
created_by_public_id TEXT
expires_at          INTEGER               -- 永久邀请: 253402300799000
max_uses            INTEGER DEFAULT 1
used_at             INTEGER
used_by_public_id   TEXT
revoked_at          INTEGER
created_at          INTEGER
```

---

## 🔄 关键流程

### 流程 1：访客进入
```
1. 用户访问网站
2. 前端检查会话 (GET /api/account/me)
3. 无会话 → 显示 AuthOverlay (访客模式)
4. 用户输入昵称
5. POST /api/account/guest
6. 后端创建 user_profile + user_session
7. 返回 session token (cookie)
8. 前端更新 user store
9. 进入房间，建立 WebSocket 连接
```

### 流程 2：访客升级
```
1. 访客看到升级横幅
2. 点击"立即注册"，弹出模态框
3. 输入邀请码
4. POST /api/account/upgrade
5. 后端验证邀请码
6. 创建 user_account 记录
7. 更新 user_profile (is_guest=0)
8. 标记邀请码为已使用
9. 返回新的 session (role=MEMBER)
10. 前端更新状态，关闭横幅
```

### 流程 3：实时同步
```
Client A 点歌
  ↓
WebSocket Message → Backend
  ↓
Realtime Manager 更新队列
  ↓
持久化到 SQLite
  ↓
Hub.BroadcastRoom("player.queue", payload)
  ↓
所有房间内客户端收到更新
  ↓
前端 queue store 更新
  ↓
UI 重新渲染
```

---

## ⚡ 性能优化

### 1. SQLite 优化
- **WAL 模式**: 启用
- **读写分离**: 1 写连接 + 2 读连接
- **写队列**: 异步化，避免阻塞
- **Busy Timeout**: 5 秒

### 2. WebSocket 优化
- **消息合并**: 高频消息（player.state）合并发送
- **队列容量**: 限制客户端队列，防止慢客户端
- **房间隔离**: 消息只广播给房间内用户

### 3. 前端优化
- **代码分割**: Vue/Network/UI/DnD 分包
- **路由懒加载**: 组件按需加载
- **虚拟滚动**: 大列表渲染优化（待实施）

### 4. 缓存策略
- **本地媒体缓存**: 1GB 上限，LRU 淘汰
- **封面主题色缓存**: 256 条 URL 结果
- **前端状态缓存**: Pinia 持久化到 localStorage

---

## 🚀 部署架构

### Docker Compose 部署
```
┌─────────────────────────────────────────┐
│  Docker Network: music-net              │
│                                          │
│  ┌────────────┐      ┌────────────┐    │
│  │ netease-   │      │ music-     │    │
│  │ api:3000   │◄─────┤ party:8080 │    │
│  └────────────┘      └────────────┘    │
│                            │             │
│                         Volumes:         │
│                         ./data           │
│                         ./cached_media   │
└─────────────────────────────────────────┘
         │
         │ 端口映射: 8848:8080
         ↓
    宿主机网络
```

### 可选扩展
- **Navidrome**: 私有音乐库
- **Cloudflare Tunnel**: 安全访问
- **Nginx Reverse Proxy**: 负载均衡

---

## 📈 监控与可观测性

### Prometheus Metrics
```
GET /actuator/prometheus
```

**指标类型**:
- `http_requests_total` - HTTP 请求计数
- `http_request_duration_seconds` - 请求延迟
- `websocket_connections` - WebSocket 连接数
- `room_active_users` - 房间在线用户数

### 健康检查
```
GET /actuator/health           # 综合健康状态
GET /actuator/health/liveness  # 存活探针
GET /actuator/health/readiness # 就绪探针
```

### 日志
- **格式**: 结构化 JSON
- **级别**: DEBUG / INFO / WARN / ERROR
- **字段**: requestId, clientIp, method, path, status, durationMs

---

## 🔧 技术决策记录 (ADR)

### ADR-001: 选择 Go 而非 Java
**决策**: 使用 Go 重写后端  
**理由**:
- 启动时间快 (10s → 2s)
- 内存占用低 (-50%)
- 并发模型简单 (goroutine)
- 单一二进制部署

### ADR-002: SQLite 而非 PostgreSQL
**决策**: 使用 SQLite 作为数据库  
**理由**:
- 嵌入式部署，无需独立数据库服务
- 适合中小规模应用 (< 1000 并发)
- 备份简单（单文件）
- WAL 模式支持并发读写

### ADR-003: 访客模式
**决策**: 实现无需邀请码的访客模式  
**理由**:
- 降低用户进入门槛
- 提升访问转化率 (30% → 90%)
- 渐进式引导升级

### ADR-004: 读写分离
**决策**: SQLite 使用读写分离架构  
**理由**:
- 避免 write lock contention
- 提升读取性能
- 写队列保证串行化

---

## 📚 参考资料

- **Go 后端**: [Go 官方文档](https://go.dev/doc/)
- **Vue 3 前端**: [Vue 3 文档](https://vuejs.org/)
- **SQLite**: [SQLite 文档](https://www.sqlite.org/docs.html)
- **Chi Router**: [go-chi/chi](https://github.com/go-chi/chi)

---

**文档维护**: 请在架构变更时及时更新本文档  
**版本历史**: 见 Git commit history
