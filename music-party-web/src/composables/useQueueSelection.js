import { computed, ref, watch } from 'vue';

export const useQueueSelection = (queueRef) => {
    const selectionMode = ref(false);
    const selectedQueueIds = ref(new Set());

    const selectedCount = computed(() => selectedQueueIds.value.size);
    const selectedIds = computed(() => Array.from(selectedQueueIds.value));
    const hasSelection = computed(() => selectedCount.value > 0);

    const replaceSelection = (nextIds) => {
        selectedQueueIds.value = new Set(nextIds);
    };

    const enterSelectionMode = (queueId) => {
        selectionMode.value = true;
        if (queueId) {
            replaceSelection([...selectedQueueIds.value, queueId]);
        }
    };

    const exitSelectionMode = () => {
        selectionMode.value = false;
        replaceSelection([]);
    };

    const toggleSelectionMode = () => {
        if (selectionMode.value) {
            exitSelectionMode();
        } else {
            selectionMode.value = true;
        }
    };

    const toggleSelected = (queueId) => {
        if (!queueId) return;
        const next = new Set(selectedQueueIds.value);
        if (next.has(queueId)) {
            next.delete(queueId);
        } else {
            next.add(queueId);
        }
        selectedQueueIds.value = next;
    };

    const isSelected = (queueId) => selectedQueueIds.value.has(queueId);

    const selectAll = () => {
        const queue = queueRef.value || [];
        replaceSelection(queue.map(item => item.queueId).filter(Boolean));
    };

    const clearSelection = () => {
        replaceSelection([]);
    };

    watch(queueRef, (queue) => {
        const validIds = new Set((queue || []).map(item => item.queueId));
        const next = new Set([...selectedQueueIds.value].filter(id => validIds.has(id)));
        if (next.size !== selectedQueueIds.value.size) {
            selectedQueueIds.value = next;
        }
    });

    return {
        selectionMode,
        selectedQueueIds,
        selectedCount,
        selectedIds,
        hasSelection,
        enterSelectionMode,
        exitSelectionMode,
        toggleSelectionMode,
        toggleSelected,
        isSelected,
        selectAll,
        clearSelection
    };
};
