# MusicParty UI Redesign — Implementation Handoff Document
## Project Path
`D:\Pluvllter-MusicParty\MusicParty-master`
## Quick Reference: User-Confirmed Design Decisions
| Decision | Choice |
|---|---|
| Base color depth | `#121212` (Spotify-style, not pure black) |
| Accent color | `#d3c2f3` (soft lavender, replaces orange `#F97316`) |
| Border radius | 8-12px medium range (not aggressive Apple-style) |
| Font: CJK (SC/JP/KR) | HarmonyOS Sans (HarmonySans) |
| Font: EN + punctuation | Geist |
| Font: Mono | JetBrains Mono (retained) |
| Dark/Light mode | **Both required**, with toggle in UI |
| Chamfer decorations | **Remove entirely**, replace with border-radius |
| Scan-line textures | **Remove entirely** |
| Corner brackets / spinning rings | **Remove entirely** |
---
## 1. Color System (Final)
### 1.1 Dark Mode (Primary)
| Token | Hex | Usage |
|---|---|---|
| `surface-0` | `#121212` | App root background |
| `surface-1` | `#1a1a1a` | Sidebars, secondary panels |
| `surface-2` | `#222222` | Main content area |
| `surface-3` | `#2a2a2a` | Hover states, elevated cards |
| `surface-4` | `#333333` | Modals, dropdowns |
| `surface-elevated` | `#3e3e3e` | Tooltips, highest z-layer |
| `text-primary` | `#f0f0f0` | Headings, song titles, key content |
| `text-secondary` | `#a0a0a0` | Artist names, subtitles |
| `text-tertiary` | `#6b6b6b` | Timestamps, labels, very faint text |
| `text-inverse` | `#ffffff` | White text on accent/dark buttons |
| `accent` | `#d3c2f3` | Primary accent (replaces orange) |
| `accent-hover` | `#e0d4f7` | Hover state — lighter lavender |
| `accent-muted` | `rgba(211,194,243,0.15)` | Subtle accent backgrounds |
| `accent-subtle` | `rgba(211,194,243,0.08)` | Very faint accent backgrounds |
| `border-default` | `rgba(255,255,255,0.08)` | Default dividers (greatly reduced from current) |
| `border-subtle` | `rgba(255,255,255,0.04)` | Near-invisible structural lines |
| `border-accent` | `rgba(211,194,243,0.5)` | Focus/active borders |
| `success` | `#22c55e` | Already in queue, online |
| `warning` | `#eab308` | Buffering, warnings |
| `error` | `#ef4444` | Errors, failures |
### 1.2 Light Mode (Alternate)
| Token | Hex | Usage |
|---|---|---|
| `surface-0` | `#ffffff` | App root background |
| `surface-1` | `#f8f8f8` | Sidebars |
| `surface-2` | `#f3f3f3` | Main content area |
| `surface-3` | `#eaeaea` | Hover states, elevated cards |
| `surface-4` | `#ffffff` | Modals (white, with shadow) |
| `surface-elevated` | `#ffffff` | Tooltips (with shadow) |
| `text-primary` | `#1a1a1a` | Headings, key content |
| `text-secondary` | `#6b6b6b` | Subtitles |
| `text-tertiary` | `#a0a0a0` | Faint text |
| `text-inverse` | `#ffffff` | White text on accent buttons |
| `accent` | `#7c5cbf` | Saturated purple for light backgrounds |
| `accent-hover` | `#8d6ec8` | Hover state |
| `accent-muted` | `rgba(124,92,191,0.12)` | Subtle accent bg (light) |
| `accent-subtle` | `rgba(124,92,191,0.06)` | Very faint accent bg (light) |
| `border-default` | `rgba(0,0,0,0.08)` | Default dividers |
| `border-subtle` | `rgba(0,0,0,0.04)` | Near-invisible structural lines |
| `border-accent` | `rgba(124,92,191,0.5)` | Focus/active borders |
| `success` | `#16a34a` | Slightly darker green for light bg |
| `warning` | `#ca8a04` | Darker yellow for light bg |
| `error` | `#dc2626` | Darker red for light bg |
---
## 2. Typography
### 2.1 Font Families
```css
/* In index.html, replace current Google Fonts link with: */
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Noto+Sans+SC:wght@400;500;700;900&family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
NOTE: Geist may not be on Google Fonts. If not available via Google Fonts CDN, use this alternative approach:
- Load Geist from https://cdn.jsdelivr.net/fontsource/fonts/geist@latest/index.css or self-host
- Alternatively, the npm package geist can be installed and imported
For HarmonyOS Sans (CJK):
- Load from CDN: https://s1.hdslb.com/static/fonts/harmonyos-sans/harmonyos-sans-sc.css or similar public CDN
- Or self-host the font files
- Fallback chain: "HarmonyOS Sans SC", "HarmonyOS Sans JP", "HarmonyOS Sans KR", "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif
Tailwind config font families:
fontFamily: {
  sans: [
    '"Geist"', '"HarmonyOS Sans SC"', '"HarmonyOS Sans JP"', '"HarmonyOS Sans KR"',
    '"Noto Sans SC"', '"PingFang SC"', '"Hiragino Sans GB"', '"Microsoft YaHei"',
    'system-ui', '-apple-system', 'sans-serif'
  ],
  mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
}
2.2 Type Scale
Level
display
h1
h2
body
caption
label
Key change: Drastically reduce font-mono usage. Mono only for: playback timestamps, queue index numbers, system timestamps. ALL other text uses font-sans.
---
3. Spacing & Radius
3.1 Spacing (use Tailwind defaults, no changes needed)
Standard 4px grid. Prefer: p-2, p-3, p-4, p-6, gap-2, gap-3, gap-4.
3.2 Border Radius
Token
radius-sm
radius-md
radius-lg
radius-xl
radius-2xl
radius-full
ALL chamfer classes removed. ALL hardcoded sharp corners replaced.
---
4. Shadow System (Dark Mode)
Token
shadow-sm
shadow-md
shadow-lg
shadow-xl
Design principle: Use surface color difference instead of shadows wherever possible. Shadows only for elements floating above the plane (modals, tooltips, floating buttons).
---
5. Motion / Animation
Current
animate-spin on decorative rings
animate-scan scanline texture
animate-ping like burst
animate-pulse on many states
Cover hover scale-110 opacity-50
transition-colors instant
Vue transitions: scale+opacity
New CSS addition — skeleton shimmer:
@keyframes shimmer {
  0% { background-position: -200px 0; }
  100% { background-position: 200px 0; }
}
.animate-shimmer {
  background: linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.04) 50%, transparent 100%);
  background-size: 200px 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}
---
6. Theme Toggle Implementation
6.1 Store Changes (src/stores/ui.js)
Add:
import { ref, watch } from 'vue'
export const isDarkMode = ref(true) // default dark
// Persist and sync
if (localStorage.getItem('theme') === 'light') {
  isDarkMode.value = false
}
watch(isDarkMode, (val) => {
  localStorage.setItem('theme', val ? 'dark' : 'light')
  document.documentElement.classList.toggle('dark', val)
  document.documentElement.classList.toggle('light', !val)
})
// Initialize on load
document.documentElement.classList.toggle('dark', isDarkMode.value)
document.documentElement.classList.toggle('light', !isDarkMode.value)
export function toggleDarkMode() {
  isDarkMode.value = !isDarkMode.value
}
6.2 Tailwind Dark Mode Config
In tailwind.config.js:
darkMode: 'class',  // Enable class-based dark mode
CSS approach: Use CSS custom properties on :root (light) and .dark (dark) selectors so that all component classes remain the same regardless of theme, and only the custom property values change.
6.3 CSS Custom Properties Strategy
Define all theme tokens as CSS custom properties in style.css:
:root {
  /* Light mode defaults */
  --surface-0: #ffffff;
  --surface-1: #f8f8f8;
  --surface-2: #f3f3f3;
  --surface-3: #eaeaea;
  --surface-4: #ffffff;
  --surface-elevated: #ffffff;
  --text-primary: #1a1a1a;
  --text-secondary: #6b6b6b;
  --text-tertiary: #a0a0a0;
  --text-inverse: #ffffff;
  --accent: #7c5cbf;
  --accent-hover: #8d6ec8;
  --accent-rgb: 124,92,191;
  --border-default: rgba(0,0,0,0.08);
  --border-subtle: rgba(0,0,0,0.04);
  --success: #16a34a;
  --warning: #ca8a04;
  --error: #dc2626;
}
.dark {
  --surface-0: #121212;
  --surface-1: #1a1a1a;
  --surface-2: #222222;
  --surface-3: #2a2a2a;
  --surface-4: #333333;
  --surface-elevated: #3e3e3e;
  --text-primary: #f0f0f0;
  --text-secondary: #a0a0a0;
  --text-tertiary: #6b6b6b;
  --text-inverse: #ffffff;
  --accent: #d3c2f3;
  --accent-hover: #e0d4f7;
  --accent-rgb: 211,194,243;
  --border-default: rgba(255,255,255,0.08);
  --border-subtle: rgba(255,255,255,0.04);
  --success: #22c55e;
  --warning: #eab308;
  --error: #ef4444;
}
Then in components, use bg-[var(--surface-0)], text-[var(--text-primary)], etc.
OR alternatively (simpler approach): Use Tailwind dark: variant with the custom color tokens. Since we define surface-*, text-primary etc. in the Tailwind config, we can use classes like bg-surface-0 dark:bg-surface-0 where the token values themselves change based on .dark class. RECOMMENDED: CSS custom properties approach because it avoids doubling every class.
6.4 Toggle UI Location
Add a theme toggle button in the Header (MainLayout.vue) next to the lite mode toggle. Icon: Sun / Moon from lucide-vue-next. On click: toggleDarkMode().
---
7. Component-by-Component Redesign Spec
7.1 MainLayout.vue
Current: bg-medical-50, header bg-white border-b border-medical-200, left sidebar bg-medical-50 border-r, center bg-medical-100/30, right sidebar bg-white border-l
New Surface Hierarchy:
Root:          bg-[var(--surface-0)]
Header:        bg-[var(--surface-1)] border-b border-[var(--border-default)]
Left sidebar:  bg-[var(--surface-1)] border-r border-[var(--border-subtle)]
Center:        bg-[var(--surface-2)]
Right sidebar: bg-[var(--surface-1)] border-l border-[var(--border-subtle)]
Mobile panels: bg-[var(--surface-4)]
- Remove all chamfer-br classes
- Logo text: text-[var(--text-primary)]
- Author label: text-[var(--text-tertiary)]
- Search button: bg-[var(--accent)] text-[var(--text-inverse)] rounded-md
- Lite mode button: border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] rounded-md
- Add theme toggle button (Sun/Moon icon)
- Mobile overlay panels: bg-[var(--surface-4)] with slide transition
7.2 PlayerControl.vue
Current: bg-white border-t border-medical-200 shadow-lg
New:
Bar:     bg-[var(--surface-4)]/80 backdrop-blur-xl border-t border-[var(--border-subtle)]
Cover:   w-16 h-16 md:w-20 md:h-20 rounded-xl shadow-lg
Title:   text-[var(--text-primary)] text-lg font-bold
Artist:  text-[var(--text-secondary)] text-xs
Time:    text-[var(--text-tertiary)] font-mono text-[10px]
Progress track: h-1.5 bg-[var(--surface-2)] rounded-full
Progress fill:  bg-[var(--accent)] rounded-full
Progress thumb: w-3 h-3 bg-[var(--accent)] rounded-full (visible on hover/drag)
Like markers: text-[var(--accent)]
Pause/play button: w-10 h-10 rounded-full bg-[var(--accent)] text-[var(--text-inverse)]
Other buttons: text-[var(--text-secondary)] hover:text-[var(--accent)]
Mobile controls: bg-[var(--surface-2)] rounded-md
Volume: same pattern as progress, dark-themed
- Remove chamfer-tl from pause button → rounded-full
- Remove chamfer-br from cover → rounded-xl
7.3 CenterConsole.vue
Remove entirely:
- Four corner bracket decorations (border-t-2 border-l-2 border-medical-300)
- Spinning dashed rings (animate-spin)
- Grid background overlay
- Watermark large text
- .animate-scan overlay
- HUD corner markers
New:
Background: bg-[var(--surface-2)]  (clean, no texture)
Cover:      w-64 h-64 md:w-72 md:h-72 rounded-xl shadow-2xl overflow-hidden
Song title: text-[var(--text-primary)] text-xl font-bold mt-4
Artist:     text-[var(--text-secondary)] text-sm mt-1
No-media:   diamond placeholder → rounded-xl border with text-[var(--text-tertiary)]
Status badge: bg-[var(--surface-0)]/80 backdrop-blur text-[var(--text-primary)] rounded-md
Like burst: single scale+fade animation, no persistent ping
Loading overlay: bg-[var(--surface-0)]/50 backdrop-blur text-[var(--text-primary)]
Lyric active: text-[var(--text-primary)] scale-[1.02]
Lyric inactive: text-[var(--text-tertiary)] opacity-50
REQ_BY label: remove HUD style, use simple text-accent text-[10px]
7.4 SearchModal.vue
Current: bg-medical-900/80 backdrop-blur-sm overlay, bg-medical-50 modal, chamfer-br, clip-tab
New:
Overlay:  bg-[var(--surface-0)]/60 backdrop-blur-sm
Modal:    bg-[var(--surface-4)] rounded-2xl shadow-xl 
Header:   bg-[var(--surface-3)] border-b border-[var(--border-default)] rounded-t-2xl
Platform tabs: pill style, active bg-[var(--accent)] text-[var(--text-inverse)] rounded-full
             inactive text-[var(--text-tertiary)] hover:bg-[var(--surface-3)] rounded-full
Input:    bg-[var(--surface-2)] border-[var(--border-default)] rounded-md focus:border-[var(--accent)]
Search btn: bg-[var(--accent)] text-[var(--text-inverse)] rounded-md
Left panel: bg-[var(--surface-3)] border-r border-[var(--border-subtle)]
Song items: hover:bg-[var(--surface-3)] rounded-lg, no hard borders
In-queue badge: text-[var(--success)] icon
Add button: text-[var(--text-tertiary)] hover:text-[var(--accent)]
- Remove chamfer-br → rounded-2xl
- Remove clip-tab → rounded-full pill
7.5 AuthOverlay.vue
Overlay: bg-[var(--surface-0)]/90 backdrop-blur-sm
Card:    bg-[var(--surface-4)] p-8 rounded-2xl shadow-xl
Remove:  left decorative color bar (w-2 bg-medical-900)
Replace: accent left bar → remove entirely, or very subtle bg-[var(--accent)]/10 rounded-l-xl as optional
Title:   text-[var(--text-primary)] text-2xl font-bold
Input:   bg-[var(--surface-2)] border-[var(--border-default)] rounded-md focus:border-[var(--accent)]
Primary btn: bg-[var(--accent)] text-[var(--text-inverse)] rounded-md font-semibold
Secondary btn: bg-[var(--surface-2)] border-[var(--border-default)] text-[var(--text-secondary)] rounded-md
- Remove chamfer-br → rounded-2xl
7.6 NamePromptModal.vue
Same style as AuthOverlay:
Overlay: bg-[var(--surface-0)]/90 backdrop-blur-sm
Card:    bg-[var(--surface-4)] p-6 rounded-2xl shadow-xl
Remove:  accent left bar
Input:   bg-[var(--surface-2)] border-[var(--border-default)] rounded-md
Confirm: bg-[var(--accent)] text-[var(--text-inverse)] rounded-md
Cancel:  text-[var(--text-tertiary)] hover:text-[var(--text-primary)]
7.7 QueueList.vue
Panel:  bg-[var(--surface-1)] (sidebar background)
Header: bg-[var(--surface-1)] border-b border-[var(--border-subtle)]
Title:  text-[var(--text-primary)] text-sm font-semibold
Count:  text-[var(--accent)]
Empty:  text-[var(--text-tertiary)] text-xs
Shuffle banner: bg-[var(--accent-muted)] border border-[var(--border-accent)] rounded-md text-[var(--accent)]
User groups: border-b border-[var(--border-subtle)]
User avatar: bg-[var(--surface-3)] text-[var(--text-secondary)] rounded-full
7.8 QueueItem.vue
Row:     flex bg-[var(--surface-2)] rounded-lg hover:bg-[var(--surface-3)] p-2 mb-1.5
         NO border (remove border border-medical-100)
Index:   text-[var(--text-tertiary)] font-mono text-xs w-6 text-center
Song:    text-[var(--text-primary)] text-sm font-semibold
Artist:  text-[var(--text-secondary)] text-xs
Loading: text-[var(--accent)] text-xs font-semibold animate-pulse (keep pulse for loading)
Failed:  text-[var(--error)] text-xs font-semibold
User tag: text-[var(--text-tertiary)] text-[10px] bg-[var(--surface-3)] px-1.5 rounded-sm
Hover actions: opacity-0 group-hover:opacity-100, bg-[var(--surface-2)]/90
Top marker: w-2 h-2 bg-[var(--accent)] rounded-full
7.9 UserList.vue
Container: p-4
Header:    text-[var(--text-tertiary)] text-sm font-semibold
Count:     bg-[var(--accent-muted)] text-[var(--accent)] text-xs px-1.5 rounded-full
Self row:  border-b border-[var(--border-subtle)] p-2 rounded-lg
           active: bg-[var(--accent-muted)]
Avatar self: bg-[var(--accent)] text-[var(--text-inverse)] rounded-full
Avatar other: bg-[var(--surface-3)] text-[var(--text-secondary)] rounded-full
Name:      text-[var(--text-primary)] text-sm font-semibold
DJ name:   text-[var(--accent)]
Audio bars: bg-[var(--accent)]
7.10 ChatOverlay.vue
Window:       bg-[var(--surface-4)] border-[var(--border-default)] rounded-xl shadow-xl
Header:       bg-[var(--surface-3)] border-b border-[var(--border-default)] rounded-t-xl
Title:        text-[var(--text-secondary)] text-xs font-semibold
Tab active:   text-[var(--text-primary)] bg-[var(--surface-2)] border-b-2 border-[var(--accent)]
Tab inactive: text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]
Messages bg:  bg-[var(--surface-2)]
Self bubble:  bg-[var(--accent-muted)] text-[var(--text-primary)] rounded-2xl rounded-bl-sm
Other bubble: bg-[var(--surface-3)] text-[var(--text-primary)] rounded-2xl rounded-br-sm
Like msg:     bg-[var(--accent-muted)] text-[var(--accent)] rounded-full
System msg:   text-[var(--text-tertiary)] text-[10px]
Input:        bg-[var(--surface-2)] border-[var(--border-default)] rounded-md focus:border-[var(--accent)]
Send btn:     bg-[var(--accent)] text-[var(--text-inverse)] rounded-md
Drag handle:  bg-[var(--surface-4)] border-[var(--border-default)] text-[var(--text-secondary)] rounded-lg shadow-md
Unread:       bg-[var(--accent)] text-[var(--text-inverse)] shadow-lg
- Remove chamfer-br → rounded-xl
7.11 ToastNotification.vue
Container: fixed, top-10, centered, z-[100]
Card:      bg-[var(--surface-elevated)]/95 backdrop-blur-lg text-[var(--text-primary)] rounded-lg shadow-xl border-l-4 min-w-[300px]
Success border-l: border-[var(--success)]
Error border-l:   border-[var(--error)]
Warning border-l: border-[var(--warning)]
Default border-l: border-[var(--text-tertiary)]
Title:     font-semibold text-sm
Message:   text-xs text-[var(--text-secondary)]
7.12 TutorialOverlay.vue
Backdrop:    bg-black/60
Spotlight:   border-2 border-[var(--accent)] shadow-[0_0_20px_rgba(var(--accent-rgb),0.4)]
             REMOVE corner decorators entirely
Tooltip:     bg-[var(--surface-4)] border-[var(--border-default)] p-4 shadow-xl rounded-lg
Step label:  text-[var(--accent)] text-xs font-semibold
Content:     text-[var(--text-primary)] text-sm font-medium
Next button: bg-[var(--accent)] text-[var(--text-inverse)] rounded-md text-xs font-semibold
Skip link:   text-[var(--text-tertiary)] hover:text-[var(--text-primary)]
Arrow:       REMOVE the rotated border-L shape, replace with CSS triangle or just position offset
- Remove chamfer-br → rounded-lg
7.13 App.vue (Start Screen)
Overlay:      bg-[var(--surface-0)]
Title:        text-[var(--text-primary)] text-4xl font-extrabold tracking-tight
Subtitle:     text-[var(--text-tertiary)] font-mono text-xs tracking-wide
Start button: bg-[var(--accent)] text-[var(--text-inverse)] font-bold text-xl rounded-lg
              hover:bg-[var(--accent-hover)] transition-all duration-200
- Remove chamfer-br → rounded-lg
7.14 Lite Mode (in MainLayout.vue)
Background: bg-[var(--surface-0)] with subtle radial gradient from surface-1
Card:       bg-[var(--surface-4)] border-[var(--border-default)] rounded-2xl shadow-2xl p-8
Cover:      w-24 h-24 rounded-xl shadow-lg
Spin ring:  border-[var(--accent)]/20 border-dashed rounded-full
Song info:  text-[var(--text-primary)] text-lg font-bold
Artist:     text-[var(--text-secondary)] text-sm
Exit btn:   bg-[var(--accent)] text-[var(--text-inverse)] rounded-lg shadow-xl
- Remove chamfer-br → rounded-2xl on card, rounded-lg on button
- Remove grid decoration → subtle radial gradient overlay or nothing
- Volume slider styled with CSS variables
---
8. Tailwind Config Changes
Replace tailwind.config.js:
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Surface tokens — values are overridden by CSS custom properties
        // These serve as fallbacks and make Tailwind utilities available
        surface: {
          0: 'var(--surface-0)',
          1: 'var(--surface-1)',
          2: 'var(--surface-2)',
          3: 'var(--surface-3)',
          4: 'var(--surface-4)',
          elevated: 'var(--surface-elevated)',
        },
        // Text tokens
        'txt': {
          primary: 'var(--text-primary)',
          secondary: 'var(--text-secondary)',
          tertiary: 'var(--text-tertiary)',
          inverse: 'var(--text-inverse)',
        },
        // Accent
        accent: {
          DEFAULT: 'var(--accent)',
          hover: 'var(--accent-hover)',
          muted: 'var(--accent-muted)',
          subtle: 'var(--accent-subtle)',
        },
        // Semantic
        success: 'var(--success)',
        warning: 'var(--warning)',
        error: 'var(--error)',
        // Borders
        line: {
          DEFAULT: 'var(--border-default)',
          subtle: 'var(--border-subtle)',
          accent: 'var(--border-accent)',
        },
        // DEPRECATED: keep medical-* temporarily for gradual migration
        medical: {
          50: '#F9FAFB',
          100: '#F3F4F6',
          200: '#E5E7EB',
          300: '#D1D5DB',
          400: '#9CA3AF',
          500: '#6B7280',
          600: '#4B5563',
          700: '#374151',
          800: '#1F2937',
          900: '#111827',
        },
      },
      fontFamily: {
        sans: [
          '"Geist"', '"HarmonyOS Sans SC"', '"HarmonyOS Sans JP"', '"HarmonyOS Sans KR"',
          '"Noto Sans SC"', '"PingFang SC"', '"Hiragino Sans GB"', '"Microsoft YaHei"',
          '"微软雅黑"', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'
        ],
        mono: [
          '"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas',
          '"Liberation Mono"', '"Courier New"', 'monospace'
        ],
      },
    },
  },
  plugins: [],
}
IMPORTANT: Because Tailwind's text-* utility conflicts with custom color names, I used txt as the color prefix for text tokens. Usage: text-txt-primary, text-txt-secondary, text-txt-tertiary.
---
9. CSS Changes (style.css)
@tailwind base;
@tailwind components;
@tailwind utilities;
/* ===== Theme: Light (default) ===== */
:root {
  --surface-0: #ffffff;
  --surface-1: #f8f8f8;
  --surface-2: #f3f3f3;
  --surface-3: #eaeaea;
  --surface-4: #ffffff;
  --surface-elevated: #ffffff;
  --text-primary: #1a1a1a;
  --text-secondary: #6b6b6b;
  --text-tertiary: #a0a0a0;
  --text-inverse: #ffffff;
  --accent: #7c5cbf;
  --accent-hover: #8d6ec8;
  --accent-rgb: 124,92,191;
  --accent-muted: rgba(124,92,191,0.12);
  --accent-subtle: rgba(124,92,191,0.06);
  --border-default: rgba(0,0,0,0.08);
  --border-subtle: rgba(0,0,0,0.04);
  --border-accent: rgba(124,92,191,0.5);
  --success: #16a34a;
  --warning: #ca8a04;
  --error: #dc2626;
  --progress-track: #e5e5e5;
  --scrollbar-thumb: rgba(0,0,0,0.15);
  --scrollbar-thumb-hover: rgba(0,0,0,0.25);
}
/* ===== Theme: Dark ===== */
.dark {
  --surface-0: #121212;
  --surface-1: #1a1a1a;
  --surface-2: #222222;
  --surface-3: #2a2a2a;
  --surface-4: #333333;
  --surface-elevated: #3e3e3e;
  --text-primary: #f0f0f0;
  --text-secondary: #a0a0a0;
  --text-tertiary: #6b6b6b;
  --text-inverse: #ffffff;
  --accent: #d3c2f3;
  --accent-hover: #e0d4f7;
  --accent-rgb: 211,194,243;
  --accent-muted: rgba(211,194,243,0.15);
  --accent-subtle: rgba(211,194,243,0.08);
  --border-default: rgba(255,255,255,0.08);
  --border-subtle: rgba(255,255,255,0.04);
  --border-accent: rgba(211,194,243,0.5);
  --success: #22c55e;
  --warning: #eab308;
  --error: #ef4444;
  --progress-track: #2a2a2a;
  --scrollbar-thumb: rgba(255,255,255,0.15);
  --scrollbar-thumb-hover: rgba(255,255,255,0.3);
}
body {
  @apply bg-[var(--surface-0)] text-[var(--text-primary)] font-sans antialiased overflow-hidden;
}
/* Scrollbar */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track {
  @apply bg-transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 3px;
}
::-webkit-scrollbar-thumb:hover {
  background: var(--scrollbar-thumb-hover);
}
/* REMOVE: .chamfer-br and .chamfer-tl — delete these rules entirely */
/* REMOVE: .animate-scan — delete this rule entirely */
/* REMOVE: .clip-tab in SearchModal scoped CSS */
/* Shimmer animation for loading states */
@keyframes shimmer {
  0% { background-position: -200px 0; }
  100% { background-position: 200px 0; }
}
.animate-shimmer {
  background: linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.04) 50%, transparent 100%);
  background-size: 200px 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}
/* Vue transition: slide-fade (replace scale+opacity with translateY+opacity) */
.slide-fade-enter-active {
  transition: all 0.15s ease-out;
}
.slide-fade-leave-active {
  transition: all 0.1s ease-in;
}
.slide-fade-enter-from {
  opacity: 0;
  transform: translateY(4px);
}
.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
/* Range input styling for dark/light mode */
input[type="range"] {
  -webkit-appearance: none;
  appearance: none;
  background: transparent;
  cursor: pointer;
}
input[type="range"]::-webkit-slider-runnable-track {
  background: var(--progress-track);
  height: 4px;
  border-radius: 2px;
}
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  height: 12px;
  width: 12px;
  border-radius: 50%;
  background: var(--accent);
  margin-top: -4px;
}
input[type="range"]::-moz-range-track {
  background: var(--progress-track);
  height: 4px;
  border-radius: 2px;
}
input[type="range"]::-moz-range-thumb {
  height: 12px;
  width: 12px;
  border-radius: 50%;
  background: var(--accent);
  border: none;
}
---
10. index.html Changes
Replace the Google Fonts link:
<!-- Current: -->
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&family=Noto+Sans+SC:wght@400;500;700;900&display=swap" rel="stylesheet">
<!-- Replace with: -->
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&family=Noto+Sans+SC:wght@400;500;700;900&display=swap" rel="stylesheet">
<!-- Geist font - load from CDN or self-host -->
<link href="https://cdn.jsdelivr.net/fontsource/fonts/geist@latest/index.css" rel="stylesheet">
<!-- HarmonyOS Sans SC - load from public CDN -->
<link href="https://s1.hdslb.com/static/fonts/harmonyos-sans/harmonyos-sans-sc.css" rel="stylesheet">
NOTE: The HarmonyOS Sans CDN URL may need verification. Alternative: self-host the WOFF2 files under public/fonts/ and add a local @font-face declaration in style.css. The implementing agent should verify CDN availability and fall back to self-hosting if needed.
---
11. Store Changes
src/stores/ui.js
Add theme toggle:
import { ref, watch } from 'vue'
export const isDarkMode = ref(localStorage.getItem('theme') !== 'light') // default dark
watch(isDarkMode, (val) => {
  localStorage.setItem('theme', val ? 'dark' : 'light')
  document.documentElement.classList.toggle('dark', val)
}, { immediate: true })
export function toggleDarkMode() {
  isDarkMode.value = !isDarkMode.value
}
src/stores/player.js
Replace medical-* color references if any exist in reactive state (likely none — colors are in templates).
---
12. Implementation Order — Batch 1
Step 1: Foundation (must be done first)
1. Update tailwind.config.js with new color tokens, font families, dark mode class
2. Update style.css with CSS custom properties (light + dark), remove chamfer/scan, add shimmer, update body scrollbar, add transitions
3. Update index.html with new font links
4. Update src/stores/ui.js with isDarkMode and toggleDarkMode
Step 2: Layout Shell
5. Rewrite MainLayout.vue — surface colors, remove borders, add theme toggle button, remove chamfer
6. Update App.vue start screen — dark first, theme-aware
Step 3: Bottom Player
7. Rewrite PlayerControl.vue — glass bar, rounded-full play button, thicker progress bar, dark theme colors, remove chamfer
Step 4: Search Modal  
8. Rewrite SearchModal.vue — dark modal, pill tabs, remove chamfer and clip-tab, rounded list items
Step 5: Auth + Name Prompt
9. Rewrite AuthOverlay.vue — dark card, rounded-2xl, remove accent bar
10. Rewrite NamePromptModal.vue — same style
Step 6: Misc Components (can be parallel)
11. Rewrite ToastNotification.vue
12. Rewrite CenterConsole.vue — remove all decorative elements, clean dark bg
13. Update CoverImage.vue — dark fallback, remove scanline
14. Update QueueList.vue — surface colors, no borders
15. Update QueueItem.vue — bg-based hierarchy, rounded-lg
16. Update UserList.vue — surface colors, rounded avatars
17. Update ChatOverlay.vue — surface-4 window, rounded bubbles
18. Update TutorialOverlay.vue — simplified tooltip, no corner deco
---
13. Hardcoded Colors to Find and Replace
Search the entire music-party-web/src/ directory for these patterns and replace with CSS variable equivalents:
Find
bg-medical-50
bg-medical-100
bg-medical-200 (as bg)
bg-medical-300 (as bg)
bg-white
bg-medical-900
text-medical-900
text-medical-800
text-medical-800/60
text-medical-500
text-medical-400
text-medical-300
border-medical-200
border-medical-100
border-medical-300
bg-accent or bg-accent/DEFAULT
bg-accent-hover
hover:bg-accent
text-accent
bg-accent/10
bg-accent/5
border-accent
chamfer-br
chamfer-tl
clip-tab (in SearchModal scoped CSS)
shadow-2xl
bg-red-500/bg-red-600 (admin mode)
text-red-500
---
## 14. Scoped CSS in Components
Several components have `<style scoped>` sections with hardcoded colors. These must also be updated:
- **MainLayout.vue**: Range input thumb colors (`#111827`, `#ff5722`, `#e5e7eb`) → use CSS variables
- **SearchModal.vue**: `.clip-tab` class → DELETE entirely
- **ChatOverlay.vue**: `rgba(249,115,22,0.6)` glow → `rgba(var(--accent-rgb),0.6)`
- **PlayerControl.vue**: If any scoped colors, update similarly
- **CenterConsole.vue**: Hardcoded rgba values in grid overlays → remove (deleting the grids entirely)
---
15. Theme Toggle Button (to add in MainLayout.vue Header)
<template>
  <!-- Add next to lite mode toggle in the header actions area -->
  <button
    @click="toggleDarkMode"
    class="p-2 rounded-md border border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--surface-3)] transition-all duration-200"
    :title="isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'"
  >
    <Sun v-if="isDarkMode" class="w-4 h-4" />
    <Moon v-else class="w-4 h-4" />
  </button>
</template>
<script setup>
import { Sun, Moon } from 'lucide-vue-next'
import { isDarkMode, toggleDarkMode } from '@/stores/ui'
</script>
---
END OF HANDOFF DOCUMENT
This document contains everything needed to implement the UI redesign: colors, typography, spacing, component-by-component specifications, file change order, CSS variable definitions, Tailwind config, and detailed find-replace mappings.
---