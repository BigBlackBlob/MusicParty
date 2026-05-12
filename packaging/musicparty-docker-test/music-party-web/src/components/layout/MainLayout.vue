<!-- src/components/layout/MainLayout.vue -->
<script setup>
import { computed, ref, onMounted } from 'vue';
import { Search, Users, ListMusic, MessageSquare, X, Minimize2, Maximize2, Volume2, Activity, Moon, Sun } from 'lucide-vue-next';
import UserList from '../UserList.vue';
import QueueList from '../QueueList.vue';
import CoverImage from '../CoverImage.vue';
import { useUserStore } from '../../stores/user';
import { useUiStore } from '../../stores/ui';
import { usePlayerStore } from '../../stores/player';

const emit = defineEmits(['search', 'toggle-mobile-chat']);
const userStore = useUserStore();
const uiStore = useUiStore();
const playerStore = usePlayerStore();

onMounted(() => {
  uiStore.fetchConfig();
});

const mobileQueueOpen = ref(false);
const mobileUserOpen = ref(false);
const mobileActivePanel = computed(() => {
  if (mobileQueueOpen.value) return 'queue';
  if (mobileUserOpen.value) return 'users';
  return '';
});

const toggleMobileQueue = () => {
  mobileQueueOpen.value = !mobileQueueOpen.value;
  if(mobileQueueOpen.value) mobileUserOpen.value = false;
};

const toggleMobileUser = () => {
  mobileUserOpen.value = !mobileUserOpen.value;
  if(mobileUserOpen.value) mobileQueueOpen.value = false;
};

const handleSearchClick = () => {
  emit('search');
}

const handleMobileChat = () => {
  emit('toggle-mobile-chat');
};
</script>

