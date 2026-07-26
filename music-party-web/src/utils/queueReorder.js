export const buildQueueReorderPayload = (queue, oldIndex, newIndex) => {
  if (!Array.isArray(queue) || oldIndex === newIndex) return null;
  if (oldIndex < 0 || newIndex < 0 || oldIndex >= queue.length || newIndex >= queue.length) return null;

  const moved = queue[oldIndex];
  const target = queue[newIndex];
  if (!moved?.queueId || !target?.queueId) {
    return null;
  }

  return {
    oldIndex,
    newIndex,
    queueId: moved.queueId,
    targetQueueId: target.queueId,
    position: oldIndex < newIndex ? 'after' : 'before'
  };
};

const queueIdOf = (element) => element?.dataset?.queueId || '';

export const isQueueReorderSourceCurrent = (queue, evt) => {
  const movedId = queueIdOf(evt?.item);
  return Boolean(movedId && Array.isArray(queue) && queue[evt.oldIndex]?.queueId === movedId);
};

export const buildQueueReorderPayloadFromDom = (evt) => {
  if (!evt || evt.oldIndex === evt.newIndex) return null;
  const movedId = queueIdOf(evt.item);
  if (!movedId) return null;

  const previousId = queueIdOf(evt.item.previousElementSibling);
  if (previousId) {
    return {
      oldIndex: evt.oldIndex,
      newIndex: evt.newIndex,
      queueId: movedId,
      targetQueueId: previousId,
      position: 'after'
    };
  }

  const nextId = queueIdOf(evt.item.nextElementSibling);
  if (nextId) {
    return {
      oldIndex: evt.oldIndex,
      newIndex: evt.newIndex,
      queueId: movedId,
      targetQueueId: nextId,
      position: 'before'
    };
  }

  return null;
};

export const applyQueueReorder = (queue, { oldIndex, newIndex, queueId, targetQueueId, position = 'before' }) => {
  const snapshot = Array.isArray(queue) ? [...queue] : [];
  if (snapshot.length < 2) return snapshot;

  const sourceIndex = queueId ? snapshot.findIndex(item => item.queueId === queueId) : oldIndex;
  if (sourceIndex < 0 || sourceIndex >= snapshot.length) return snapshot;

  const [item] = snapshot.splice(sourceIndex, 1);
  let insertIndex = newIndex;

  if (targetQueueId) {
    const targetIndex = snapshot.findIndex(candidate => candidate.queueId === targetQueueId);
    if (targetIndex < 0) return queue;
    insertIndex = position === 'after' ? targetIndex + 1 : targetIndex;
  }

  if (!Number.isFinite(insertIndex)) return queue;
  insertIndex = Math.max(0, Math.min(insertIndex, snapshot.length));
  snapshot.splice(insertIndex, 0, item);
  return snapshot;
};
