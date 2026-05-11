# MusicParty Detailed Project Roadmap

本文档是在 `AGENT_HANDOFF_PLAN.md` 基础上继续细化的执行规划，目标是让后续 agent 可以直接按阶段拆任务、评估风险、开始实现。

项目目录：

- `D:\Pluvllter-MusicParty\MusicParty-master`

当前产品方向：

- 保持现有多人房间听歌 UX。
- 增加受权限控制的全员同步 seek。
- UI 从当前工业/医疗风格改为更接近 Spotify / Apple Music 的现代音乐产品风格。
- 避免任意直链音频方案的安全和兼容风险，优先探索 Navidrome 作为稳定媒体库音源。
- 支持 Windows 本地调试，不依赖 WSL2。

---

## 0. Current Architecture Snapshot

### Frontend

- Vue 3
- Vite
- Pinia
- Tailwind CSS
- STOMP WebSocket
- lucide-vue-next icons

关键文件：

- `music-party-web/src/App.vue`
- `music-party-web/src/components/layout/MainLayout.vue`
- `music-party-web/src/components/PlayerControl.vue`
- `music-party-web/src/components/AudioEngine.vue`
- `music-party-web/src/components/SearchModal.vue`
- `music-party-web/src/composables/useAudio.js`
- `music-party-web/src/stores/player.js`
- `music-party-web/src/stores/user.js`
- `music-party-web/src/constants/api.js`
- `music-party-web/src/services/socket.js`
- `music-party-web/src/services/socketHandler.js`

### Backend

- Spring Boot 3.2.5
- Java 21
- Spring WebSocket / STOMP
- Spring WebFlux `WebClient`
- FFmpeg live stream relay

关键文件：

- `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`
- `src/main/java/org/thornex/musicparty/controller/ApiController.java`
- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- `src/main/java/org/thornex/musicparty/service/MusicQueueManager.java`
- `src/main/java/org/thornex/musicparty/service/api/IMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/api/NeteaseMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/api/BilibiliMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/stream/LiveStreamService.java`
- `src/main/java/org/thornex/musicparty/config/AppProperties.java`
- `src/main/resources/application.yml`

---

## 1. Milestone Overview

建议分成 4 个里程碑推进。

### Milestone 1: Owner-only Seek

目标：

- 当前曲目点歌人可以拖动进度条。
- 松手后所有人跳转到同一时间。
- 非点歌人不能 seek。
- 不引入管理员角色系统。

优先级：

- P0

预计风险：

- 中低。涉及前后端协议和播放同步，但模型清晰。

### Milestone 2: Windows Local Debug Workflow

目标：

- 明确 Windows 本地调试依赖、启动方式和环境变量。
- 不依赖 WSL2。
- 可以稳定启动前端、后端，并完成基础播放/同步调试。

优先级：

- P0

预计风险：

- 低。主要是文档和环境验证。

### Milestone 3: UI Visual Redesign, Batch 1

目标：

- 保持现有 UX。
- 先改第一视觉层：
  - 主布局
  - 底部播放器
  - 搜索弹窗
- 风格靠近 Spotify / Apple Music：更现代、更干净、更统一。

优先级：

- P1

预计风险：

- 中。设计质量依赖一致的 token 和组件规则。

### Milestone 4: Navidrome Provider Phase 1

目标：

- 新增 `navidrome` 音源 provider。
- 支持搜索、入队、播放、封面。
- 复用现有同步播放和队列机制。

优先级：

- P1/P2

预计风险：

- 中。需要验证 Navidrome API 认证、stream URL、FFmpeg 兼容性。

---

## 2. Milestone 1: Owner-only Seek

### 2.1 Product Behavior

最终行为：

- 所有人都能看到进度条。
- 只有当前曲目的点歌人可以拖动进度条。
- 拖动过程中只在本地预览时间。
- 松手时发送一次 seek 指令。
- 后端校验权限后更新全局播放时间锚点。
- 后端广播新 player state，所有客户端按新时间同步。
- 非点歌人尝试 seek 时后端拒绝。

权限规则：

- allowed:
  - `currentEnqueuerId == currentUser.token`
  - `sessionId == "SYSTEM"` 或后端内部系统调用
- denied:
  - guest
  - 非点歌人
  - 无当前播放曲目

注意：

- 第一版不做管理员手动 seek。
- 当前应用主要是 PUBLIC 房间，管理员 tag 不应成为第一版权限核心。

### 2.2 Backend Tasks

#### Task 1: Add SeekRequest DTO

