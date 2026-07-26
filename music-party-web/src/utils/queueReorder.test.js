import { describe, expect, it } from 'vitest';
import { applyQueueReorder, buildQueueReorderPayload, buildQueueReorderPayloadFromDom, isQueueReorderSourceCurrent } from './queueReorder';

const queue = [
  { queueId: 'a' },
  { queueId: 'b' },
  { queueId: 'c' },
  { queueId: 'd' }
];

describe('buildQueueReorderPayload', () => {
  it('uses after when an item is dragged downward to a later index', () => {
    expect(buildQueueReorderPayload(queue, 0, 2)).toEqual({
      oldIndex: 0,
      newIndex: 2,
      queueId: 'a',
      targetQueueId: 'c',
      position: 'after'
    });
  });

  it('uses before when an item is dragged upward to an earlier index', () => {
    expect(buildQueueReorderPayload(queue, 3, 1)).toEqual({
      oldIndex: 3,
      newIndex: 1,
      queueId: 'd',
      targetQueueId: 'b',
      position: 'before'
    });
  });

  it('skips payloads that cannot be verified by queue id', () => {
    expect(buildQueueReorderPayload([{ queueId: 'a' }, {}], 0, 1)).toBeNull();
    expect(buildQueueReorderPayload(queue, 1, 1)).toBeNull();
  });
});

describe('buildQueueReorderPayloadFromDom', () => {
  const item = (id) => {
    const element = document.createElement('div');
    element.dataset.queueId = id;
    return element;
  };

  it('places the moved item after its previous DOM sibling when dropped in the middle or bottom', () => {
    const first = item('a');
    const moved = item('b');
    const third = item('c');
    const list = document.createElement('div');
    list.append(first, third, moved);

    expect(buildQueueReorderPayloadFromDom({ oldIndex: 1, newIndex: 2, item: moved })).toEqual({
      oldIndex: 1,
      newIndex: 2,
      queueId: 'b',
      targetQueueId: 'c',
      position: 'after'
    });
  });

  it('places the moved item before its next DOM sibling when dropped at the top', () => {
    const moved = item('c');
    const first = item('a');
    const second = item('b');
    const list = document.createElement('div');
    list.append(moved, first, second);

    expect(buildQueueReorderPayloadFromDom({ oldIndex: 2, newIndex: 0, item: moved })).toEqual({
      oldIndex: 2,
      newIndex: 0,
      queueId: 'c',
      targetQueueId: 'a',
      position: 'before'
    });
  });

  it('returns null when DOM neighbors are missing', () => {
    const moved = item('a');
    expect(buildQueueReorderPayloadFromDom({ oldIndex: 0, newIndex: 1, item: moved })).toBeNull();
  });
});

describe('isQueueReorderSourceCurrent', () => {
  it('only permits the index fallback when the dragged DOM item still matches the queue source', () => {
    const item = document.createElement('div');
    item.dataset.queueId = 'a';

    expect(isQueueReorderSourceCurrent(queue, { item, oldIndex: 0 })).toBe(true);
    expect(isQueueReorderSourceCurrent([{ queueId: 'b' }, ...queue.slice(1)], { item, oldIndex: 0 })).toBe(false);
  });
});

describe('applyQueueReorder', () => {
  it('optimistically mirrors a downward by-id reorder', () => {
    expect(applyQueueReorder(queue, {
      oldIndex: 0,
      newIndex: 2,
      queueId: 'a',
      targetQueueId: 'c',
      position: 'after'
    }).map(item => item.queueId)).toEqual(['b', 'c', 'a', 'd']);
  });

  it('optimistically mirrors an upward by-id reorder', () => {
    expect(applyQueueReorder(queue, {
      oldIndex: 3,
      newIndex: 1,
      queueId: 'd',
      targetQueueId: 'b',
      position: 'before'
    }).map(item => item.queueId)).toEqual(['a', 'd', 'b', 'c']);
  });
});
