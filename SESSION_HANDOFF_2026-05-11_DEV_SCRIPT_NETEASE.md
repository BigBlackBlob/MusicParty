# MusicParty Session Handoff - 2026-05-11 Dev Script / Netease API

## 项目位置

- 项目根目录：`D:\Pluvllter-MusicParty\MusicParty-master`
- 前端目录：`D:\Pluvllter-MusicParty\MusicParty-master\music-party-web`
- 后端：Spring Boot 3.2.5 / Java 21
- 前端：Vue 3 / Vite / Pinia / Tailwind CSS

---

## 本轮主要目标

本轮从主播放页面 UI 精修继续推进到本地调试体验：

1. 继续精修主播放页面 `CenterConsole.vue` 的歌词排版和无歌词场景。
2. 提升浅色模式下错误提示的可读性。
3. 给项目补非 Docker 本地调试脚本。
4. 支持本地读取 `cookies.json` 注入网易云 Cookie。
5. 增强前后端报错留痕，方便排查搜索/播放失败。
6. 排查本地 NeteaseCloudMusicApi 与后端连接失败问题。
7. 用户最终决定后续优先手动打开 Git Bash 运行脚本，不再继续折腾 `.cmd` / PowerShell 桥接。

---

## 已完成的 UI 相关工作

### 主播放页 / 歌词

主要文件：

- `music-party-web/src/components/CenterConsole.vue`

已完成方向：

- 主播放页面已经从原来的 HUD / 控制台风格转向更干净的 Apple Music 式主舞台。
- 歌词区域改为直接排版展示，不再套玻璃卡片。
- 有歌词时突出当前行，并降低前后行权重。
- 无歌词场景做了更安静的空状态处理，避免页面显得残缺。
- 保留封面、点赞交互、播放状态等既有逻辑。

### 错误提示

涉及文件：

- `music-party-web/src/components/ToastNotification.vue`
- `music-party-web/src/utils/errors.js`
- `music-party-web/src/composables/useSearchLogic.js`
- `music-party-web/src/composables/usePlaylistLogic.js`
- `music-party-web/src/components/SearchModal.vue`

已完成方向：

- 修正浅色模式下错误提示文字对比度过低的问题。
- 前端错误展示增强，尽量把后端返回的 `message` / `error` / response body 信息带出来。
- 搜索失败时不再只显示过短、难以记录的提示。

验证：

- 前端 `npm run build` 曾通过。

---

## 已完成的后端日志增强

涉及文件：

- `src/main/java/org/thornex/musicparty/controller/ApiController.java`
- `src/main/java/org/thornex/musicparty/exception/GlobalExceptionHandler.java`
- `src/main/resources/application.yml`

已完成方向：

- 搜索 API 增加请求日志，例如平台和关键词长度。
- 搜索失败时记录平台、关键词和完整异常栈。
- 全局异常处理增加详细异常日志。
- `application.yml` 中日志包名修正为 `org.thornex.musicparty`，避免旧包名导致 DEBUG 日志不生效。

验证：

- 后端 `mvn -q -DskipTests compile` 曾通过。

---

## Cookie 配置现状

新增或调整文件：

- `.gitignore`
- `cookies.example.json`
- 项目根目录预期本地私有文件：`cookies.json`

当前本地调试 Cookie 格式：

```json
{
  "neteaseCookie": "MUSIC_U=xxxx...; __csrf=xxxx...",
  "bilibiliSessdata": ""
}
```

约定：

- `cookies.json` 放在项目根目录。
- `cookies.json` 已加入 `.gitignore`，不应提交。
- 启动脚本读取 `cookies.json` 后设置：
  - `NETEASE_COOKIE`
  - `BILIBILI_SESSDATA`

已确认：

- 后端日志曾显示 `Netease Cookie configured.`
- 后端启动日志曾显示 `effective-netease-cookie-length: 2120`
- 因此 Cookie 注入链路已经生效。

---

## Netease API 排查结论

用户在页面内搜索歌曲时报：

```text
An unexpected internal server error occurred.
```

后端日志中的真实原因是：

```text
WebClientRequestException: Connection refused: getsockopt: /127.0.0.1:3000
Request to GET http://127.0.0.1:3000/search
```

结论：

- 后端已经收到前端搜索请求。
- 后端已经按 `NETEASE_API_URL=http://127.0.0.1:3000` 去请求本地 NeteaseCloudMusicApi。
- 失败原因不是 Cookie，也不是后端搜索逻辑本身。
- 失败原因是本地 `NeteaseCloudMusicApi` 当时没有在 `127.0.0.1:3000` 监听。
- 用户手动打开 Git Bash，在项目目录运行 `./start-dev.sh --start-netease-api` 后确认功能正常。

---

## 本轮恢复的 Git Bash 脚本

用户误删 Bash 启动脚本后，已重新创建：

- `start-dev.sh`
- `start-netease-api.sh`

### 推荐启动方式

用户当前明确倾向：

- 手动打开 Git Bash。
- 进入项目根目录。
- 直接运行 Bash 脚本。
- 不再通过 `.cmd` 或 PowerShell 间接调用。

推荐命令：

