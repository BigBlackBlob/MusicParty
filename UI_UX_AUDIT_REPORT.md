# MusicParty UI/UX 审计报告

> 基于 `ui-ux-pro-max` 设计指南对 MusicParty 项目前端进行全面评估。
> 评估日期：2026-05-11
> 项目定位：Entertainment / Social Music Party
> 技术栈：Vue 3 + Tailwind CSS + Pinia
> 设计方向：Apple Music / Spotify 风格，沉浸式、内容优先、暗色优先

---

## 目录

1. [Priority 1 — Accessibility (CRITICAL)](#1-priority-1--accessibility-critical)
2. [Priority 2 — Touch & Interaction (CRITICAL)](#2-priority-2--touch--interaction-critical)
3. [Priority 3 — Performance (HIGH)](#3-priority-3--performance-high)
4. [Priority 4 — Style Selection (HIGH)](#4-priority-4--style-selection-high)
5. [Priority 5 — Layout & Responsive (HIGH)](#5-priority-5--layout--responsive-high)
6. [Priority 6 — Typography & Color (MEDIUM)](#6-priority-6--typography--color-medium)
7. [Priority 7 — Animation (MEDIUM)](#7-priority-7--animation-medium)
8. [Priority 8 — Forms & Feedback (MEDIUM)](#8-priority-8--forms--feedback-medium)
9. [Priority 9 — Navigation Patterns (HIGH)](#9-priority-9--navigation-patterns-high)
10. [修改建议优先级总览](#修改建议优先级总览)

---

## ui-ux-pro-max 设计系统推荐

运行 `--design-system "entertainment social music immersive dark minimal"` 得到：

### 推荐 Style: Dark Mode (OLED)
- Keywords: Dark theme, low light, high contrast, deep black, midnight blue, eye-friendly, OLED, night mode, power efficient
- Best For: Night-mode apps, coding platforms, **entertainment**, eye-strain prevention
- Performance: ⚡ Excellent | Accessibility: ✓ WCAG AAA

### 推荐 Colors (Music Streaming)
| Token | Value | Usage |
|-------|-------|-------|
| Primary | `#1E1B4B` | 主色 |
| Secondary | `#4338CA` | 辅色 |
| Accent/CTA | `#22C55E` | 播放/活跃状态 |
| Background | `#0F0F23` | 背景 |
| Foreground | `#F8FAFC` | 前景文字 |
| Muted | `#27273B` | 弱化区域 |
| Border | `#312E81` | 边框 |
| Destructive | `#EF4444` | 危险操作 |

### 推荐 Typography: Righteous / Poppins
- Mood: music, entertainment, fun, energetic, bold, performance
- Best For: Music platforms, entertainment, events, festivals
- Google Fonts: [Poppins](https://fonts.google.com/specimen/Poppins) + [Righteous](https://fonts.google.com/specimen/Righteous)

### Key Effects
- Minimal glow (`text-shadow: 0 0 10px`)
- Dark-to-light transitions
- Low white emission, high readability, visible focus

### 反模式警告
- Cluttered layout
- Poor audio player UX

---

## 1. Priority 1 — Accessibility (CRITICAL)

### 1.1 已做到 ✅

| 项目 | 状态 | 位置 |
|------|------|------|
| `prefers-reduced-motion` | ✅ 已实现 | `AppleLyricsPanel.vue:306-310` |
| 语义颜色令牌 CSS 变量系统 | ✅ | `style.css` 全套 `--surface-*` / `--text-*` / `--accent-*` |
| `aria-live="off"` 歌词区域标注 | ✅ | `AppleLyricsPanel.vue:23` |
| dark/light 双主题支持 | ✅ | `ui.js` + CSS class 切换 |

### 1.2 需修复 🔴

| # | 问题 | 文件 | 行号 | 严重度 | 修复方案 |
|---|------|------|------|--------|----------|
| 1 | 缺少 skip-link（跳至主内容） | `MainLayout.vue` | 40 | 高 | 添加 `<a href="#main-content" class="sr-only focus:not-sr-only ...">跳至主内容</a>` |
| 2 | `outline-none` 无 focus 替代——搜索输入框 | `SearchModal.vue` | 33 | 高 | 加 `focus:ring-2 focus:ring-[var(--accent)]` |
| 3 | `outline-none` 无 focus 替代——歌单搜索 | `SearchModal.vue` | 64 | 高 | 同上 |
| 4 | `outline-none` 无 focus 替代——聊天输入 | `ChatOverlay.vue` | 114 | 高 | 同上 |
| 5 | `outline-none` 无 focus 替代——用户名输入 | `UserList.vue` | 37 | 高 | 同上 |
| 6 | 无 `aria-label`——所有 icon-only 按钮 | `PlayerControl.vue` | 121,125,142,149,152 | 高 | 添加 `aria-label="下载"` 等 |
| 7 | 无 `aria-label`——移动端控制按钮 | `PlayerControl.vue` | 87-112 | 高 | 添加 `aria-label` |
| 8 | 无 `aria-label`——X 关闭按钮 | `SearchModal.vue` | 4 | 中 | 添加 `aria-label="关闭"` |
| 9 | 无 `aria-label`——X 关闭按钮 | `ChatOverlay.vue` | 32 | 中 | 添加 `aria-label="关闭聊天"` |
| 10 | 无 `aria-label`——主题切换按钮 | `MainLayout.vue` | 67-74 | 中 | 添加 `aria-label`（已有 title，不够） |
| 11 | 无 `aria-label`——精简模式按钮 | `MainLayout.vue` | 77-83 | 中 | 同上 |
| 12 | `<img>` 无 `alt` 属性——封面图 | `CenterConsole.vue` | 44-49 | 高 | 添加 `:alt="nowPlaying?.music.name + ' 封面'"` |
| 13 | `<img>` 无 `alt` 属性——搜索结果封面 | `SearchModal.vue` | 157-159 | 高 | 添加 `:alt="song.name + ' 封面'"` |
| 14 | `<img>` 无 `alt` 属性——歌单封面 | `SearchModal.vue` | 107-109 | 中 | 添加 `:alt="pl.name + ' 封面'"` |
| 15 | `<img>` 无 `alt` 属性——用户头像 | `SearchModal.vue` | 79 | 中 | 添加 `:alt="user.name"` |
| 16 | `<img>` 无 `alt` 属性——CoverImage 组件 | `CoverImage.vue` | 3-8 | 高 | 添加 prop `alt` 并绑定到 `<img :alt="alt" />` |
| 17 | 模态框无 Escape 关闭 | `SearchModal.vue` | 2 | 高 | 添加 `@keydown.escape="emit('close')"` |
| 18 | 颜色传达在线状态无文字辅助 | `UserList.vue` | 84,98 | 中 | 在线小圆点加 `aria-label="在线"` |
| 19 | `Screen reader` 无法识别"DJ"/"ME" 身份标识 | `UserList.vue` | 28-29 | 中 | icon-only 身份标识加 `aria-label` |

---

## 2. Priority 2 — Touch & Interaction (CRITICAL)

### 2.1 已做到 ✅

| 项目 | 状态 | 位置 |
|------|------|------|
| `touch-action` 进度条/音量滑块 | ✅ | `PlayerControl.vue:56,158` |
| 触摸滚动意图检测 | ✅ | `AppleLyricsPanel.vue:220-235` |
| 双击点赞移动端适配 | ✅ | `CenterConsole.vue:209-225` |
| `cursor-pointer` 已使用 | ✅ | `CenterConsole.vue`, `PlayerControl.vue` 多处 |
| Toast 通知 feedback | ✅ | `useToast` composable |

### 2.2 需修复 🔴

| # | 问题 | 文件 | 行号 | 严重度 | 修复方案 |
|---|------|------|------|--------|----------|
| 20 | Download 按钮触控目标 20px | `PlayerControl.vue` | 121 | 高 | 加 `min-w-[44px] min-h-[44px]` 或 `p-3` |
| 21 | Shuffle 按钮触控目标 20px | `PlayerControl.vue` | 125 | 高 | 同上 |
| 22 | SkipForward 按钮触控目标 24px | `PlayerControl.vue` | 142 | 高 | 同上 |
| 23 | Volume 按钮触控目标 20px | `PlayerControl.vue` | 149-153 | 高 | 同上 |
| 24 | 移动端人数按钮 36×36px | `MainLayout.vue` | 53-65 | 高 | 改为 `w-11 h-11` (44px) |
| 25 | 移动端主题按钮 36×36px | `MainLayout.vue` | 67-74 | 高 | 改为 `min-w-[44px] min-h-[44px]` |
| 26 | 移动端队列按钮 36×36px | `MainLayout.vue` | 108 | 高 | 同上 |
| 27 | 歌词字体控制按钮 40px | `AppleLyricsPanel.vue` | 39-44 | 中 | 改为 `w-11 h-11` (44px) |
| 28 | 移动端控制按钮无按压反馈 | `PlayerControl.vue` | 87-112 | 高 | 添加 `active:scale-[0.95] active:bg-[var(--surface-3)]` |
| 29 | 搜索框触控间距不足 | `SearchModal.vue` | 29-45 | 中 | search 和 button 间距 `gap-2` (8px) 刚达标，但 mobile 上可增至 12px |

---

## 3. Priority 3 — Performance (HIGH)

### 3.1 已做到 ✅

| 项目 | 状态 | 位置 |
|------|------|------|
| 虚拟列表（>50 项） | ✅ | `QueueList.vue:89-92` `useVirtualList` |
| `will-change: transform, opacity` | ✅ | `AppleLyricsPanel.vue:284` |
| `backdrop-blur` 适度使用 | ✅ | 仅在 `PlayerControl.vue` + modal 中 |
| 主站字体预连接 | ✅ | `index.html:8-9` preconnect |
| debounce (drag threshold 5px) | ✅ | `ChatOverlay.vue:222` |

### 3.2 需改进 🟡

| # | 问题 | 文件 | 行号 | 修复方案 |
|---|------|------|------|----------|
| 30 | 封面 `<img>` 无 size 声明 → CLS 风险 | `CenterConsole.vue` | 44-49 | 添加 `:width` / `:height` 或 `aspect-square` |
| 31 | 3 个外部字体同时加载无 `font-display: swap` | `index.html` | 10-12 | 加 `&display=swap` 到 Google Fonts URL |
| 32 | 搜索封面无 `loading="lazy"` | `SearchModal.vue` | 157-159 | 添加 `loading="lazy"` |
| 33 | 进度条持续 transition 每帧触发 | `PlayerControl.vue` | 62 | 正常播放时去掉 transition，仅 seek drag 时启用 |
| 34 | 歌词容器 `overflow-y-auto` 无 `contain: layout style` | `AppleLyricsPanel.vue` | 7 | 添加 `contain: strict` 隔离 layout 影响 |

---

## 4. Priority 4 — Style Selection (HIGH)

### 4.1 评估

项目风格 **Dark Mode (OLED) + 内容优先沉浸式** 与 Apple Music / Spotify 方向高度匹配。`ui-ux-pro-max` 推荐的 **Music Streaming** 配色方案与现有 `#d3c2f3` (紫色 accent) 方案差异点：

### 4.2 需改进 🟡

| # | 问题 | 说明 | 修复方案 |
|---|------|------|----------|
| 35 | 工业/HUD 文案残留 | 多处使用 "LOADING DATA STREAM...", "> NO RECORDS", "OPERATIVES", "SYSTEM STANDBY" 等 | 改为自然语言（"加载中...", "暂无记录", "在线成员", "系统待机"） |
| 36 | `font-mono` 过度使用 | 大量 UI 文本使用等宽字体，HUD 风格残余 | 仅在时间/进度/ID 场景保留 mono，其余改 `font-sans` |
| 37 | Accent 绿色缺失 | 仅有紫色 accent，缺少"播放/活跃/成功"的语义绿色 | 将 `--success` (已有 `#22c55e`) 扩展到播放按钮等活跃状态 |
| 38 | 没有 Primary CTA 区分 | 每个页面多个相同视觉权重的按钮 | 每个 View 明确一个 primary action（如搜索页的"搜索"按钮） |

---

## 5. Priority 5 — Layout & Responsive (HIGH)

### 5.1 已做到 ✅

| 项目 | 状态 | 位置 |
|------|------|------|
| `h-[100dvh]` | ✅ | `MainLayout.vue:40` |
| viewport meta 正确 | ✅ | `index.html:7` |
| md 断点三栏/单栏切换 | ✅ | `MainLayout.vue` |

### 5.2 需改进 🟡

| # | 问题 | 文件 | 行号 | 修复方案 |
|---|------|------|------|----------|
| 39 | 左侧栏固定 `w-64` (256px) 不响应 | `MainLayout.vue` | 95 | 改为 `w-[18%] min-w-[240px] max-w-[320px]` |
| 40 | 右侧栏固定 `w-80` (320px) 不响应 | `MainLayout.vue` | 103 | 同上 |
| 41 | 移动端队列/用户面板全屏覆盖无法同屏 | `MainLayout.vue` | 113-131 | 改为底部 sheet 滑出模式 |
| 42 | 无系统 z-index scale | `MainLayout.vue` 等多处 | 各处 | 定义 CSS 变量：`--z-base:0; --z-header:40; --z-modal:60; --z-toast:100` |
| 43 | 大屏 >1440px 主内容区被压缩 | `MainLayout.vue` | 99 | 给 `<main>` 加 `max-w-[1920px] mx-auto` |
| 44 | 移动端无底部导航栏 | `MainLayout.vue` | — | 添加底部 tab bar（搜索/队列/聊天/成员） |

---

## 6. Priority 6 — Typography & Color (MEDIUM)

### 6.1 已做到 ✅

| 项目 | 状态 |
|------|------|
| 完整的语义色令牌 dark/light 双套 | ✅ `style.css` |
| 动态 accent 从封面提取 | ✅ `ui.js:102-124` |
| 字体栈 fallback 合理 | ✅ `tailwind.config.js:44-58` |
| 滚动条自定义 | ✅ `style.css:103-116` |
| 自定义 range input 样式 | ✅ `style.css:144-181` |

### 6.2 需改进 🟡

| # | 问题 | 文件 | 行号 | 对比度 | 修复方案 |
|---|------|------|------|--------|----------|
| 45 | 暗色 `--text-secondary: #a0a0a0` | `style.css` | 13 | ~3.5:1 (低于 AA) | 改为 `#b8b8b8` (4.7:1) |
| 46 | 暗色 `--text-tertiary: #6b6b6b` | `style.css` | 14 | ~2.2:1 (严重不合格) | 改为 `#8a8a8a` (3.7:1，大文本可接受) |
| 47 | 浅色 `--text-secondary: #6b6b6b` | `style.css` | 59 | ~3.8:1 (低于 AA) | 改为 `#5a5a5a` (5.7:1) |
| 48 | 浅色 `--text-tertiary: #a0a0a0` | `style.css` | 60 | ~2.6:1 (严重不合格) | 改为 `#787878` (4.7:1) |
| 49 | 歌词 `line-height: 1.04` | `AppleLyricsPanel.vue` | 144 | — | 改为 `1.3`，中文歌词需要更高行高 |
| 50 | `text-[10px]` 低于 12px 最低标准 | `SearchModal.vue:162,172,191` 等多处 | — | 改为 `text-xs` (12px) |
| 51 | `text-[9px]` 低于 12px 最低标准 | `ChatOverlay.vue:65,125` | — | 改为 `text-[11px]` |
| 52 | 歌词 `font-weight 750/650/600` 不规范 | `AppleLyricsPanel.vue` | 143,156,169 | 用标准 weight: 700/600/500 |
| 53 | toast 文本色暗色 `--toast-message: #c9c9c9` | `style.css` | 36 | ~4.1:1 勉强 | 改为 `#d4d4d4` |

---

## 7. Priority 7 — Animation (MEDIUM)

### 7.1 已做到 ✅

| 项目 | 状态 | 位置 |
|------|------|------|
| `prefers-reduced-motion` (歌词) | ✅ | `AppleLyricsPanel.vue:306-310` |
| `cubic-bezier(0.16,1,0.3,1)` 弹性缓动 | ✅ | `AppleLyricsPanel.vue:28` |
| transform/opacity-only 动画 | ✅ | 歌词行 scale + translate3d |
| Vue Transition 组件 | ✅ | 封面点赞 overlay, 聊天窗口 |
| exit < enter (150ms vs 100ms slide) | ✅ | `style.css:130-136` |

### 7.2 需改进 🟡

| # | 问题 | 文件 | 行号 | 修复方案 |
|---|------|------|------|----------|
| 54 | UserList `.bar` 无 reduced-motion | `UserList.vue` | 141-156 | 加 `@media (prefers-reduced-motion: reduce) { .bar { animation: none; } }` |
| 55 | 封面 hover scale 移动端不适用 | `CenterConsole.vue` | 48 | 移动端用 `active:scale-[1.02]` 替代 `group-hover` |
| 56 | 进度条每帧 transition | `PlayerControl.vue` | 62 | 改用 `:class` 控制只在 seek 时 transition |
| 57 | 搜索 modal 无进入动画 | `SearchModal.vue` | 2 | 包一层 `<Transition>` 加 slide+fade 进入 |

---

## 8. Priority 8 — Forms & Feedback (MEDIUM)

### 8.1 需改进 🟡

| # | 问题 | 文件 | 行号 | 修复方案 |
|---|------|------|------|----------|
| 58 | 搜索输入框只有 placeholder 无 label | `SearchModal.vue` | 32 | 添加 `<label for="search-input" class="sr-only">搜索音乐</label>` |
| 59 | 聊天输入框只有 placeholder 无 label | `ChatOverlay.vue` | 113 | 添加 `aria-label="消息内容"` |
| 60 | 用户名修改输入框无 label | `UserList.vue` | 37 | 添加 `aria-label="修改昵称"` |
| 61 | 歌单搜索输入框无 label | `SearchModal.vue` | 64 | 添加 `aria-label="搜索用户"` |
| 62 | 队列 empty state 无引导动作 | `QueueList.vue` | 10 | 添加"去搜索歌曲"按钮 |
| 63 | Loading 文案风格不友好 | `SearchModal.vue` | 133 | 改为"加载中..." |
| 64 | No data 文案风格不友好 | `SearchModal.vue` | 150 | 改为"未找到相关歌曲" |

---

## 9. Priority 9 — Navigation Patterns (HIGH)

### 9.1 需改进 🟡

| # | 问题 | 文件 | 行号 | 修复方案 |
|---|------|------|------|----------|
| 65 | 移动端 4 个独立入口散落各处 | `MainLayout.vue` | 各处 | 统一为底部 tab bar (搜索/队列/聊天/成员) |
| 66 | 聊天窗口无边缘吸附 | `ChatOverlay.vue` | 178-185 | 加左/右侧 snap，释放后自动吸附最近边缘 |
| 67 | 模态框不能点遮罩关闭 | `SearchModal.vue` | 2 | 遮罩层加 `@click.self="emit('close')"` |
| 68 | 没有 deep link / URL routing | 全局 | — | 房间状态无法通过 URL 分享 |
| 69 | 精简模式下无法切换回来（除退出按钮外） | `MainLayout.vue` | 133-209 | 精简模式应保留顶部小 bar 方便切换 |

---

## 10. 修改建议优先级总览

### 🔴 P0 — 必须立即修复（Accessibility + Touch Critical）：19 项

| # | 类别 | 问题简述 |
|---|------|----------|
| 1 | A11y | 缺少 skip-link |
| 2-5 | A11y | `outline-none` 无 `focus:ring` 替代（4 处 input） |
| 6-8 | A11y | icon-only 按钮无 `aria-label`（PlayerControl 8 个 + SearchModal 关闭 + ChatOverlay 关闭） |
| 12-16 | A11y | `<img>` 无 `alt`（CenterConsole, SearchModal, CoverImage 等 5 处） |
| 17 | A11y | 模态框无 Escape 键关闭 |
| 18-19 | A11y | 在线状态/DJ 标识无 screen reader 支持 |
| 20-23 | Touch | PC 端控制按钮触控 <44px（4 个） |
| 24-27 | Touch | 移动端按钮触控 <44px（4 处） |
| 28 | Touch | 移动端按钮无按压反馈 `active:` |
| 29 | Touch | 搜索框间距移动端需增大 |

### 🟡 P1 — 高优先级改进：19 项

| # | 类别 | 问题简述 |
|---|------|----------|
| 30-34 | Perf | CLS 风险、字体加载、图片 lazy、进度条性能 |
| 35-36 | Style | 工业文案残留、font-mono 过度使用 |
| 39-44 | Layout | 侧栏固定宽度、移动端缺少底部 nav、z-index |
| 45-48 | Color | dark/light 模式下 text-secondary/tertiary 对比度不合格 |
| 54-57 | Anim | reduced-motion 缺失、进度条 transition 浪费 |
| 58-59 | Form | 输入框缺少 label |

### 🟢 P2 — 中优先级优化：12 项

| # | 类别 | 问题简述 |
|---|------|----------|
| 10-11 | A11y | 主题/精简按钮 aria-label 补充 |
| 37-38 | Style | 绿色 accent 语义扩展、Primary CTA 区分 |
| 49-53 | Typo | 歌词行高、超小字号、font-weight 规范 |
| 62-64 | UX | Empty state 引导、文案风格 |
| 65-69 | Nav | 底部 tab bar、聊天吸附、遮罩关闭 |

---

## 附录：关键文件索引

| 文件 | 用途 |
|------|------|
| `music-party-web/src/style.css` | 全局主题令牌 dark/light |
| `music-party-web/tailwind.config.js` | Tailwind 主题扩展 |
| `music-party-web/src/stores/ui.js` | UI 状态管理 + 动态 accent |
| `music-party-web/src/stores/player.js` | 播放器状态管理 |
| `music-party-web/src/components/layout/MainLayout.vue` | 主布局（header/aside/main） |
| `music-party-web/src/components/CenterConsole.vue` | 中心舞台 + 封面 + 歌词 |
| `music-party-web/src/components/PlayerControl.vue` | 底部播放控制栏 |
| `music-party-web/src/components/AppleLyricsPanel.vue` | 自定义歌词面板 |
| `music-party-web/src/components/SearchModal.vue` | 搜索模态框 |
| `music-party-web/src/components/QueueList.vue` | 播放队列 |
| `music-party-web/src/components/UserList.vue` | 在线用户列表 |
| `music-party-web/src/components/ChatOverlay.vue` | 可拖拽聊天窗口 |
| `music-party-web/src/components/CoverImage.vue` | 封面图片通用组件 |
| `music-party-web/src/components/ToastNotification.vue` | Toast 通知 |
| `music-party-web/index.html` | 入口 HTML |
