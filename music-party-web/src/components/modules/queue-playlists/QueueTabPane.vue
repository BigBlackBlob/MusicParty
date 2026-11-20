<template>
  <div class="qp-pane">
    <Transition name="qp-slide">
      <div v-if="selectionMode" class="qp-selection-bar">
        <span class="qp-selection-count">{{ selectedCount }} {{ t('queue.selected') }}</span>
        <button type="button" class="qp-text-btn" @click="selectAll">{{ t('queue.all') }}</button>
        <button type="button" class="qp-text-btn" :disabled="!hasSelection" @click="batchTop">{{ t('queue.top') }}</button>
        <button type="button" class="qp-text-btn qp-text-btn--danger" :disabled="!hasSelection" @click="batchRemove">{{ t('queue.remove') }}</button>
      </div>
    </Transition>

    <div
      ref="queueScrollerRef"
      class="qp-scroll"
      :class="{ 'qp-scroll--virtualized': virtualized }"
      @scroll.passive="onScroll"
    >
      <div v-if="queue.length === 0" class="qp-empty">
        <span class="qp-empty-icon" aria-hidden="true">
          <ListMusic :size="22" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="qp-empty-title">{{ t('queue.empty') }}</div>
        <div class="qp-empty-desc">{{ t('queue.emptyDesc') }}</div>
        <button
          v-if="openSearch"
          type="button"
          class="qp-primary-btn"
          @click="openSearch()"
        >
          {{ t('search.searchAndAdd') }}
        </button>
      </div>

      <div v-else class="qp-track-stack">
        <div ref="queueListRef" class="qp-track-stack">
          <QueueItem
            v-for="(item, index) in interactiveQueue"
            :key="item.queueId || `${item.music?.platform || 'track'}:${item.music?.id || index}`"
            :data-queue-id="item.queueId"
            :item="item"
            class="h-[66px] shrink-0"
            :index="Number(index)"
            :selection-mode="selectionMode"
            :selected="isSelected(item.queueId)"
            @toggle-select="toggleSelected(item.queueId)"
          />
        </div>
        <div v-if="virtualized" class="flex flex-col">
          <div aria-hidden="true" :style="{ height: `${topPadding}px` }"></div>
          <QueueItem
            v-for="(item, index) in visibleQueue"
            :key="item.queueId || `${item.music?.platform || 'track'}:${item.music?.id || index}`"
            :item="item"
            class="mb-[6px] h-[66px] shrink-0"
            :index="50 + Math.floor(topPadding / 72) + Number(index)"
            :selection-mode="selectionMode"
            :selected="isSelected(item.queueId)"
            @toggle-select="toggleSelected(item.queueId)"
          />
          <div aria-hidden="true" :style="{ height: `${bottomPadding}px` }"></div>
        </div>
      </div>
    </div>

    <footer class="qp-bottom-bar">
      <span class="qp-bottom-meta">{{ queue.length }} {{ t('queue.tracks') }}</span>
      <div class="qp-bottom-actions">
        <button
          type="button"
          class="qp-bar-btn"
          :class="{ 'is-accent': selectionMode }"
          :disabled="queue.length === 0"
          @click="toggleSelectionMode"
        >
          {{ t('queue.selectTracks') }}
        </button>
        <button
          type="button"
          class="qp-bar-btn"
          :disabled="queue.length === 0"
          @click="exportQueueSongs"
        >
          {{ t('common.export') }}
        </button>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ListMusic } from 'lucide-vue-next'
import Sortable from 'sortablejs'
import { useRoomCommandStore } from '../../../domains/realtime/roomCommandStore'
import { useRoomRuntimeStore } from '../../../domains/realtime/roomRuntimeStore'
import { useQueueSelection } from '../../../composables/useQueueSelection'
import { useVirtualQueue } from '../../../composables/useVirtualQueue'
import {
  buildQueueReorderPayload,
  buildQueueReorderPayloadFromDom,
  isQueueReorderSourceCurrent,
  restoreDraggedNode,
} from '../../../utils/queueReorder'
import QueueItem from '../../QueueItem.vue'

const emit = defineEmits<{
  'update:selectionMode': [value: boolean]
}>()

const commands = useRoomCommandStore()
const runtime = useRoomRuntimeStore()
const openSearch = inject<(() => void) | null>('openSearch', null)
const { t } = useI18n()

