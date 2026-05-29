# MusicParty NRT

> 私有化部署的多人实时听歌房间。搜索、点歌、排队、聊天、歌词、移动端播放和可选直播流都在同一个 Web 应用里完成。

本仓库是基于上游 MusicParty 的个人 fork。NRT-Base 分支在上游多人听歌基础上扩展了账号体系、多房间、SQLite 持久化、房间/个人歌单、本地媒体库、Subsonic/Navidrome 私有曲库、移动端、桌面模块化布局、播放同步、直播流、可访问性、国际化、前端构建体积和 Docker/CI 发布流程。

![Java](https://img.shields.io/badge/Java-21-orange)
![Spring Boot](https://img.shields.io/badge/Spring_Boot-3.2.5-green)
![Vue](https://img.shields.io/badge/Vue-3-4FC08D)
![Vite](https://img.shields.io/badge/Vite-7-646CFF)
![Docker](https://img.shields.io/badge/Docker-Ready-blue)

## 预览

![桌面端主界面](docs/assets/readme/desktop.png)

![移动端播放界面](docs/assets/readme/mobile.png)

## 核心特性

- **多人同步播放**：基于 Spring Boot WebSocket/STOMP 分发播放状态，前端按服务端时间轴、RTT 和漂移校准进度，支持手动 resync、平滑 seek 和浏览器侧切歌过渡。
- **多房间与私密房间**：支持 Lounge 以外的多房间会话，房间有独立队列、聊天、播放状态和在线人数；房主/管理员可创建、编辑、删除房间，私密房间使用密码校验和房间访问 token。
- **SQLite 持久化**：房间、队列、历史、聊天、播放快照、账号、用户资料、歌单、本地曲库和 Subsonic 源配置可持久化到 `data/`；冷房间可按快照恢复，旧 JSON 数据有迁移状态保护。
- **账号与权限**：首次启动创建管理员账号，后续使用账号密码登录；支持注册入口、登录限流、会话 token、显示名修改、密码修改、退出登录确认、管理员命令、房间管理权限和本地上传授权。
- **多音乐源与搜索**：支持网易云音乐、Bilibili、本地媒体库、Navidrome/Subsonic；搜索、用户歌单、歌单歌曲、专辑搜索、专辑歌曲、歌词详情和封面主题色提取统一走后端 API。
- **Subsonic/Navidrome 私有曲库**：管理员可在设置页管理多个 Subsonic 源、测试连接、排序、移除、绑定到房间并维护授权用户；音频/封面通过 MusicParty 后端代理，凭据加密持久化。
- **本地媒体库**：管理员或授权用户可上传本地音频，后端负责元数据/封面提取、转码和媒体代理；本地曲目可参与搜索、点歌和浏览器播放。
- **队列协作**：点歌、拖拽排序、置顶、批量置顶、批量删除、喜欢歌曲、喜欢列表导出，以及防止单人霸榜的公平随机策略。
- **房间与个人歌单**：支持房间共享歌单和个人歌单的创建、重命名、删除、曲目增删、排序、导入、导出和一键加入队列；可从网易云等外部歌单导入。
- **桌面模块化 UI**：桌面主界面采用可配置模块布局，支持 Now Playing、歌词、队列、聊天、在线成员、房间歌单等模块，提供编辑模式、列宽调整、模块选择、全局缩放和主舞台比例调节。
- **移动端体验**：移动端使用独立 shell、底部导航和播放/队列/搜索/聊天页面，支持移动预览、播放页密度设置、安全区适配、迷你歌词和完整歌词浮层。
- **歌词体验**：支持歌词与翻译歌词展示、自动滚动、字号调整、对齐切换和翻译开关；桌面和移动端共用 Apple 风格歌词面板。
- **实时互动**：聊天室、系统消息、在线成员、活跃成员弹层、点赞反馈、房间人数变化和直播流听众计数实时同步。
- **可选 HTTP 直播流**：通过 FFmpeg 输出 `/radio/stream`，带访问 key 和服务端广播管理，适合在 VRChat 等外部场景收听。
- **可访问性与国际化**：前端接入 `vue-i18n`，中英文文案覆盖主要界面；桌面壳、播放控制、队列/歌单图标按钮和可点击曲目行提供 accessible name、键盘焦点状态与按钮语义。
- **本地字体与构建分包**：Material Symbols 字体已本地化；Vite 生产构建会拆分 Vue、网络、UI、拖拽和工具依赖，降低入口 chunk 体积。
- **发布与质量门禁**：提供 Docker Compose、可选 Navidrome Compose、GitHub Actions 质量检查、Docker 发布和 Aliyun ACR 镜像工作流；前端包含 Vitest/ESLint 覆盖关键队列、音频、布局、a11y 和 payload 行为。

## 快速部署

推荐直接使用 Docker Compose 拉取预构建镜像运行：

```bash
docker compose pull
docker compose up -d
```

默认访问地址：

```text
http://localhost:8848
```

首次访问页面时会进入初始化流程，第一个注册账号会自动成为管理员。正式部署前至少修改这些环境变量：

```yaml
- BASE_URL=https://music.example.com
- NETEASE_COOKIE=
- BILIBILI_SESSDATA=
```

`BASE_URL` 必须是用户实际访问的完整地址，包含协议。直播流链接、部分后端生成的绝对 URL 都依赖它。
`NETEASE_COOKIE`、`BILIBILI_SESSDATA`、Navidrome/Subsonic 凭据等环境变量只作为首次迁移输入；长期配置应在设置页保存到 `data/` 卷内的 SQLite 数据库。

### 选择镜像源

`docker-compose.yml` 默认使用 GHCR：

```yaml
image: ${MUSIC_PARTY_IMAGE:-ghcr.io/bigblackblob/musicparty:nrt-base}
```

如果 VPS 需要走阿里云 ACR，把这一行的默认值改成：

```yaml
image: ${MUSIC_PARTY_IMAGE:-crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party:latest}
```

也可以不改文件，启动前设置环境变量：

```bash
MUSIC_PARTY_IMAGE=ghcr.io/bigblackblob/musicparty:nrt-base docker compose up -d
```

如果 VPS 访问 GHCR 不稳定，可以改用阿里云 ACR：

```bash
MUSIC_PARTY_IMAGE=crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party:latest docker compose up -d
```

如果要在本机从源码构建镜像，使用额外的 build override：

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## 环境变量

| 变量名 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `BASE_URL` | 否 | `http://localhost:8080` | 对外访问地址，生成直播流链接时使用。 |
| `ALLOWED_ORIGINS` | 否 | 从 `BASE_URL` 与本地开发地址派生 | WebSocket 允许的 Origin，多个值用英文逗号分隔。 |
| `APP_AUTHOR_NAME` | 否 | `ThorNex` | 页面品牌/作者名。 |
| `APP_BACK_WORDS` | 否 | `THORNEX` | 播放区背景装饰文字。 |
| `NETEASE_API_URL` | 是 | `http://netease-api:3000` | NeteaseCloudMusicApi 地址。 |
| `NETEASE_COOKIE` | 否 | 空 | 首次迁移用网易云 Cookie；长期值在设置页持久化保存。 |
| `NETEASE_QUALITY` | 否 | `exhigh` | 首次迁移用网易云音质，可选 `standard`、`higher`、`exhigh`、`lossless`、`hires`。 |
| `BILIBILI_SESSDATA` | 否 | 空 | 首次迁移用 Bilibili SESSDATA；长期值在设置页持久化保存。 |
| `YOUTUBE_ENABLED` | 否 | `true` | 是否启用 YouTube 搜索源；未配置 API key 或缺少 `yt-dlp` 时平台会自动隐藏。 |
| `YOUTUBE_API_KEY` | 否 | 空 | YouTube Data API Key，用于搜索和获取视频元数据。 |
| `YTDLP_PATH` | 否 | `yt-dlp` | `yt-dlp` 可执行文件路径；Docker 镜像内默认可直接使用。 |
| `YOUTUBE_SEARCH_LIMIT` | 否 | `20` | YouTube 单次搜索最大结果数。 |
| `QUEUE_MAX_SIZE` | 否 | `1000` | 队列最大长度。 |
| `QUEUE_HISTORY_SIZE` | 否 | `50` | 历史记录保留数量。 |
| `QUEUE_MAX_USER_SONGS` | 否 | `100` | 单用户最大排队歌曲数。 |
| `PLAYLIST_IMPORT_LIMIT` | 否 | `100` | 歌单/专辑导入时的歌曲数量上限。 |
| `CHAT_HISTORY_LIMIT` | 否 | `1000` | 聊天历史保留数量。 |
| `CHAT_MIN_INTERVAL` | 否 | `1000` | 聊天发送间隔，单位毫秒。 |
| `CHAT_MAX_LENGTH` | 否 | `200` | 单条聊天消息最大字符数。 |
| `CACHE_MAX_SIZE` | 否 | `1GB` | 本地媒体缓存上限，例如 `512MB`、`2GB`。 |
| `AUTH_RATE_LIMIT_ENABLED` | 否 | `true` | 是否启用账号登录限流。 |
| `AUTH_MAX_ATTEMPTS` | 否 | `5` | 登录最大失败次数。 |
| `AUTH_WINDOW_SECONDS` | 否 | `60` | 登录失败统计窗口。 |
| `AUTH_BLOCK_DURATION` | 否 | `300` | 触发限流后的封锁秒数。 |
| `NAVIDROME_ENABLED` | 否 | `false` | 是否启用 Navidrome 平台。 |
| `NAVIDROME_BASE_URL` | 否 | `http://navidrome:4533` | Navidrome 服务地址。 |
| `NAVIDROME_USERNAME` | 否 | 空 | 首次迁移用 Navidrome 用户名；长期值在设置页持久化保存。 |
| `NAVIDROME_PASSWORD` | 否 | 空 | 首次迁移用 Navidrome 密码；长期值加密保存。 |
| `NAVIDROME_CLIENT` | 否 | `musicparty` | Subsonic 客户端名。 |
| `NAVIDROME_API_VERSION` | 否 | `1.16.1` | Subsonic API 版本。 |
| `NAVIDROME_ALLOWED_USERS` | 否 | 空 | 首次迁移用授权列表，推荐填写账号用户名或 `publicId`，逗号分隔。 |

## 数据与持久化

- 默认 Docker 部署会把应用数据写入 `data/` 卷，包含 SQLite 数据库、本地上传媒体、缓存、站点设置和迁移状态。
- SQLite schema 覆盖房间、队列、播放快照、聊天、账号、用户资料、个人歌单、房间歌单、本地媒体库和 Subsonic 源配置。
- 队列、历史、聊天和播放状态会在关键变更后即时持久化，服务重启后可恢复房间状态；长期凭据应通过设置页保存，而不是长期依赖环境变量。
- 开发和测试环境仍保留 in-memory persistence fallback，便于单元测试和轻量本地调试。

## 管理员命令

登录管理员账号后，可在设置页或前端命令入口执行以下管理员命令：

| 命令 | 说明 |
| --- | --- |
| `//LOCK <TYPE> <ON/OFF>` | 锁定控制权限。`TYPE` 可选 `PAUSE`、`SKIP`、`SHUFFLE`、`ALL`。 |
| `//PAUSE` | 管理员强制暂停/恢复播放。 |
| `//SKIP` | 管理员强制切歌。 |
| `//SHUFFLE` | 管理员强制切换随机模式。 |
| `//RESET` | 重置播放状态、队列和聊天记录，谨慎使用。 |
| `//CLEAR <QUEUE/CHAT>` | 清空队列或聊天历史。 |
| `//STREAM ON` | 开启 HTTP 直播流。 |
| `//STREAM OFF` | 关闭 HTTP 直播流。 |
| `//COOKIE netease <cookie>` | 动态更新网易云 Cookie，并持久化到数据库。 |
| `//COOKIE bilibili <sessdata>` | 动态更新 Bilibili SESSDATA，并持久化到数据库。 |

聊天框命令：

```text
//stream
```

当直播流已开启时，用户可在系统消息中获得自己的 `/radio/stream?key=...` 收听链接。

> 直播流依赖 FFmpeg，会增加 CPU、内存和公网流量消耗。Bilibili 音频会经过本地缓存和服务端转发，也会消耗 VPS 流量。

## Navidrome 可选实验功能

Navidrome 用于接入私有曲库，不影响默认部署。启用示例：

```bash
docker compose -f docker-compose.yml -f docker-compose.navidrome.yml --env-file .env.navidrome up -d
```

访问边界：

- 只对白名单中的账号开放；推荐使用账号用户名或稳定 `publicId`，改昵称不会影响授权。
- 用户名白名单是轻量房间信任模型，不是强身份认证。
- Navidrome 音频通过 MusicParty 后端代理给浏览器，Navidrome 凭据不会直接暴露给前端。
- 当前版本的 HTTP 直播流不支持 Navidrome 曲目，Navidrome 主要用于浏览器播放。

详细部署和 rclone 挂载说明见 [docs/navidrome-rclone.md](docs/navidrome-rclone.md)。

## 管理与设置中心

设置中心集中放置正式运行时配置：

- **个人资料**：查看账号、publicId、最近登录、平台绑定、喜欢歌曲/个人歌单统计，修改显示名或密码。
- **房间管理**：创建公开/私密房间，房主或管理员可编辑房间名称、密码和可见性，删除房间时会迁移在线用户并清理房间数据。
- **音源管理**：维护网易云/Bilibili Cookie、Subsonic/Navidrome 源、本地媒体库上传权限和授权用户。
- **布局与显示**：切换主题、移动端预览、移动播放页密度、桌面主舞台比例和全局缩放。

## 本地开发

### Git Bash 一键启动

Windows 本地推荐使用 Git Bash：

```bash
./start-dev.sh --start-netease-api
```

本地启动脚本默认会尝试展示 Navidrome 源：`NAVIDROME_ENABLED=true`、`NAVIDROME_BASE_URL=http://127.0.0.1:4533`、`NAVIDROME_ALLOWED_USERS=*`。Navidrome 鉴权可放在 `.env.local`、环境变量或 `cookies.json`。临时关闭可用：

```bash
./start-dev.sh --no-navidrome
```

默认启动：

```text
后端：http://localhost:8080
前端：http://127.0.0.1:5173
网易云 API：http://127.0.0.1:3000
```

Windows fresh start 可先清理 8080/5173/3000 上的旧 Java/Node/Vite 进程，再启动同一套开发服务：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\fresh-start.ps1 -StartNeteaseApi
```

只启动后端和前端：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\fresh-start.ps1
```

移动端预览：

```text
http://127.0.0.1:5173/?mobilePreview=1
```

如果你使用本地 Navidrome：

```bash
./start-dev.sh --navidrome-local
```

### Cookie 配置

本地调试可以把 Cookie 放在忽略提交的 `cookies.json`：

```json
{
  "neteaseCookie": "MUSIC_U=xxxx...; __csrf=xxxx...",
  "bilibiliSessdata": "",
  "navidrome": {
    "baseUrl": "http://127.0.0.1:4533",
    "username": "admin",
    "password": "secret",
    "allowedUsers": "*"
  }
}
```

也可以通过环境变量传入：

```bash
export NETEASE_COOKIE="MUSIC_U=xxxx...; __csrf=xxxx..."
./start-dev.sh --start-netease-api
```

运行中也可以用管理员命令动态更新：

```text
//COOKIE netease MUSIC_U=xxxx...; __csrf=xxxx...
```

### 手动启动

前端：

```bash
cd music-party-web
npm install
npm run dev
```

后端：

```bash
mvn spring-boot:run
```

生产构建：

```bash
cd music-party-web
npm run build
cd ..
mvn clean package -DskipTests
```

前端提交前推荐至少执行：

```bash
cd music-party-web
npm run lint
npm run test:run
npm run build
```

## 技术栈

- 后端：Java 21、Spring Boot 3.2、WebSocket/STOMP、WebFlux、FFmpeg
- 前端：Vue 3、Vite 7、Pinia、Tailwind CSS、vue-i18n、lucide-vue-next、Material Symbols 本地字体
- 部署：Docker、Docker Compose，可选 Cloudflare Tunnel、Navidrome、rclone

## 免责声明

- 本项目仅供学习交流和个人自托管使用，请勿用于商业用途。
- 网易云音乐、Bilibili、Navidrome/Subsonic 等接口能力取决于第三方服务和账号状态，项目不保证可用性。
- Bilibili 与网易云相关 Cookie/Sessdata 请自行妥善保管，不要提交到仓库。
- 请尊重版权，支持正版音乐。

## License

上游 README 标注为 MIT License。当前仓库根目录暂未包含独立 `LICENSE` 文件，正式发布前建议补齐许可证文件以避免分发歧义。
