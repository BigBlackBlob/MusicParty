# MusicParty NRT Fork

## English

MusicParty is a shared music party web app. Users can join the same room, search and queue songs, chat, react to the current track, and listen through the web player or the optional HTTP live stream endpoint.

This repository is a personal fork based on the upstream MusicParty project, with extra work focused on UI polish, accessibility, queue ergonomics, album import, playback control, and streaming stability.

### Fork Improvements

- Refined responsive UI and mobile navigation.
- Improved accessibility with labels, alt text, focus states, live regions, and larger touch targets.
- Reworked Chinese UI copy to remove old console/HUD-style wording.
- Added NetEase album search and full-album import.
- Added seek permission control: only the user who queued the current song can drag the progress bar.
- Improved player synchronization by sending server timestamps and calculating progress on a server-aligned time axis.
- Reduced unnecessary ffmpeg restarts in the live stream path.
- Added Docker Compose local build support so deployments can build from the current fork source.

### Docker Deployment

```bash
docker compose up -d --build
```

The app is exposed by default at:

```text
http://localhost:8848
```

Before production deployment, update `docker-compose.yml` environment variables such as `ADMIN_PASSWORD`, `BASE_URL`, `NETEASE_COOKIE`, and `BILIBILI_SESSDATA`.

### Local Development

Backend:

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

Frontend:

```bash
cd music-party-web
npm install
npm run build
```

## 中文

MusicParty 是一个多人共享音乐派对 Web 应用。用户可以加入同一个房间，搜索并点歌、查看队列、聊天、点赞当前歌曲，并通过网页播放器或可选的 HTTP 直播流收听音乐。

本仓库是基于上游 MusicParty 项目的个人 fork，主要围绕界面体验、可访问性、队列操作、网易云专辑导入、播放控制和音频推流稳定性进行了改进。

### Fork 改进

- 优化响应式界面和移动端底部快捷入口。
- 补充可访问性细节，包括 label、alt、focus 状态、live region 和更大的触控区域。
- 将旧控制台/HUD 风格文案调整为更自然的中文表达。
- 新增网易云专辑搜索和整张专辑导入。
- 新增进度拖拽权限控制：只有当前歌曲的点播者可以调整进度。
- 改进播放同步：后端下发服务端时间戳，前端按服务端时间轴推算进度。
- 优化直播流路径，减少普通状态广播导致的 ffmpeg 无意义重启。
- 为 Docker Compose 增加本地 build 支持，方便直接从当前 fork 源码部署。

### Docker 部署

```bash
docker compose up -d --build
```

默认访问地址：

```text
http://localhost:8848
```

正式部署前建议修改 `docker-compose.yml` 中的 `ADMIN_PASSWORD`、`BASE_URL`、`NETEASE_COOKIE`、`BILIBILI_SESSDATA` 等环境变量。

### 本地开发

后端：

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

前端：

```bash
cd music-party-web
npm install
npm run build
```