新增文件：

- `src/main/java/org/thornex/musicparty/dto/SeekRequest.java`

建议结构：

```java
package org.thornex.musicparty.dto;

public record SeekRequest(long positionMs) {}
```

验收标准：

- DTO 能被 STOMP payload 反序列化。

#### Task 2: Add WebSocket Destination

修改文件：

- `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`

新增 mapping：

```java
@MessageMapping("/control/seek")
public void seek(@Payload SeekRequest request, @Header("simpSessionId") String sessionId) {
    if (isGuest(sessionId)) return;
    musicPlayerService.seekTo(request.positionMs(), sessionId);
}
```

验收标准：

- 客户端可以发送 `/app/control/seek`。
- guest 不能触发 seek。
- 非 guest 请求进入 `MusicPlayerService`。

#### Task 3: Add Seek Logic in MusicPlayerService

修改文件：

- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`

新增方法建议：

```java
public void seekTo(long positionMs, String sessionId) {
    // validate current music
    // validate owner permission
    // clamp position
    // update anchors
    // broadcast state
}
```

核心逻辑：

1. `currentMusic.get()` 为空则 return。
2. 如果 `sessionId != "SYSTEM"`：
   - 通过 `userService.getUser(sessionId)` 找 user。
   - user 不存在则 return。
   - user token 必须等于 `currentEnqueuerId.get()`。
3. 将目标进度 clamp 到 `[0, duration - 500]` 或 `[0, duration]`。
4. 更新：
   - `positionAnchor.set(clampedPosition)`
   - `timestampAnchor.set(System.currentTimeMillis())`
5. 保持当前 pause 状态不变。
6. `broadcastFullPlayerState()`。
7. 可选：发布 system/player event 给 seek 发起者或全体。

验收标准：

- 点歌人 seek 成功。
- 非点歌人 seek 不改变后端播放位置。
- seek 后正在播放状态继续播放。
- seek 后暂停状态仍保持暂停。
- seek 到接近结尾不会导致异常或负数。

#### Task 4: Decide Whether Seek Shares Global Cooldown

当前 `pause/skip/shuffle` 有全局控制冷却。

建议：

- 第一版 seek 使用独立较短冷却，例如 `500ms` 或 `1000ms`。
- 但由于前端只在松手发送一次，后端冷却主要防异常请求。

推荐实现：

- 复用 `isRateLimited(sessionId)`，简单但会影响点歌人连续微调。
- 或新增 `lastSeekTimestamp`，更清晰。

建议选择：

- 第一版复用 `isRateLimited`，实现简单。
- 如果用户觉得拖拽后连续微调手感差，再拆独立冷却。

### 2.3 Frontend Tasks

#### Task 1: Add WS Destination

修改文件：

- `music-party-web/src/constants/api.js`

新增：

```js
PLAYER_SEEK: '/app/control/seek',
```

验收标准：

- store 可以引用 `WS_DEST.PLAYER_SEEK`。

#### Task 2: Add Store Action

修改文件：

- `music-party-web/src/stores/player.js`

新增：

```js
const seek = (positionMs) =>
  requireAuth() && checkCooldown() && socketService.send(WS_DEST.PLAYER_SEEK, { positionMs });
