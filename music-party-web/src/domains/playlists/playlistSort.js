import Sortable from 'sortablejs'
import { restoreDraggedNode } from '../../utils/queueReorder'

/** @param {HTMLElement} list
 * @param {(id: string, position: number) => void} onReorder
 */
export function bindPlaylistSort(list, onReorder) {
  let anchor = null
  const sortable = new Sortable(list, {
    draggable: '[data-reorder-id]',
    filter: 'input, textarea, select, [data-no-drag]',
    preventOnFilter: false,
    animation: 160,
    ghostClass: 'mp-drag-ghost',
    chosenClass: 'mp-drag-chosen',
    dragClass: 'mp-drag-active',
    onChoose(event) { anchor = event.item.nextSibling },
    onEnd(event) {
      const { item, oldDraggableIndex, newDraggableIndex } = event
      const restored = restoreDraggedNode(event, anchor)
      anchor = null
      const id = item.dataset.reorderId
      if (restored && id && newDraggableIndex != null && newDraggableIndex !== oldDraggableIndex) {
        onReorder(id, newDraggableIndex)
      }
    },
  })
  return () => sortable.destroy()
}