const queue = computed(() => runtime.queue)
const queueListRef = ref<HTMLElement | null>(null)
const queueScrollerRef = ref<HTMLElement | null>(null)
let sortableInstance: Sortable | null = null
let dragStartedInInteractiveZone = false
let dragAnchor: Node | null = null

const {
  virtualized,
  interactiveQueue,
  visibleQueue,
  topPadding,
  bottomPadding,
  onScroll,
  observeScroller,
} = useVirtualQueue(queue)

const {
  selectionMode,
  selectedCount,
  selectedIds,
  hasSelection,
  toggleSelectionMode,
  exitSelectionMode,
  toggleSelected,
  isSelected,
  selectAll,
} = useQueueSelection(queue)

watch(selectionMode, (val) => emit('update:selectionMode', val), { immediate: true })

const dragDisabled = computed(() => selectionMode.value || !commands.canOperate)

const destroySortable = () => {
  if (!sortableInstance) return
  sortableInstance.destroy()
  sortableInstance = null
}

const initSortable = () => {
  if (!queueListRef.value) return
  sortableInstance = new Sortable(queueListRef.value, {
    animation: 160,
    ghostClass: 'mp-drag-ghost',
    chosenClass: 'mp-drag-chosen',
    dragClass: 'mp-drag-active',
    disabled: dragDisabled.value,
    scroll: queueScrollerRef.value ?? true,
    scrollSensitivity: 80,
    scrollSpeed: 20,
    onChoose: (evt) => {
      dragAnchor = evt.item.nextSibling
    },
    onStart: (evt) => {
      dragStartedInInteractiveZone = (evt.oldIndex ?? 999) < 50
    },
    onMove: (evt) => {
      if (!queueListRef.value) return false
      return dragStartedInInteractiveZone && Array.from(queueListRef.value.children).indexOf(evt.related) < 50
    },
    onEnd: (evt) => {
      if (!dragStartedInInteractiveZone) {
        restoreDraggedNode(evt, dragAnchor)
        dragAnchor = null
        return
      }
      const payload = buildQueueReorderPayloadFromDom(evt)
        || (isQueueReorderSourceCurrent(queue.value, evt)
          ? buildQueueReorderPayload(queue.value, evt.oldIndex, evt.newIndex)
          : null)
      restoreDraggedNode(evt, dragAnchor)
      dragAnchor = null
      if (payload && payload.oldIndex < 50 && payload.newIndex < 50) {
        commands.reorderQueue(
          payload.oldIndex,
          payload.newIndex,
          payload.queueId,
          payload.targetQueueId,
          payload.position,
        )
      } else if (evt.oldIndex !== evt.newIndex) {
        commands.requestResync('queue-reorder-stale-dom', true)
      }
      dragStartedInInteractiveZone = false
    },
  })
}

watch(queueScrollerRef, (el) => {
  if (el) observeScroller(el)
})

watch([dragDisabled, queueListRef], async () => {
  await nextTick()
  if (sortableInstance && sortableInstance.el !== queueListRef.value) destroySortable()
  if (!sortableInstance) {
    initSortable()
  } else {
    sortableInstance.option('disabled', dragDisabled.value)
  }
})

onMounted(async () => {
  await nextTick()
  initSortable()
})

onBeforeUnmount(() => {
  destroySortable()
})

const formatArtists = (artists?: string[] | null) => (
  Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist')
)

const batchTop = () => {
  const ids = selectedIds.value
  if (!ids.length) return
  commands.topSongs(ids)
  exitSelectionMode()
}

const batchRemove = () => {
  const ids = selectedIds.value
  if (!ids.length) return
  commands.removeSongs(ids)
  exitSelectionMode()
}

const exportQueueSongs = () => {
  if (!queue.value.length) return
  const lines = queue.value.map((item, idx) => {
    const name = item.music?.name || ''
    const artists = formatArtists(item.music?.artists)
    return `${idx + 1}. ${name} - ${artists}`
  })
  const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `musicparty-queue-${new Date().toISOString().slice(0, 10)}.txt`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

defineExpose({
  toggleSelectionMode,
})
</script>

<style scoped>
.qp-scroll--virtualized {
  /* Keep the top inset outside the scrollTop used by useVirtualQueue. */
  margin-top: var(--spacing-sm);
  padding-top: 0;
}
</style>