```

导出 `seek`。

注意：

- 前端 cooldown 是用户体验层，不能替代后端鉴权。
- 如果 seek 和 pause 共用 `checkCooldown`，可能导致刚拖完不能立刻暂停。可以先接受，后续再优化。

#### Task 3: Add Progress Drag State

修改文件：

- `music-party-web/src/components/PlayerControl.vue`

新增状态：

- `progressTrackRef`
- `isDraggingProgress`
- `dragProgressMs`

新增 computed：

- `canSeek`
- `displayProgressMs`
- `displayProgressPercent`

`canSeek` 建议：

```js
const canSeek = computed(() => {
  return !!nowPlaying.value
    && !player.isLoading
    && !player.isErrorState
    && nowPlaying.value.enqueuedById === userStore.userToken;
});
```

注意：

- 需要引入 `useUserStore`。
- 如果 `nowPlaying.enqueuedById` 字段路径实际为 `nowPlaying.value.enqueuedById`，按当前 DTO 即可。

#### Task 4: Pointer/Touch Drag

建议使用 Pointer Events，不分 mouse/touch：

- `@pointerdown`
- `window.addEventListener('pointermove', ...)`
- `window.addEventListener('pointerup', ...)`

交互：

1. pointerdown:
   - 如果 `!canSeek` return。
   - 设置 `isDraggingProgress = true`。
   - 根据 track rect 计算目标 ms。
2. pointermove:
   - 如果 dragging，更新 `dragProgressMs`。
3. pointerup:
   - 如果 dragging，调用 `player.seek(dragProgressMs)`。
   - 清理事件。

验收标准：

- 桌面鼠标可拖。
- 移动端触控可拖。
- 非点歌人无法拖。
- 拖动不改变布局高度。
- 拖动过程中时间显示跟随预览。

#### Task 5: Coordinate with useAudio Correction

修改文件：

- `music-party-web/src/composables/useAudio.js`
- `music-party-web/src/stores/player.js`

建议在 player store 增加：

- `isSeekingPreview`

拖拽时：

- `player.isSeekingPreview = true`

松手/取消：

- `player.isSeekingPreview = false`

`useAudio.js` 同步循环中：

- 如果 `playerStore.isSeekingPreview`，不要覆盖 `localProgress`，也不要纠偏 `audio.currentTime`。

验收标准：

- 拖拽时进度不会被 200ms sync loop 拉回。
- 松手后等后端广播恢复权威同步。

### 2.4 Testing Plan

手动测试：

1. 打开两个浏览器窗口。
2. 用户 A 点歌。
3. 用户 A 拖动进度条，A/B 同步跳转。
4. 用户 B 尝试拖动，B 本地不应触发 seek。
5. 如果绕过前端发 STOMP seek，后端也应拒绝。
6. 播放中 seek 后继续播放。
7. 暂停中 seek 后仍暂停，但位置变化。
8. 接近歌曲结尾 seek 不崩溃。

可选自动测试：

- 后端 `MusicPlayerService.seekTo` 单元测试。
- 权限判断测试。

---

## 3. Milestone 2: Windows Local Debug Workflow

### 3.1 Goal

建立一套不依赖 WSL2 的本地调试方式。

### 3.2 Required Tools

必须：

- JDK 21
- Node.js 18+
- npm
- 项目自带 Maven Wrapper 或本地 Maven

可选但推荐：

- FFmpeg in PATH
- Navidrome for stable local media source
- NeteaseCloudMusicApi for existing netease path

### 3.3 Suggested Environment Variables

本地最小：

```powershell
$env:ADMIN_PASSWORD="admin123"
$env:BASE_URL="http://localhost:8080"
$env:NETEASE_API_URL="http://localhost:3000"
```

如果测试 FFmpeg live stream：

```powershell
$env:APP_MUSIC_API_FFMPEG_PATH="ffmpeg"
```

注意：

- 当前 `AppProperties` 有 `ffmpegPath` 字段，但 `application.yml` 没显式映射该字段。
- Spring Boot relaxed binding 通常可通过环境变量绑定，但建议后续 agent 验证。
- 如果绑定不工作，可以在 `application.yml` 增加：

```yaml
ffmpeg-path: ${FFMPEG_PATH:ffmpeg}
```

放在 `app.music-api` 下。

### 3.4 Run Commands

后端：

```powershell
cd D:\Pluvllter-MusicParty\MusicParty-master
.\mvnw.cmd spring-boot:run
```

前端：

```powershell
cd D:\Pluvllter-MusicParty\MusicParty-master\music-party-web
npm install
npm run dev
```

### 3.5 Validation Checklist

1. 后端启动在 `http://localhost:8080`。
2. 前端 Vite 启动成功。
3. `/api/config` 返回配置。
4. WebSocket `/ws` 可连接。
5. 两个浏览器窗口可看到同一 player state。
6. 搜索功能根据配置可用或明确报错。
7. FFmpeg 不在 PATH 时，普通网页播放不受影响。

### 3.6 Deliverable

建议新增：

- `WINDOWS_DEV_SETUP.md`

内容：

- 工具安装
- 环境变量
- 启动命令
- 常见问题
- 可选 Navidrome 调试方式

---

## 4. Milestone 3: UI Visual Redesign Batch 1

### 4.1 Scope

第一批只改：

- `MainLayout.vue`
- `PlayerControl.vue`
- `SearchModal.vue`
- 可能同步调整：
  - `tailwind.config.js`
  - `style.css`

不改：

- 播放逻辑
- 搜索逻辑
- 队列排序逻辑
- 聊天逻辑
- WebSocket 协议

### 4.2 Design Direction

目标产品感：

- Spotify desktop / Apple Music desktop 的成熟音乐产品感。
- 暗色主界面为主。
- 封面和内容层级更突出。
- 控件更克制。
- 边框减少，层级依靠 surface / shadow / opacity。

