import { onMounted, onUnmounted } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useUiStore } from '../stores/ui';

export function useShortcuts(actions = {}) {
  const player = usePlayerStore();
  const ui = useUiStore();

  const handleKeyDown = (e) => {
    // Ignore if typing in an input or textarea
    if (['INPUT', 'TEXTAREA'].includes(e.target.tagName) || e.target.isContentEditable) {
      return;
    }

    const key = e.key.toLowerCase();
    const isMod = e.metaKey || e.ctrlKey;

    switch (key) {
      case ' ': // Space: Play/Pause
        e.preventDefault();
        player.togglePause();
        break;
      
      case 'l': // L: Like
        player.sendLike();
        break;

      case 's': // S: Shuffle
        player.toggleShuffle();
        break;

      case 'n': // N: Next
      case 'arrowright':
        if (isMod) player.playNext();
        break;

      case '/': // /: Search
        e.preventDefault();
        actions.onSearch?.();
        break;

      case 'm': // M: Mute
        ui.setVolume(ui.volume > 0 ? 0 : 0.5);
        break;

      case 'escape': // Esc: Close modals or exit Lite mode
        if (ui.isLiteMode) {
          ui.toggleLiteMode();
        } else {
          actions.onCloseModals?.();
        }
        break;
    }
  };

  onMounted(() => {
    window.addEventListener('keydown', handleKeyDown);
  });

  onUnmounted(() => {
    window.removeEventListener('keydown', handleKeyDown);
  });

  return {
    shortcuts: [
      { key: 'Space', label: '播放 / 暂停' },
      { key: 'L', label: '喜欢歌曲' },
      { key: 'S', label: '随机播放' },
      { key: 'Ctrl + →', label: '下一首' },
      { key: '/', label: '快速搜索' },
      { key: 'M', label: '静音切换' },
      { key: 'Esc', label: '退出模式 / 关闭' },
    ]
  };
}
