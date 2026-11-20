<template>
  <section class="qp-module" :aria-label="t('queue.title')">
    <!-- Single 46px Flat Underline Top Bar -->
    <header class="qp-header">
      <nav ref="tabsNavRef" class="qp-tabs" role="tablist" :aria-label="t('queue.title')">
        <button
          :ref="(el) => setTabRef('queue', el)"
          type="button"
          role="tab"
          class="qp-tab"
          :class="{ 'is-active': activeTab === 'queue' }"
          :aria-selected="activeTab === 'queue'"
          @click="activeTab = 'queue'"
        >
          <span>{{ t('queue.upNext') }}</span>
          <span class="qp-count">{{ formatCount(queue.length) }}</span>
        </button>

        <button
          :ref="(el) => setTabRef('playlists', el)"
          type="button"
          role="tab"
          class="qp-tab"
          :class="{ 'is-active': activeTab === 'playlists' }"
          :aria-selected="activeTab === 'playlists'"
          @click="activeTab = 'playlists'"
        >
          <span>{{ t('layout.modules.playlists') }}</span>
          <span class="qp-count">{{ formatCount(playlistTabCount) }}</span>
        </button>

        <span
          v-if="tabIndicator.ready"
          class="qp-tab-indicator"
          aria-hidden="true"
          :style="tabIndicatorStyle"
        />
      </nav>

      <!-- Contextual Right Actions -->
      <div class="qp-header-actions">
        <template v-if="activeTab === 'queue'">
          <button
            type="button"
            class="qp-icon-btn"
            :disabled="!commands.canOperate || queue.length < 2"
            :aria-label="t('player.shuffle')"
            :title="t('player.shuffle')"
            @click="commands.toggleShuffle()"
          >
            <Shuffle :size="17" :stroke-width="1.75" aria-hidden="true" />
          </button>
          <button
            v-if="queue.length > 0"
            type="button"
            class="qp-icon-btn"
            :class="{ 'is-accent': queueSelectionMode }"
            :aria-label="t('queue.selectTracks')"
            :title="t('queue.selectTracks')"
            :aria-pressed="queueSelectionMode"
            @click="queuePaneRef?.toggleSelectionMode()"
          >
            <ListChecks :size="17" :stroke-width="1.75" aria-hidden="true" />
          </button>
        </template>
      </div>
    </header>

    <p v-if="errorMessage || playlistStore.error" class="qp-error-bar" role="alert">
      {{ errorMessage || playlistStore.error }}
    </p>

    <Transition name="qp-tab-fade" mode="out-in">
      <QueueTabPane
        v-if="activeTab === 'queue'"
        ref="queuePaneRef"
        key="queue"
        @update:selection-mode="queueSelectionMode = $event"
      />
      <PlaylistsTabPane
        v-else
        ref="playlistsPaneRef"
        key="playlists"
        v-model:scope="scope"
        @update:busy="playlistsBusy = $event"
        @update:error="errorMessage = $event"
      />
    </Transition>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { ListChecks, Shuffle } from 'lucide-vue-next'
import type { PlaylistScope } from '../../domains/playlists/playlistService'
import { useRoomCommandStore } from '../../domains/realtime/roomCommandStore'
import { useRoomRuntimeStore } from '../../domains/realtime/roomRuntimeStore'
import { useRoomPlaylistsStore } from '../../stores/roomPlaylists'
import { useUserPlaylistsStore } from '../../stores/userPlaylists'
import QueueTabPane from './queue-playlists/QueueTabPane.vue'
import PlaylistsTabPane from './queue-playlists/PlaylistsTabPane.vue'
import './queue-playlists/qp-shared.css'

const commands = useRoomCommandStore()
const runtime = useRoomRuntimeStore()
const roomPlaylists = useRoomPlaylistsStore()
const personalPlaylists = useUserPlaylistsStore()
const { t } = useI18n()

type TabId = 'queue' | 'playlists'
const activeTab = ref<TabId>('queue')
const scope = ref<PlaylistScope['type']>('room')
const queueSelectionMode = ref(false)
const playlistsBusy = ref(false)
const errorMessage = ref('')