避免：

- 大量工业风切角。
- 高对比橙色到处出现。
- 过多边框。
- 过强 decorative 文案。
- 大面积单一色调导致沉闷。

### 4.3 Proposed Design Tokens

建议新 token：

- `surface.base`: app background
- `surface.panel`: sidebars / queue / modal
- `surface.raised`: player / floating overlay
- `text.primary`
- `text.secondary`
- `text.muted`
- `accent.primary`
- `accent.soft`
- `border.subtle`

可能色彩方向：

- base: near black / charcoal
- panel: dark gray
- raised: slightly lighter gray
- accent: green or red/pink, but use sparingly

注意：

- 不建议完全模仿 Spotify green。可以用更克制的 accent。
- Apple Music 的 red/pink accent 也可以，但不要做大面积粉红。

### 4.4 Component Plan

#### MainLayout

目标：

- Header 更像 app chrome。
- 左侧用户栏和右侧队列栏统一 surface。
- 中央 stage 减少杂乱背景。
- 移动端 drawer 保持现有逻辑。

验收标准：

- 桌面三栏结构仍可用。
- 移动端用户/队列入口仍可用。
- 精简模式仍可切换。
- 搜索按钮仍明显。

#### PlayerControl

目标：

- 更接近成熟音乐播放器底栏。
- 封面、歌曲名、artist、时间、控制区层级清楚。
- 进度条更易拖拽，hit area 变大，视觉轨道不必过粗。
- 音量控制更统一。

验收标准：

- 桌面布局不拥挤。
- 移动端按钮不重叠。
- 长歌名不撑破。
- seek 权限态有清楚视觉差异。

#### SearchModal

目标：

- 从工业弹窗改成音乐库搜索 overlay。
- 平台切换更像 segmented control。
- 搜索结果更像音乐列表。
- 保留用户歌单/搜索结果现有 UX。

验收标准：

- 平台切换仍可用。
- 搜索、绑定用户、歌单导入仍可用。
- 移动端 playlist/songs 双视图仍可用。

### 4.5 Whether to Add shadcn-vue / Reka UI

建议：

- Batch 1 不急着引入新依赖。
- 先用 Tailwind 重建 visual system。
- 如果后续 modal、tooltip、tabs、slider 需要更强 a11y，再引入 Reka UI/shadcn-vue。

理由：

- 当前项目组件量不大。
- 先引入库可能把任务从视觉统一变成依赖迁移。
- Seek slider 可以先手写，因为已有音量拖拽模式。

### 4.6 UI Acceptance Tests

手动检查：

1. Desktop 1440x900。
2. Desktop 1920x1080。
3. Mobile 390x844。
4. Mobile 430x932。
5. 长歌名、长 artist。
6. 无当前播放。
7. 断线态。
8. loading/buffering/error。
9. 搜索弹窗打开/关闭。
10. 队列为空/队列很长。

---

## 5. Milestone 4: Navidrome Provider Phase 1

### 5.1 Goal

新增一个稳定本地/私有媒体库音源：

- `platform = navidrome`
- 搜索歌曲
- 点歌入队
- 播放音频
- 显示封面
- 复用现有同步逻辑

### 5.2 Non-goals for Phase 1

暂不做：

- Jellyfin
- WebDAV
- 多 Navidrome 服务器
- 用户级 Navidrome 账号绑定
- 歌单导入
- 专辑浏览
- 艺术家页
- 智能推荐

### 5.3 Backend Design

新增服务：

- `src/main/java/org/thornex/musicparty/service/api/NavidromeMusicApiService.java`

实现：

- `IMusicApiService`

新增配置：

- `AppProperties.NavidromeApiConfig`

建议字段：

```java
private NavidromeApiConfig navidrome;

public static class NavidromeApiConfig extends ApiConfig {
    private boolean enabled = false;
    private String username;
    private String password;
    private String salt;
}
```

但最终字段取决于选用的 Navidrome/Subsonic 认证方式。

### 5.4 API Method Mapping

`searchMusic(keyword)`:

- 调 Navidrome/Subsonic search endpoint。
- 映射到 `Music`：
  - `id`
  - `title`
  - `artist`
  - `durationMs`
  - `platform = "navidrome"`
  - `coverUrl`

`getPlayableMusic(musicId)`:

- 获取详情或直接构建 stream URL。
- 返回 `PlayableMusic`：
  - `url` 为可播放 stream URL。
  - `duration` 必须可靠。
  - `needsProxy` 第一版建议为 `false`，除非浏览器播放或 FFmpeg 遇到 auth/header 问题。

