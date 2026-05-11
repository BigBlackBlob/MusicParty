# MusicParty UI Handoff - 2026-05-11

## 本轮目标

用户当前将重点收敛到主播放页面的 UI 改造，暂时不处理房间鉴权、Public 房间、密码访问等问题。

核心诉求：

1. 将主播放区域重构为更接近 Apple Music 的风格。
2. 保持原有播放和交互逻辑，不做功能层面的大改。
3. 去掉偏“系统控制台 / HUD”的视觉表达。

参考方向：

- 用户点名参考 `https://github.com/amll-dev/applemusic-like-lyric`

---

## 最终确认的视觉要求

用户在本轮最后明确要求如下：

1. 整个 UI 不要有“被一层白色半透明遮罩盖住”的感觉。
2. 原项目主播放页面背后的圆环背景要去掉。
3. 左上角 `Now Playing` 要去掉。
4. 封面下方的标题、副标题和提示信息全部去掉。
5. 右侧 `Lyrics` 卡片要去掉。
6. 歌词区域应当：
   - 只在检测到歌词时出现
   - 直接以 Apple Music 风格的文字排版渲染
   - 不再包一层卡片式容器

---

## 本轮已完成的实现

### 1. 主播放页主舞台已重构

主要改动文件：

- `music-party-web/src/components/CenterConsole.vue`

当前实现状态：

- 左侧保留大封面主舞台
- 保留点赞交互和点赞爆发动画
- 保留加载中遮罩
- 背景改为更克制的封面氛围层
- 移除了圆环 / 可视化 canvas
- 移除了左上状态胶囊
- 移除了封面下方歌曲标题、艺术家和提示胶囊
- 右侧歌词仅在存在歌词时渲染
- 歌词不再放在玻璃卡片中，而是直接作为大字排版展示

### 2. 为新页面导出过同步状态字段

改动文件：

- `music-party-web/src/stores/player.js`

说明：

- 本轮中途曾为 UI 显示导出 `lastSyncTime`
- 目前 `CenterConsole.vue` 已不再使用这些同步状态文案，但导出本身仍保留，不影响运行

---

## 当前代码结果

### 已确认通过

- `npm run build` 已通过

### 本地预览端口

已确认可启动的前端备用地址：

- `http://127.0.0.1:5174/`

补充说明：

- 用户机器上 `5173` 当时已被旧前端实例占用
- 用户已说明这是因为之前忘记关闭旧前端，可以保留备用端口

---

## 当前页面结构理解

用户最初在浏览器中选中的主播放区域是：

- `main.flex-1.bg-[var(--surface-2)].relative.flex.flex-col.overflow-hidden.z-10`

实际对应主要文件：

- `music-party-web/src/components/layout/MainLayout.vue`
- `music-party-web/src/components/CenterConsole.vue`

本轮实际重点改的是：

- `CenterConsole.vue`

`MainLayout.vue` 本轮未做主结构改造。

---

## 本轮过程中已明确的边界

暂不处理：

1. 房间 auth
2. Public 房间显示问题
3. 密码访问逻辑
4. 管理员 / 房间权限体系

这些问题此前讨论过，但用户已明确表示当前可以先搁置。

---

## 后续建议

如果在新对话里继续推进，建议优先按这个顺序：

1. 继续微调 `CenterConsole.vue` 的歌词排版
   - 当前句更强
   - 前后句更弱
   - 更接近 Apple Music 的字重、字号、间距

2. 处理“无歌词”场景的版面完整性
   - 当前无歌词时右侧会完全消失
   - 需要决定这是预期，还是需要用更克制的留白 / 元信息来平衡布局

3. 再决定是否继续统一底部播放器和整体舞台
   - `music-party-web/src/components/PlayerControl.vue`
   - `music-party-web/src/components/layout/MainLayout.vue`

---

## 本轮涉及的关键文件

- `D:/Pluvllter-MusicParty/MusicParty-master/music-party-web/src/components/CenterConsole.vue`
- `D:/Pluvllter-MusicParty/MusicParty-master/music-party-web/src/stores/player.js`
- `D:/Pluvllter-MusicParty/MusicParty-master/music-party-web/src/components/layout/MainLayout.vue`
- `D:/Pluvllter-MusicParty/MusicParty-master/music-party-web/src/components/PlayerControl.vue`

---

## 给下一轮 agent 的一句话总结

当前主播放页已经从原来的控制台风格改成了“封面主舞台 + 条件歌词区”的 Apple Music 方向版本；下一步不要回头重做结构，应该在现有 `CenterConsole.vue` 基础上继续做精修。
