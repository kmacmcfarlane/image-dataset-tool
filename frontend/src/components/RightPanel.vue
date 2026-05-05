<template>
  <aside
    v-if="visible"
    class="right-panel"
    :style="{ width: `${panelWidth}px` }"
    data-testid="right-panel"
  >
    <div
      class="resize-handle"
      data-testid="right-panel-resize"
      @mousedown="startResize"
    />
    <div class="right-panel-header">
      <slot name="header">
        <span>Details</span>
      </slot>
      <button
        class="panel-close"
        data-testid="right-panel-close"
        aria-label="Close panel"
        @click="emit('close')"
      >
        &times;
      </button>
    </div>
    <div class="right-panel-content">
      <slot />
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  /** Whether the panel is visible */
  visible: boolean
}>()

/**
 * Emits:
 * - close: fired when the user clicks the close button. No payload.
 */
const emit = defineEmits<{
  close: []
}>()

const panelWidth = ref(320)
const MIN_WIDTH = 200
const MAX_WIDTH = 600

function startResize(event: MouseEvent): void {
  event.preventDefault()
  const startX = event.clientX
  const startWidth = panelWidth.value

  function onMouseMove(e: MouseEvent): void {
    // Dragging left increases width (panel is on right side)
    const delta = startX - e.clientX
    panelWidth.value = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, startWidth + delta))
  }

  function onMouseUp(): void {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}
</script>

<style scoped>
.right-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-color);
  border-left: 1px solid var(--border-color);
  position: relative;
  flex-shrink: 0;
}

.resize-handle {
  position: absolute;
  left: -3px;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: var(--accent-color);
  opacity: 0.3;
}

.right-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
}

.panel-close {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 18px;
  padding: 0 4px;
  line-height: 1;
}

.right-panel-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}
</style>
