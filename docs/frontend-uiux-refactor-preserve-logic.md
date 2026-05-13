# MusicParty 前端 UI/UX 重构：可保留核心逻辑说明

## 目的

本文档用于指导后续 agent 做 MusicParty 前端 UI/UX 重构。

本次重构的重点应放在布局系统、视觉层级、组件结构、响应式尺寸 contract 和 CSS 架构上。不要把工作扩大成播放业务、WebSocket 同步、队列业务或后端 API 的重写。

当前前端的主要问题不是业务内核不可用，而是 UI 层经过多轮局部修补后，缺少统一的布局基准和组件尺寸约束，导致不同设备上出现错位、不等宽、控件互相挤压等问题。

## 建议保留不动的核心逻辑

### `music-party-web/src/stores/player.js`

这是前端播放器和房间状态的核心 store，建议作为稳定业务 API 保留。

应保留：

- 播放状态：`nowPlaying`、`isPaused`、`isShuffle`、`isLoading`。
- 队列状态：`queue`。
- 锁定状态：`isPauseLocked`、`isSkipLocked`、`isShuffleLocked`。
- 播放同步状态：`remotePosition`、`lastSyncTime`、`serverClockOffset`、`stateVersion`、`playEpoch`。
- 控制动作：`playNext()`、`togglePause()`、`toggleShuffle()`、`seek()`。
- 队列动作：`enqueue()`、`enqueuePlaylist()`、`enqueueAlbum()`、`topSong()`、`removeSong()`、`topSongs()`、`removeSongs()`。
- 喜欢歌曲：`likedSongs`、`addLikedSong()`、`removeLikedSong()`、`isSongLiked()`。
- 同步逻辑：`syncState()`、`getCurrentProgress()`、`requestPing()`、`requestResync()`、`requestSyncRefresh()`。
- 歌词加载逻辑：监听当前歌曲变化后调用 lyric API。

新的 UI 组件应该只消费这些状态和动作，不应绕过 store 直接操作 WebSocket。

### `music-party-web/src/composables/useAudio.js`

这是浏览器 `<audio>` 播放与全局同步的核心逻辑，建议保留。

应保留：

- drift 修正。
- smooth seek。
- 后台/前台切换后的恢复。
- 网络恢复后的重连和 resync。
- wake lock。
- Media Session。
- audio error retry。
- `localProgress`、`isBuffering`、`isErrorState` 输出。

UI 重构时可以重做播放器控件，但不应重写这里的同步算法。

### `music-party-web/src/components/AudioEngine.vue`

建议继续保留一个全局 `AudioEngine`，负责挂载 `<audio>` 和连接 `useAudio()`。

可允许轻微调整：

- `<audio>` 的位置。
- Navidrome tokenized URL 的生成方式。
- 是否暴露更多调试状态。

不建议把 `<audio>` 分散到各个播放器 UI 组件里。

### `music-party-web/src/services/socket.js`

这是 STOMP 客户端封装，建议保留。

应保留：

- `connect()`。
- `send()`。
- `forceReconnect()`。
- `disconnect()`。

UI 组件不应直接实例化 STOMP client。

### `music-party-web/src/services/socketHandler.js`

这是 socket message 到 store/toast/chat 的分发层，建议保留。

应保留：

- player state sync。
- queue update。
- online user update。
- chat message dispatch。
- backend event toast。
- password changed / auth error handling。

重构 UI 时，可以调整 toast 的视觉表现，但不应把这些事件处理逻辑散落回组件。

### `music-party-web/src/stores/user.js`

这是用户身份状态核心，建议保留。

应保留：

- `userToken`。
- `currentUser`。
- `onlineUsers`。
- `isGuest`。
- `isAuthPassed`。
- `showNameModal`。
- `initUser()`。
- `setOnlineUsers()`。
- `resolveName()`。
- `setPostNameAction()`。
- `resetAuthentication()`。
- 平台绑定相关状态和方法。

注意：该文件里存在一些旧代码痕迹，例如文件顶层和 store 内部都有 `currentUser/storageName` 概念。它不阻塞 UI 重构，可作为后续小清理处理。

### API 层

建议保留：

- `music-party-web/src/api/client.js`
- `music-party-web/src/api/music.js`
- `music-party-web/src/api/auth.js`

这些文件是 REST API 的稳定边界。新 UI 不应直接拼接后端 URL 或重写 axios wrapper。

### 工具函数

建议保留：

- `music-party-web/src/utils/format.js`
- `music-party-web/src/utils/likedSongs.js`
- `music-party-web/src/utils/audioUrl.js`
- `music-party-web/src/utils/parser.js`
- `music-party-web/src/utils/errors.js`

