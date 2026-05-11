<template>
  <div class="rotation-editor" data-testid="rotation-editor">
    <!-- 90-degree buttons row -->
    <div class="rotation-buttons" data-testid="rotation-buttons">
      <n-button
        size="small"
        data-testid="rotate-left-btn"
        title="Rotate 90° left"
        @click="rotateLeft"
      >
        ↺ 90°
      </n-button>
      <span class="rotation-value" data-testid="rotation-value">
        {{ displayDeg }}°
      </span>
      <n-button
        size="small"
        data-testid="rotate-right-btn"
        title="Rotate 90° right"
        @click="rotateRight"
      >
        ↻ 90°
      </n-button>
    </div>

    <!-- Free-angle slider (0–359) -->
    <div class="rotation-slider-row">
      <n-slider
        :value="props.rotationDeg"
        :min="0"
        :max="359"
        :step="1"
        data-testid="rotation-slider"
        @update:value="onSliderChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * RotationEditor: 90-degree rotation buttons and free-angle slider.
 *
 * Events:
 * - update:rotation-deg (newDeg: number) — emitted whenever rotation changes
 */
import { computed } from 'vue'
import { NButton, NSlider } from 'naive-ui'

const props = defineProps<{
  rotationDeg: number
}>()

// AC: emit rotation changes to parent for auto-save wiring
const emit = defineEmits<{
  (e: 'update:rotation-deg', deg: number): void
}>()

const displayDeg = computed(() => Math.round(props.rotationDeg))

function normalise(deg: number): number {
  return ((deg % 360) + 360) % 360
}

// AC: 90-degree left rotation button
function rotateLeft(): void {
  emit('update:rotation-deg', normalise(props.rotationDeg - 90))
}

// AC: 90-degree right rotation button
function rotateRight(): void {
  emit('update:rotation-deg', normalise(props.rotationDeg + 90))
}

// AC: free-angle slider (0-359)
function onSliderChange(val: number): void {
  emit('update:rotation-deg', val)
}
</script>

<style scoped>
.rotation-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rotation-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rotation-value {
  min-width: 44px;
  text-align: center;
  font-variant-numeric: tabular-nums;
  color: var(--n-text-color-2, #aaa);
  font-size: 13px;
}

.rotation-slider-row {
  padding: 0 2px;
}
</style>