`getLyric(musicId)`:

- Phase 1 可返回 `Mono.just("")`。

`getUserPlaylists/searchUsers/getPlaylistMusics`:

- Phase 1 可以先返回空列表。
- 或只实现 `getPlaylistMusics` 如果 Navidrome playlist 很容易接。

### 5.5 Frontend Changes

修改：

- `SearchModal.vue`

当前平台列表写死：

```vue
v-for="p in ['netease', 'bilibili']"
```

Phase 1 可改为：

```js
const platforms = ['netease', 'bilibili', 'navidrome'];
```

更好方案：

- `/api/config` 返回 enabled platforms。
- 前端动态渲染。

推荐第一版：

- 如果后端 Navidrome 未配置，不应该展示 navidrome tab。
- 所以建议扩展 `/api/config`：
  - `platforms: ["netease", "bilibili", "navidrome"]`

需要注意：

- 当前 `/api/config` 返回 `Map<String, String>`，如果要返回数组，需要改 DTO 或 `Map<String, Object>`。

### 5.6 Validation Items

必须验证：

1. Navidrome stream URL 能否被浏览器 `<audio>` 直接播放。
2. Navidrome stream URL 能否被 FFmpeg 直接读取。
3. 是否需要认证 header。
4. 是否需要后端代理 stream。
5. duration 是否毫秒级可靠。
6. cover art 是否需要代理。

如果需要后端代理：

- 新增 `/api/navidrome/stream/{id}` 或通用 `/proxy/navidrome/...`。
- 需要避免开放任意代理能力。

### 5.7 Risk Register

风险 1：认证方式不清晰。

- 缓解：先用服务端统一配置账号，不做用户绑定。

风险 2：stream URL 带认证参数，暴露给前端。

- 缓解：本地/私有部署可接受；生产可改后端代理。

风险 3：FFmpeg 读取失败。

- 缓解：在 `LiveStreamService` 中对 navidrome 增加必要 headers 或代理。

风险 4：搜索结果和当前 UI 模型不完全匹配。

- 缓解：Phase 1 只做歌曲搜索，不做复杂 browse。

---

## 6. Dependency and Scope Rules

### Do Not Do in Early Phases

- 不重构整个播放器状态机。
- 不引入完整角色系统。
- 不把管理员权限扩展成复杂 ACL。
- 不把 UI 迁移到完整组件库。
- 不同时做 Navidrome 和 Jellyfin。
- 不做任意直链播放。
- 不做 WebDAV 首发。

### Keep Existing Behavior

- 队列公平随机逻辑。
- 现有搜索入队流程。
- 聊天和命令机制。
- STOMP 状态广播模式。
- FFmpeg live stream 的总体架构。

---

## 7. Acceptance Criteria Summary

### Owner-only Seek Done When

- 点歌人可拖动进度条。
- 非点歌人不能拖动，后端也拒绝伪造请求。
- seek 后所有客户端同步到同一位置。
- pause/play 状态不被 seek 意外改变。
- 桌面和移动端都可用。

### Windows Local Debug Done When

- 文档说明安装依赖、环境变量、启动命令。
- 前端和后端可在 Windows 直接启动。
- 不需要 WSL2。
- FFmpeg/Navidrome/NeteaseCloudMusicApi 被标注为可选或增强项。

### UI Batch 1 Done When

- 主布局、底部播放器、搜索弹窗视觉统一。
- UX 没变。
- 移动端可用。
- 长文本不溢出。
- loading/error/empty/paused/playing 状态可读。

### Navidrome Phase 1 Done When

- 配置 Navidrome 后可搜索歌曲。
- 可入队。
- 可播放。
- 可显示封面。
- seek 和同步逻辑正常。
- 不配置 Navidrome 时不会破坏现有平台。

---

## 8. Decisions Needed from User

以下问题需要用户拍板。后续 agent 不应擅自把这些做成复杂方案。

### Decision A: Seek Rejection UX

非点歌人点击或拖动进度条时，前端怎么表现？

选项：

1. 静默不可拖，仅 cursor/opacity 表示不可操作。
2. 点击时 toast 提示“只有点歌人可以调整进度”。
3. 鼠标 hover 显示 tooltip，点击不 toast。

推荐：

- 选项 3。
- 理由：不打扰，规则也清楚。

### Decision B: Seek Cooldown

seek 后端冷却策略怎么定？

选项：