<template>
    <div class="h-[100dvh] w-screen relative flex flex-col overflow-hidden bg-[var(--surface-0)]">
    <a href="#main-content" class="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-4 focus:z-[var(--z-skip-link)] focus:rounded-xl focus:bg-[var(--accent)] focus:px-4 focus:py-3 focus:text-sm focus:font-semibold focus:text-[var(--text-inverse)] focus:shadow-lg focus:outline-none">跳至主内容</a>
    <!-- 1. 顶部栏 Header -->
    <header v-if="!uiStore.isLiteMode" class="h-14 bg-[var(--surface-1)] border-b border-[var(--border-default)] flex justify-between items-center px-4 md:px-6 flex-shrink-0 relative z-[var(--z-header)]">
      <div class="flex items-center gap-2 flex-shrink-0">
        <div class="w-2.5 h-2.5 md:w-3 md:h-3 rounded-full bg-[var(--accent)] flex-shrink-0"></div>
        <div class="flex items-baseline gap-1">
          <a href="https://github.com/PluvIIter/MusicParty" target="_blank" class="font-bold text-base md:text-xl tracking-tight text-[var(--text-primary)] whitespace-nowrap hover:text-[var(--accent)] transition-colors">MUSIC PARTY</a>
          <span class="text-[var(--text-tertiary)] font-mono font-normal text-[10px] md:text-xs whitespace-nowrap">by {{ uiStore.authorName }}</span>
        </div>
      </div>

      <div class="flex items-center gap-2 md:gap-4">
        <!-- 移动端：显示人数按钮 -->
        <button
          id="tutorial-rename-mobile"
          @click="toggleMobileUser"
          class="md:hidden relative flex min-w-[44px] min-h-[44px] items-center justify-center bg-[var(--surface-2)] border border-[var(--border-default)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] active:scale-[0.96] transition-colors overflow-hidden group rounded-lg transform-gpu"
          aria-label="在线成员"
          :class="{ 'bg-[var(--surface-3)] text-[var(--text-primary)] border-[var(--border-accent)]': mobileUserOpen }"
        >
          <span
              class="absolute inset-0 flex items-center justify-center font-black text-4xl leading-none text-[var(--accent-muted)] pointer-events-none z-0 select-none scale-110 font-mono"
          >
            {{ userStore.onlineUsers.length > 9 ? 'N' : userStore.onlineUsers.length }}
          </span>
          <Users class="w-5 h-5 relative z-10" />
        </button>

        <button
            @click="uiStore.toggleDarkMode"
            class="flex items-center justify-center min-w-[44px] min-h-[44px] border border-[var(--border-default)] bg-[var(--surface-2)] hover:bg-[var(--surface-3)] active:scale-[0.96] text-[var(--text-secondary)] transition-all rounded-lg"
            :title="uiStore.isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'"
            :aria-label="uiStore.isDarkMode ? '切换到浅色模式' : '切换到深色模式'"
        >
          <Sun v-if="uiStore.isDarkMode" class="w-4 h-4" />
          <Moon v-else class="w-4 h-4" />
        </button>

        <!-- 精简模式按钮 -->
        <button
            @click="uiStore.toggleLiteMode"
            class="flex items-center justify-center min-w-[44px] min-h-[44px] border border-[var(--border-default)] bg-[var(--surface-2)] hover:bg-[var(--surface-3)] active:scale-[0.96] text-[var(--text-secondary)] transition-all rounded-lg"
            title="精简模式"
            aria-label="切换精简模式"
        >
          <Minimize2 class="w-4 h-4" />
        </button>

        <!-- 搜索按钮 -->
        <button id="tutorial-search" @click="handleSearchClick" class="flex items-center justify-center min-w-[44px] min-h-[44px] md:w-auto md:h-9 md:px-4 border border-[var(--accent)] bg-[var(--accent)] hover:bg-[var(--accent-hover)] active:scale-[0.96] font-semibold text-sm text-[var(--text-inverse)] transition-all rounded-lg gap-2" aria-label="打开搜索">
          <Search class="w-4 h-4" />
          <span class="hidden md:inline">SEARCH</span>
        </button>
      </div>
    </header>

    <!-- 2. 主体内容区 -->
    <div v-if="!uiStore.isLiteMode" class="flex-1 flex overflow-hidden relative">
      <aside class="hidden w-[clamp(14rem,18vw,18rem)] flex-shrink-0 bg-[var(--surface-1)] border-r border-[var(--border-subtle)] md:block overflow-y-auto">
        <UserList />
      </aside>

      <main id="main-content" class="min-w-0 flex-1 bg-[var(--surface-2)] relative flex flex-col overflow-hidden z-[var(--z-content)]">
        <slot></slot>
      </main>

      <aside class="hidden w-[clamp(20rem,24vw,28rem)] flex-shrink-0 bg-[var(--surface-1)] border-l border-[var(--border-subtle)] lg:block overflow-hidden">
        <QueueList />
      </aside>

      <div class="md:hidden fixed inset-x-4 bottom-4 z-[var(--z-overlay)]">
        <div class="mx-auto flex max-w-md items-center justify-between gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)]/95 px-2 py-2 shadow-2xl backdrop-blur-xl">
          <button @click="handleSearchClick" class="flex min-h-[44px] flex-1 items-center justify-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold text-[var(--text-primary)] active:scale-[0.96]" aria-label="搜索">
            <Search class="h-4 w-4 text-[var(--accent)]" />
            搜索
          </button>
          <button
            @click="toggleMobileQueue"
            class="flex min-h-[44px] flex-1 items-center justify-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold active:scale-[0.96]"
            :class="mobileActivePanel === 'queue' ? 'bg-[var(--accent-subtle)] text-[var(--accent)]' : 'text-[var(--text-primary)]'"
            aria-label="播放队列"
          >
            <ListMusic class="h-4 w-4 text-[var(--accent)]" />
            队列
          </button>
          <button @click="handleMobileChat" class="flex min-h-[44px] flex-1 items-center justify-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold text-[var(--text-primary)] active:scale-[0.96]" aria-label="聊天">
            <MessageSquare class="h-4 w-4 text-[var(--accent)]" />
            聊天
          </button>
          <button
            @click="toggleMobileUser"
            class="flex min-h-[44px] flex-1 items-center justify-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold active:scale-[0.96]"
            :class="mobileActivePanel === 'users' ? 'bg-[var(--accent-subtle)] text-[var(--accent)]' : 'text-[var(--text-primary)]'"
            aria-label="在线成员"
          >
            <Users class="h-4 w-4 text-[var(--accent)]" />
            成员
          </button>
        </div>
      </div>

      <Transition name="slide-fade">
        <div v-if="mobileQueueOpen" class="md:hidden absolute inset-0 bg-[var(--surface-0)] z-[var(--z-overlay)] pt-4 overflow-y-auto">
          <div class="px-4 pb-2 border-b border-[var(--border-subtle)] mb-2 flex justify-between">
            <span class="text-xs font-mono text-[var(--text-tertiary)]">播放队列</span>
            <button @click="mobileQueueOpen = false" class="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-lg active:scale-[0.96]" aria-label="关闭播放队列"><X class="w-4 h-4"/></button>
          </div>
          <QueueList />
        </div>
      </Transition>

      <Transition name="slide-fade">
        <div v-if="mobileUserOpen" class="md:hidden absolute inset-0 bg-[var(--surface-0)] z-[var(--z-overlay)] pt-4 overflow-y-auto">
          <div class="px-4 pb-2 border-b border-[var(--border-subtle)] mb-2 flex justify-between">
            <span class="text-xs font-mono text-[var(--text-tertiary)]">在线成员</span>
            <button @click="mobileUserOpen = false" class="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-lg active:scale-[0.96]" aria-label="关闭在线成员"><X class="w-4 h-4"/></button>
          </div>
          <UserList />
        </div>
      </Transition>
    </div>

    <!-- 3. 精简模式视图 -->
    <div v-else class="flex-1 flex flex-col items-center justify-center bg-[radial-gradient(circle_at_top,rgba(211,194,243,0.08),transparent_40%),var(--surface-0)] relative overflow-hidden p-6">

      <div class="relative z-10 w-full max-w-lg flex flex-col items-center gap-10">
        <!-- 头部状态 (文案简化) -->
        <div class="flex items-center gap-3 text-xs text-[var(--text-tertiary)] tracking-[0.2em] uppercase font-sans">
          <Activity class="w-3 h-3 text-[var(--accent)]" />
          <span>精简模式</span>
        </div>

        <!-- 核心显示卡片 -->
        <div class="w-full bg-[var(--surface-4)] border border-[var(--border-default)] shadow-2xl relative p-8 rounded-2xl">

          <div class="flex flex-col md:flex-row items-center gap-8">
            <!-- 封面区 -->
            <div class="relative flex-shrink-0">
               <div class="w-24 h-24 border border-[var(--border-default)] flex items-center justify-center bg-[var(--surface-2)] shadow-inner overflow-hidden rounded-xl">
                  <CoverImage :src="playerStore.nowPlaying?.music.coverUrl" :alt="playerStore.nowPlaying ? `${playerStore.nowPlaying.music.name} 封面` : '歌曲封面'" loading="eager" class="w-full h-full object-cover" />
                </div>
            </div>

            <!-- 歌曲信息区 -->
            <div class="flex-1 min-w-0 flex flex-col items-center md:items-start text-center md:text-left font-sans">
               <span class="mb-1 text-xs font-mono uppercase tracking-widest text-[var(--accent)]">正在播放</span>
               <h2 class="text-2xl md:text-3xl font-bold text-[var(--text-primary)] tracking-tight leading-tight mb-2 line-clamp-2">
                 {{ playerStore.nowPlaying?.music.name || '系统待机' }}
               </h2>
               <div class="flex items-center gap-2 text-sm font-semibold text-[var(--text-secondary)]">
                 <span class="w-2 h-2 rounded-full bg-[var(--accent)]"></span>
                 {{ playerStore.nowPlaying?.music.artists.join(' / ') || '无内容' }}
               </div>
            </div>
          </div>
        </div>

        <!-- 音量控制 -->
        <div class="w-full max-w-[320px] bg-[var(--surface-4)]/80 backdrop-blur-sm border border-[var(--border-default)] p-4 flex flex-col gap-3 shadow-sm rounded-2xl">
           <div class="flex justify-between items-center text-xs font-mono text-[var(--text-tertiary)] uppercase tracking-wider">
              <span>音量</span>
              <span class="text-[var(--text-primary)] font-bold">{{ Math.round(uiStore.volume * 100) }}%</span>
           </div>
           <div class="flex items-center gap-3">
              <Volume2 class="w-4 h-4 text-[var(--text-tertiary)] flex-shrink-0" />
              <div class="flex-1 flex items-center h-4">
                  <input
                    type="range" min="0" max="1" step="0.01"
                    v-model.number="uiStore.volume"
                    class="w-full h-1 appearance-none cursor-pointer"
                  />
              </div>
           </div>
        </div>
        
        <!-- 后台自动精简开关 (变色优化) -->
        <label class="flex items-center gap-2 cursor-pointer group select-none">
           <div class="relative w-8 h-4 rounded-full transition-colors duration-300"
                :class="uiStore.autoLiteMode ? 'bg-[var(--accent)]' : 'bg-[var(--surface-3)]'">
              <input type="checkbox" v-model="uiStore.autoLiteMode" class="hidden" />
              <div class="absolute left-0.5 top-0.5 w-3 h-3 bg-white rounded-full transition-transform duration-300"
                   :style="{ transform: uiStore.autoLiteMode ? 'translateX(16px)' : 'translateX(0)' }"></div>
           </div>
           <span class="text-xs font-mono text-[var(--text-tertiary)] group-hover:text-[var(--text-secondary)] transition-colors">
              后台播放时自动进入精简模式
           </span>
        </label>

        <!-- 退出动作 -->
        <button
            @click="uiStore.toggleLiteMode"
            class="w-full min-h-[44px] bg-[var(--surface-4)] text-[var(--text-primary)] font-semibold py-4 flex items-center justify-center gap-4 transition-all hover:bg-[var(--surface-3)] active:scale-[0.98] group shadow-xl rounded-2xl border border-[var(--border-default)]"
        >
          <Maximize2 class="w-5 h-5 transition-transform group-hover:scale-110" />
          <span class="text-sm tracking-widest uppercase">退出精简模式</span>
        </button>
      </div>
    </div>

    <slot v-if="!uiStore.isLiteMode" name="player"></slot>
  </div>
</template>

<style scoped>
.slide-fade-enter-active, .slide-fade-leave-active { transition: all 0.3s ease; }
.slide-fade-enter-from, .slide-fade-leave-to { transform: translateY(10px); opacity: 0; }

@media (prefers-reduced-motion: reduce) {
  .slide-fade-enter-active,
  .slide-fade-leave-active {
    transition-duration: 0.01ms;
  }
}
</style>