const queuePaneRef = ref<InstanceType<typeof QueueTabPane> | null>(null)
const playlistsPaneRef = ref<InstanceType<typeof PlaylistsTabPane> | null>(null)

const queue = computed(() => runtime.queue)
const playlistStore = computed(() => (scope.value === 'room' ? roomPlaylists : personalPlaylists))
const playlistTabCount = computed(() => {
  const base = playlistStore.value.playlists.length
  if (scope.value === 'personal' && !playlistStore.value.playlists.some(p => p.systemKey === 'liked-songs')) {
    return base + 1
  }
  return base
})

const tabsNavRef = ref<HTMLElement | null>(null)
const tabRefs = new Map<TabId, HTMLElement>()
const tabIndicator = ref({ left: 0, width: 0, ready: false })
let tabsResizeObserver: ResizeObserver | null = null

const setTabRef = (tab: TabId, el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) tabRefs.set(tab, el)
  else tabRefs.delete(tab)
}

const updateTabIndicator = () => {
  const el = tabRefs.get(activeTab.value)
  if (!el) return
  tabIndicator.value = {
    left: el.offsetLeft + 6,
    width: Math.max(0, el.offsetWidth - 12),
    ready: true,
  }
}

const tabIndicatorStyle = computed(() => ({
  transform: `translateX(${tabIndicator.value.left}px)`,
  width: `${tabIndicator.value.width}px`,
}))

watch(
  [activeTab, () => queue.value.length, playlistTabCount],
  async () => {
    await nextTick()
    updateTabIndicator()
  },
)

watch(activeTab, () => {
  errorMessage.value = ''
})

const formatCount = (count: number) => (count > 99 ? '99+' : String(count))

onMounted(() => {
  void nextTick(updateTabIndicator)
  if (tabsNavRef.value && typeof ResizeObserver !== 'undefined') {
    tabsResizeObserver = new ResizeObserver(updateTabIndicator)
    tabsResizeObserver.observe(tabsNavRef.value)
  }
  void roomPlaylists.loadPlaylists()
})

onBeforeUnmount(() => {
  tabsResizeObserver?.disconnect()
  tabsResizeObserver = null
})
</script>

<style scoped>
.qp-module {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  background: var(--surface-panel);
  color: var(--text-primary);
  overflow: hidden;
}

.qp-module :where(button, input, select, textarea) {
  font-family: inherit;
  color: inherit;
  background: transparent;
  border: 0;
}

.qp-module button {
  cursor: pointer;
}

.qp-module button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.qp-module :is(button, input, select, textarea, [tabindex]):focus-visible {
  outline: 2px solid var(--border-accent);
  outline-offset: 2px;
}

.qp-header {
  height: 46px;
  padding: 0 10px 0 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.qp-tabs {
  position: relative;
  display: flex;
  align-items: stretch;
  height: 100%;
  gap: 2px;
  min-width: 0;
}

.qp-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 9px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-tertiary);
  white-space: nowrap;
  transition: color var(--motion-enter) var(--motion-ease-out);
}

.qp-tab:hover {
  color: var(--text-secondary);
}

.qp-tab.is-active {
  color: var(--text-primary);
  font-weight: 600;
}

.qp-tab-indicator {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  border-radius: var(--radius-full);
  background: var(--accent);
  pointer-events: none;
  transition: transform var(--motion-panel) var(--motion-ease-standard),
    width var(--motion-panel) var(--motion-ease-standard);
}

.qp-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 17px;
  height: 16px;
  padding: 0 5px;
  border-radius: var(--radius-full);
  font-size: 10.5px;
  font-weight: 600;
  background: var(--surface-control);
  color: var(--text-tertiary);
  transition: background var(--motion-enter) var(--motion-ease-out),
    color var(--motion-enter) var(--motion-ease-out);
}

.qp-tab.is-active .qp-count {
  background: var(--accent-muted);
  color: var(--accent);
}

.qp-header-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

@container (max-width: 315px) {
  .qp-header {
    padding: 0 6px 0 8px;
  }

  .qp-tab {
    padding: 0 6px;
    font-size: 12px;
    gap: 4px;
  }

  .qp-count {
    min-width: 15px;
    height: 15px;
    padding: 0 4px;
    font-size: 10px;
  }
}
</style>