1. 复用现有全局控制冷却，简单。
2. seek 使用独立 500ms 冷却。
3. seek 不加冷却，只依赖前端松手发送一次。

推荐：

- 选项 2。
- 理由：不会影响 pause/skip，且能防异常刷请求。

### Decision C: Seek Near End Behavior

允许拖到歌曲最后一刻吗？

选项：

1. 允许拖到 `duration`。
2. 最大只能到 `duration - 500ms`。
3. 最大只能到 `duration - 2000ms`。

推荐：

- 选项 2。
- 理由：避免立刻触发结束逻辑，同时不明显影响用户体验。

### Decision D: UI Theme Base

第一版 UI 视觉底色选哪个？

选项：

1. 深色主界面，接近 Spotify。
2. 浅色主界面，接近 Apple Music light。
3. 自动深浅主题，但第一期工作量增加。

推荐：

- 选项 1。
- 理由：音乐房间、封面展示、播放器底栏更容易出效果，也更接近成熟音乐产品。

### Decision E: Accent Color

第一版 accent 颜色选哪个？

选项：

1. Spotify-like green。
2. Apple-like red/pink。
3. 保留当前 orange，但降低使用面积。
4. 使用中性 blue/cyan，避免太像某一家。

推荐：

- 选项 4 或 3。
- 理由：不要做成单纯仿站。若希望保留项目识别度，选 3。

### Decision F: UI Dependency Strategy

是否第一期引入 shadcn-vue / Reka UI？

选项：

1. 第一批不引入，先用 Tailwind 重做视觉。
2. 立即引入 shadcn-vue。
3. 立即引入 Reka UI primitives。

推荐：

- 选项 1。
- 理由：先减少变量，避免依赖迁移扩大范围。

### Decision G: Navidrome Exposure in UI

Navidrome tab 如何显示？

选项：

1. 总是显示，未配置时报错。
2. 后端 `/api/config` 返回 enabled platforms，前端动态显示。
3. 用前端环境变量控制显示。

推荐：

- 选项 2。
- 理由：配置源在后端，部署行为更一致。

### Decision H: Navidrome Credentials

Navidrome 第一版认证方式怎么做？

选项：

1. 服务端统一配置一个 Navidrome 账号，所有用户共享。
2. 每个用户自己绑定 Navidrome 账号。
3. 只支持无认证 Navidrome。

推荐：

- 选项 1。
- 理由：最符合当前房间模型，最少 UI 和权限复杂度。

### Decision I: Navidrome Stream URL Handling

Navidrome 音频流第一版怎么给前端？

选项：

1. 直接把 Navidrome stream URL 下发给前端。
2. 后端代理音频流，不暴露 Navidrome URL。
3. 先直接下发；如果浏览器或 FFmpeg 有问题，再加代理。

推荐：

- 选项 3。
- 理由：先验证最小闭环，再处理兼容和安全增强。

### Decision J: Windows Local Source Priority

Windows 本地调试优先保证哪个音源？

选项：

1. 先保证现有 NeteaseCloudMusicApi。
2. 先搭 Navidrome，作为后续主调试音源。
3. 两个都搭。

推荐：

- 选项 1，然后 Phase 2 加 Navidrome。
- 理由：先验证当前项目能跑，再引入新音源。

---

## 9. Recommended Answers

如果用户不想逐项选择，建议默认采用：

- A: 3
- B: 2
- C: 2
- D: 1
- E: 3
- F: 1
- G: 2
- H: 1
- I: 3
- J: 1

这组选择的特点：

- 保守。
- 实现范围可控。
- 不提前引入复杂角色、代理、组件库或多账号模型。
- 适合先把功能跑通，再迭代质量。

---

## 10. Suggested Next Agent Execution Order

1. 读取 `AGENT_HANDOFF_PLAN.md` 和本文件。
2. 先确认用户对 Decision A-J 的选择。
3. 实现 Milestone 1 owner-only seek。
4. 追加或更新 Windows 本地调试文档。
5. 做 UI Batch 1 设计草案，不直接大改全部组件。
6. UI Batch 1 完成后再开始 Navidrome API spike。

---

## 11. Notes for Future Implementation

- 当前项目根目录可能不是 git repository，不能假设 `git status` 可用。
- 不要用 destructive git 命令。
- 不要把管理员权限扩展成第一阶段目标。
- 不要让 Navidrome 控制播放时钟；MusicParty 后端仍然是同步权威。
- 所有全员播放控制都必须以后端状态为准，前端只是 UI 和请求发起者。