其中 `audioUrl.js` 对 Navidrome 播放 URL token 注入有用；`likedSongs.js` 对喜欢歌曲导出有用。

## 可以保留但建议包一层的逻辑

### 搜索相关

涉及文件：

- `music-party-web/src/composables/useSearchLogic.js`
- `music-party-web/src/composables/usePlaylistLogic.js`
- `music-party-web/src/composables/usePlatforms.js`

这些逻辑可以保留，但建议在 UI 重构中整理成更稳定的搜索域接口。

推荐输出接口：

- `platforms`
- `selectedPlatform`
- `supportsAlbumSearch`
- `searchType`
- `keyword`
- `songs`
- `albums`
- `loading`
- `loadPlatforms()`
- `search()`
- `selectPlatform()`

桌面搜索和移动搜索应共用同一套搜索状态和动作，避免状态分叉。

### 队列多选

涉及文件：

- `music-party-web/src/composables/useQueueSelection.js`

该 composable 的选择状态、全选、清理无效 queueId 等逻辑可以保留。

可以重写：

- 多选工具栏。
- 删除确认 UI。
- 长按进入选择模式的视觉反馈。
- 批量操作按钮布局。

不建议重写选择状态本身。

### 歌词相关

涉及文件：

- `music-party-web/src/components/AppleLyricsPanel.vue`
- `music-party-web/src/components/mobile/MobileMiniLyrics.vue`

歌词同步、active line 查找、翻译显示等逻辑可以保留或迁移。

视觉层建议重做，因为当前歌词组件里存在较多 viewport 相关字号、`clamp()` 和局部样式，跨设备可控性一般。

## 建议重写或大幅重构的 UI 层

### `music-party-web/src/components/layout/MainLayout.vue`

这是桌面 app shell，建议大幅重写。

当前问题：

- header、left users、center、right queue、bottom player 的尺寸策略不统一。
- `clamp()`、固定宽度、浮动 mobile panel 混杂。
- 组件职责偏多，既负责桌面布局，也夹带移动端 panel 逻辑和 lite mode 逻辑。

建议重构成明确的 app shell：

- `TopBar`
- `LeftRail` 或 `MembersPanel`
- `MainStage`
- `RightPanel`
- `PlayerDock`
- `LiteModeView`

### `music-party-web/src/components/QueueList.vue`

这是桌面队列/喜欢视图，建议重写结构。

当前问题：

- 队列视图和喜欢视图 header action 不对称。
- 喜欢页面导出按钮会改变 header 视觉重心。
- 队列/喜欢 list item 的尺寸 contract 不统一。

建议抽象：

- `PanelHeader`
- `SegmentedControl`
- `PanelActionSlot`
- `TrackListItem`
- `BulkActionBar`

队列和喜欢视图应共享同一套 panel 宽度、header 高度、列表内边距和 item 起点。

### `music-party-web/src/components/mobile/MobileQueueView.vue`

这是移动端队列/喜欢视图，建议大幅重构。

当前问题：

- 队列 item 和喜欢 item 使用不同 grid columns。
- 导出按钮在 liked view 中占据 header 空间，造成队列/喜欢视图不等宽、不等重。
- selection rail 是浮动绝对定位，容易在不同屏幕高度和安全区下错位。

建议：

- 统一 `TrackListItem`。
- selection/index/action 作为 slot。
- 批量操作放底部固定 action bar，不用悬浮 rail。
- liked export 放入 header action icon 或 overflow menu，避免改变布局基准。

### `music-party-web/src/components/PlayerControl.vue`

这是桌面播放器控制区，建议拆分。

业务动作可以保留，但 UI 结构建议拆成：

- `NowPlayingCard`
- `ProgressScrubber`
- `TransportControls`
- `VolumeControl`
- `TrackActions`
- `LikeButton`

不要在单个组件里继续堆叠封面、标题、进度条、marker、下载、来源页、音量和锁状态。

### `music-party-web/src/components/CenterConsole.vue`

这是桌面主视觉核心。如果做整体 UI/UX 重构，应重新定义它的角色。

建议决策：

- 要么成为当前播放的沉浸主舞台。
- 要么成为房间状态 dashboard。
- 不建议继续作为一组混合视觉装饰和状态块的容器。

### 移动端整体组件

涉及文件：

- `music-party-web/src/components/mobile/MobileLayout.vue`
- `music-party-web/src/components/mobile/MobileNowPlaying.vue`
- `music-party-web/src/components/mobile/MobileSearchView.vue`
- `music-party-web/src/components/mobile/MobileBottomNav.vue`
- `music-party-web/src/components/mobile/MobileChatView.vue`

