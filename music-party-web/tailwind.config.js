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
        surface: {
          0: 'var(--surface-0)',
          1: 'var(--surface-1)',
          2: 'var(--surface-2)',
          3: 'var(--surface-3)',
          4: 'var(--surface-4)',
          elevated: 'var(--surface-elevated)',
        },
        txt: {
          primary: 'var(--text-primary)',
          secondary: 'var(--text-secondary)',
          tertiary: 'var(--text-tertiary)',
          inverse: 'var(--text-inverse)',
        },
        accent: {
          DEFAULT: 'var(--accent)',
          hover: 'var(--accent-hover)',
          muted: 'var(--accent-muted)',
          subtle: 'var(--accent-subtle)',
        },
        line: {
          DEFAULT: 'var(--border-default)',
          subtle: 'var(--border-subtle)',
          accent: 'var(--border-accent)',
        },
        success: 'var(--success)',
        warning: 'var(--warning)',
        error: 'var(--error)',
        progress: {
          track: 'var(--progress-track)',
        }
      },
      fontFamily: {
        sans: [
          '"Geist"',
          '"HarmonyOS Sans SC"',
          '"HarmonyOS Sans JP"',
          '"HarmonyOS Sans KR"',
          '"Noto Sans SC"',
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei"',
          '"微软雅黑"',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'sans-serif'
        ],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', '"Liberation Mono"', '"Courier New"', 'monospace'],
      }
    },
  },
  plugins: [],
}