```bash
cd /d/Pluvllter-MusicParty/MusicParty-master
./start-dev.sh --start-netease-api
```

如果提示权限问题：

```bash
chmod +x start-dev.sh start-netease-api.sh
```

### `start-dev.sh` 当前行为

当前脚本设计为纯 Git Bash 入口：

- 读取项目根目录 `cookies.json`。
- 默认设置 `NETEASE_API_URL=http://127.0.0.1:3000`。
- 使用 `--start-netease-api` 时启动本地 `NeteaseCloudMusicApi`。
- 等待 `127.0.0.1:3000` 端口打开。
- 启动 Spring Boot 后端。
- 启动 Vite 前端。
- 所有子进程运行在同一个 Git Bash 会话内。
- 按 `Ctrl+C` 停止由脚本启动的子进程。

默认地址：

```text
Netease API: http://127.0.0.1:3000
Backend:     http://127.0.0.1:8080
Frontend:    http://127.0.0.1:5173
```

固定日志文件：

```text
.dev-logs/netease-api.log
.dev-logs/backend-dev.log
.dev-logs/frontend-dev.log
```

### `start-netease-api.sh` 当前行为

可单独运行：

```bash
./start-netease-api.sh
```

行为：

- 读取 `cookies.json` 中的 `neteaseCookie`。
- 设置 `PORT=3000`。
- 设置 `NETEASE_COOKIE`。
- 执行：

```bash
npx -y NeteaseCloudMusicApi@latest
```

输出写入：

```text
.dev-logs/netease-api.log
```

---

## 为什么不再继续 `.cmd` / PowerShell 桥接

本轮多次尝试让 Windows `.cmd` 或 PowerShell 间接调用 Bash 脚本，但出现过：

- `--start-netease-api` 参数被吞。
- 新开的 cmd 窗口瞬间关闭，无法保留错误。
- Codex 当前 PowerShell 环境内 `bash` 不在 PATH，但用户手动打开 Git Bash 可以正常执行。

最终结论：

- 后续本地调试优先使用手动 Git Bash。
- 不建议在短期内继续投入 `.cmd` 包装脚本，性价比低。
- 这不会影响 Docker 打包部署。Docker 依赖 `Dockerfile` / `docker-compose.yml` / 应用环境变量，不依赖本地 `start-dev.sh`。

---

## Docker 部署风险评估

当前 Bash 脚本属于本地开发辅助，不参与 Docker 构建链路。

因此：

- 手动 Git Bash 启动不会影响 Docker 重新打包。
- `cookies.json` 不会进入镜像，前提是 Docker build context 没有强行复制它，且 `.dockerignore` / 构建步骤没有覆盖该约定。
- Docker 环境下仍应通过 `docker-compose.yml` 或部署平台环境变量设置：
  - `NETEASE_API_URL`
  - `NETEASE_COOKIE`
  - `NETEASE_QUALITY`

注意：

- Docker Compose 中 `NETEASE_API_URL` 默认应指向容器服务名，例如 `http://netease-api:3000`。
- 本地 Git Bash 脚本中默认是 `http://127.0.0.1:3000`。
- 两者是不同运行环境下的合理配置，不要混用。

---

## 当前未验证点

由于 Codex 所在 PowerShell 环境中 `bash` 不在 PATH，本轮没有在 Codex 里执行：

```bash
bash -n start-dev.sh
bash -n start-netease-api.sh
```

但用户此前已确认手动 Git Bash 方式可以正常启动项目。

另外，当前目录没有 `.git`：

```text
D:\Pluvllter-MusicParty\MusicParty-master\.git 不存在
```

所以本轮无法使用 `git status` 做最终变更核对。

---

## 下个会话建议优先事项

1. 先让用户手动 Git Bash 跑一次：

```bash
./start-dev.sh --start-netease-api
```

2. 若仍报错，优先查看：

```text
.dev-logs/netease-api.log
.dev-logs/backend-dev.log
.dev-logs/frontend-dev.log
```

3. 如果 Netease API 端口未开，先单独运行：

```bash
./start-netease-api.sh
```

4. 如果搜索正常，继续回到主播放页视觉打磨：

- 歌词字号、行距、当前行动效。
- 无歌词空状态文案与布局。
- 浅色 / 深色模式一致性。
- 搜索弹窗错误信息展示尺寸和可复制性。

5. 如果准备 Docker 部署，再单独检查：

- `docker-compose.yml` 的 `NETEASE_COOKIE`
- `NETEASE_API_URL=http://netease-api:3000`
- 是否需要 `.dockerignore` 明确排除 `cookies.json`

---

## 给下个 Agent 的关键判断

- 不要把 Cookie 注入当作主要问题，当前证据显示 Cookie 已生效。
- 不要优先改后端 Netease 搜索逻辑，已观察到失败点是连接不到 `127.0.0.1:3000`。
- 不要继续优先修 `.cmd` 启动器，用户已决定手动 Git Bash。
- 若要验证前端完整功能，先保证 `NeteaseCloudMusicApi` 正在监听 3000。
- 本地脚本和 Docker 部署配置是两条独立链路，避免互相污染。