建议按页面级布局重新组织。

移动端应有统一的：

- safe-area 处理。
- top bar 高度。
- bottom nav 高度。
- page padding。
- list item 高度。
- action button 尺寸。
- modal/sheet 规则。

不要继续逐个组件修 padding 和 grid。

## 新 UI 应遵守的边界

新的 UI 组件只应该通过以下入口读取或操作业务：

```js
const player = usePlayerStore()
const user = useUserStore()
const ui = useUiStore()
const chat = useChatStore()
```

允许调用：

```js
player.playNext()
player.togglePause()
player.toggleShuffle()
player.seek(positionMs)
player.enqueue(platform, musicId)
player.enqueuePlaylist(platform, playlistId)
player.enqueueAlbum(platform, albumId)
player.topSong(queueId)
player.removeSong(queueId)
player.topSongs(queueIds)
player.removeSongs(queueIds)
player.sendLike()
player.addLikedSong()
player.removeLikedSong(key)
player.isSongLiked(music)
player.sendChatMessage(content)
```

不建议新 UI 做这些事：

- 直接调用 `socketService.send()`。
- 直接订阅 STOMP topic。
- 重新计算服务端同步时间。
- 重写 audio drift 修正。
- 重写 Wake Lock / Media Session。
- 直接拼接后端 API URL。
- 在 UI 组件里重复实现 liked songs 导出格式。
- 在多个页面里复制队列 item markup。

## 推荐重构方向

### 1. 先建立布局 tokens

建议先定义一组固定语义 token，再写组件。

示例：

- app height。
- top bar height。
- desktop left panel width。
- desktop right panel width。
- player dock height。
- mobile top bar height。
- mobile bottom nav height。
- panel padding。
- list item compact height。
- list item regular height。
- icon button size。
- action button min width。
- safe area padding。

重点是建立绝对尺寸和响应式约束，不要继续依赖零散 `clamp()` 和局部 magic number。

### 2. 抽 UI primitives

建议优先抽：

- `AppShell`
- `Panel`
- `PanelHeader`
- `SegmentedControl`
- `TrackListItem`
- `IconButton`
- `ActionBar`
- `BottomSheet`
- `EmptyState`

这些组件应只关心视觉和布局，不包含 socket 或播放同步逻辑。

### 3. 先重构队列/喜欢视图

原因：

- 当前问题最明确。
- 队列和喜欢视图能验证 list item、header、action slot、empty state、bulk action 的组件契约。
- 修好后可复用到搜索结果、歌单歌曲、喜欢歌曲。

目标：

- 队列和喜欢视图同宽。
- 标题、tab、action、列表文本起点一致。
- 导出按钮不改变整个 panel 的视觉基准。
- 移动端和桌面端共享 item 结构。

### 4. 再重构播放器控制区

目标：

- 当前播放信息、进度条、主控制按钮、次级动作、音量各自有明确区域。
- 下载、来源页、喜欢、音量等次级动作不挤压主播放控制。
- 进度条 marker 不影响布局高度。

### 5. 最后重构桌面/移动整体布局

目标：

- 桌面：稳定 app shell，不再靠临时宽度修补。
- 移动：页面级导航和 safe-area 统一，不再每个组件单独处理高度。
- lite mode：作为独立模式组件，不混在主布局组件里。

## 关键结论

前端业务逻辑大约 70% 到 80% 可以保留。

应保留的核心是：

- Pinia stores。
- WebSocket/STOMP 封装。
- socket message 分发。
- audio 播放同步。
- REST API wrapper。
- 队列/喜欢/搜索相关 composable 的业务部分。

应重写的核心是：

- app shell。
- panel/header/tab/list/action 的布局结构。
- 队列与喜欢列表 UI。
- 播放器控制区 UI。
- 移动端页面结构。
- 设计 tokens 和 CSS 组织。

后续 agent 的重点不应是“继续调几个 padding”，而应是建立一套稳定 UI 架构，让所有视图都服从同一组尺寸、间距和响应式规则。

## 注意事项

- 不要提交或引用 `NAVIDROME_HANDOFF_2026-05-13.md` 里的本地凭据。
- 本文档只描述前端 UI/UX 重构边界，不包含后端 Navidrome 修补计划。
- Navidrome 相关前端逻辑应通过现有 `musicApi.getPlatforms()`、`musicApi.search()` 和 `withPlaybackToken()` 继续接入。
- HTTP Radio Stream 不支持 Navidrome，这不是前端 UI 重构目标。
