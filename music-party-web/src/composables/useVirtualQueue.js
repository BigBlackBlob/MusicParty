import { computed, onBeforeUnmount, ref } from 'vue';

// Keep the first rows mounted so the full interactive area remains sortable.
export const QUEUE_INTERACTIVE_LIMIT = 50;
const VIRTUALIZE_AFTER = 80;
const ROW_HEIGHT = 72;
const OVERSCAN = 6;

export const useVirtualQueue = (queue) => {
  const scrollTop = ref(0);
  const viewportHeight = ref(0);
  let resizeObserver = null;

  const virtualized = computed(() => queue.value.length > VIRTUALIZE_AFTER);
  const interactiveQueue = computed(() => virtualized.value ? queue.value.slice(0, QUEUE_INTERACTIVE_LIMIT) : queue.value);
  const remainingQueue = computed(() => virtualized.value ? queue.value.slice(QUEUE_INTERACTIVE_LIMIT) : []);
  const start = computed(() => Math.max(0, Math.floor(Math.max(0, scrollTop.value - QUEUE_INTERACTIVE_LIMIT * ROW_HEIGHT) / ROW_HEIGHT) - OVERSCAN));
  const count = computed(() => Math.ceil(viewportHeight.value / ROW_HEIGHT) + OVERSCAN * 2);
  const visibleQueue = computed(() => remainingQueue.value.slice(start.value, start.value + count.value));
  const topPadding = computed(() => start.value * ROW_HEIGHT);
  const bottomPadding = computed(() => Math.max(0, (remainingQueue.value.length - start.value - visibleQueue.value.length) * ROW_HEIGHT));

  const onScroll = (event) => {
    const element = event.currentTarget;
    scrollTop.value = element.scrollTop;
    viewportHeight.value = element.clientHeight;
  };

  const observeScroller = (element) => {
    resizeObserver?.disconnect();
    resizeObserver = null;
    if (!element) return;
    viewportHeight.value = element.clientHeight;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => { viewportHeight.value = element.clientHeight; });
      resizeObserver.observe(element);
    }
  };

  onBeforeUnmount(() => resizeObserver?.disconnect());

  return { virtualized, interactiveQueue, visibleQueue, topPadding, bottomPadding, onScroll, observeScroller };
};
