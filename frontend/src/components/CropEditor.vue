<template>
  <!--
    CropEditor: drag-to-draw, resizable, movable crop overlay.

    Renders over an image container. The crop box is stored as percentages
    of the container dimensions. A semi-transparent mask darkens the area
    outside the crop rectangle.

    Events:
    - update:crop-box ({ x, y, w, h }: CropBox | null) — emitted whenever crop changes;
      all values are percentages [0–100]; null means crop is cleared.
  -->
  <div
    ref="containerEl"
    class="crop-editor"
    data-testid="crop-editor"
    @mousedown.self="onContainerMousedown"
  >
    <!-- Dim masks outside crop box -->
    <template v-if="activeCrop">
      <!-- Top -->
      <div
        class="crop-mask"
        :style="{
          top: '0',
          left: '0',
          right: '0',
          height: activeCrop.y + '%',
        }"
      />
      <!-- Bottom -->
      <div
        class="crop-mask"
        :style="{
          bottom: '0',
          left: '0',
          right: '0',
          top: (activeCrop.y + activeCrop.h) + '%',
        }"
      />
      <!-- Left -->
      <div
        class="crop-mask"
        :style="{
          top: activeCrop.y + '%',
          left: '0',
          width: activeCrop.x + '%',
          height: activeCrop.h + '%',
        }"
      />
      <!-- Right -->
      <div
        class="crop-mask"
        :style="{
          top: activeCrop.y + '%',
          right: '0',
          left: (activeCrop.x + activeCrop.w) + '%',
          height: activeCrop.h + '%',
        }"
      />

      <!-- Crop box -->
      <div
        class="crop-box"
        data-testid="crop-box"
        :style="{
          left: activeCrop.x + '%',
          top: activeCrop.y + '%',
          width: activeCrop.w + '%',
          height: activeCrop.h + '%',
        }"
        @mousedown.stop="onBoxMousedown"
      >
        <!-- Resize handles -->
        <div
          v-for="handle in HANDLES"
          :key="handle"
          class="crop-handle"
          :class="`handle-${handle}`"
          :data-testid="`crop-handle-${handle}`"
          @mousedown.stop="onHandleMousedown(handle, $event)"
        />
      </div>
    </template>

    <!-- New-crop draw area (shown when no active crop) -->
    <template v-if="!activeCrop && !drawing">
      <div class="crop-draw-hint" data-testid="crop-draw-hint">
        Drag to crop
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
/**
 * CropEditor: drag-to-draw, resize, move crop overlay.
 *
 * Events:
 * - update:crop-box (box: CropBox | null) — fires on every pointer interaction end.
 */
import { ref, computed, onUnmounted } from 'vue'
import type { CropBox } from '../api/samples'

const props = defineProps<{
  cropBox: CropBox | null
}>()

const emit = defineEmits<{
  (e: 'update:crop-box', box: CropBox | null): void
}>()

const containerEl = ref<HTMLElement | null>(null)

// Active crop is the local (live) value during drag; persists what's shown.
const localCrop = ref<CropBox | null>(props.cropBox ? { ...props.cropBox } : null)

// Synced external prop → local when prop changes externally (e.g., reset).
import { watch } from 'vue'
watch(
  () => props.cropBox,
  (next) => {
    localCrop.value = next ? { ...next } : null
  },
)

const activeCrop = computed(() => localCrop.value)

// --- Drawing new crop box ---
const drawing = ref(false)
let drawStart = { px: 0, py: 0 }

function containerPct(clientX: number, clientY: number): { px: number; py: number } {
  const el = containerEl.value
  if (!el) return { px: 0, py: 0 }
  const rect = el.getBoundingClientRect()
  return {
    px: Math.max(0, Math.min(100, ((clientX - rect.left) / rect.width) * 100)),
    py: Math.max(0, Math.min(100, ((clientY - rect.top) / rect.height) * 100)),
  }
}

function onContainerMousedown(event: MouseEvent): void {
  if (event.button !== 0) return
  drawing.value = true
  drawStart = containerPct(event.clientX, event.clientY)
  localCrop.value = null

  document.addEventListener('mousemove', onDrawMousemove)
  document.addEventListener('mouseup', onDrawMouseup)
}

function onDrawMousemove(event: MouseEvent): void {
  if (!drawing.value) return
  const { px, py } = containerPct(event.clientX, event.clientY)
  const x = Math.min(drawStart.px, px)
  const y = Math.min(drawStart.py, py)
  const w = Math.abs(px - drawStart.px)
  const h = Math.abs(py - drawStart.py)
  if (w > 0.5 && h > 0.5) {
    localCrop.value = { x, y, w, h }
  }
}

function onDrawMouseup(event: MouseEvent): void {
  drawing.value = false
  document.removeEventListener('mousemove', onDrawMousemove)
  document.removeEventListener('mouseup', onDrawMouseup)

  const { px, py } = containerPct(event.clientX, event.clientY)
  const x = Math.min(drawStart.px, px)
  const y = Math.min(drawStart.py, py)
  const w = Math.abs(px - drawStart.px)
  const h = Math.abs(py - drawStart.py)

  if (w < 2 || h < 2) {
    // Too small — clear crop.
    localCrop.value = null
    emit('update:crop-box', null)
    return
  }

  localCrop.value = { x, y, w, h }
  emit('update:crop-box', { x, y, w, h })
}

// --- Moving the crop box ---
let moveStart = { px: 0, py: 0, cx: 0, cy: 0 }

function onBoxMousedown(event: MouseEvent): void {
  if (event.button !== 0 || !localCrop.value) return
  const { px, py } = containerPct(event.clientX, event.clientY)
  moveStart = { px, py, cx: localCrop.value.x, cy: localCrop.value.y }

  document.addEventListener('mousemove', onMoveMousemove)
  document.addEventListener('mouseup', onMoveMouseup)
}

function onMoveMousemove(event: MouseEvent): void {
  if (!localCrop.value) return
  const { px, py } = containerPct(event.clientX, event.clientY)
  const dx = px - moveStart.px
  const dy = py - moveStart.py
  const w = localCrop.value.w
  const h = localCrop.value.h
  const newX = Math.max(0, Math.min(100 - w, moveStart.cx + dx))
  const newY = Math.max(0, Math.min(100 - h, moveStart.cy + dy))
  localCrop.value = { x: newX, y: newY, w, h }
}

function onMoveMouseup(): void {
  document.removeEventListener('mousemove', onMoveMousemove)
  document.removeEventListener('mouseup', onMoveMouseup)
  if (localCrop.value) {
    emit('update:crop-box', { ...localCrop.value })
  }
}

// --- Resizing crop box ---
type Handle = 'nw' | 'n' | 'ne' | 'e' | 'se' | 's' | 'sw' | 'w'
const HANDLES: Handle[] = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']

let resizeHandle: Handle = 'se'
let resizeStart = { px: 0, py: 0, box: { x: 0, y: 0, w: 0, h: 0 } }

function onHandleMousedown(handle: Handle, event: MouseEvent): void {
  if (event.button !== 0 || !localCrop.value) return
  resizeHandle = handle
  const { px, py } = containerPct(event.clientX, event.clientY)
  resizeStart = { px, py, box: { ...localCrop.value } }

  document.addEventListener('mousemove', onResizeMousemove)
  document.addEventListener('mouseup', onResizeMouseup)
}

function onResizeMousemove(event: MouseEvent): void {
  if (!localCrop.value) return
  const { px, py } = containerPct(event.clientX, event.clientY)
  const dx = px - resizeStart.px
  const dy = py - resizeStart.py
  const b = resizeStart.box

  let { x, y, w, h } = b

  // Adjust based on which handle is being dragged.
  if (resizeHandle.includes('e')) w = Math.max(2, b.w + dx)
  if (resizeHandle.includes('s')) h = Math.max(2, b.h + dy)
  if (resizeHandle.includes('w')) {
    const newX = Math.min(b.x + b.w - 2, b.x + dx)
    w = b.w - (newX - b.x)
    x = newX
  }
  if (resizeHandle.includes('n')) {
    const newY = Math.min(b.y + b.h - 2, b.y + dy)
    h = b.h - (newY - b.y)
    y = newY
  }

  // Clamp to [0, 100].
  x = Math.max(0, x)
  y = Math.max(0, y)
  w = Math.min(100 - x, w)
  h = Math.min(100 - y, h)

  localCrop.value = { x, y, w, h }
}

function onResizeMouseup(): void {
  document.removeEventListener('mousemove', onResizeMousemove)
  document.removeEventListener('mouseup', onResizeMouseup)
  if (localCrop.value) {
    emit('update:crop-box', { ...localCrop.value })
  }
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onDrawMousemove)
  document.removeEventListener('mouseup', onDrawMouseup)
  document.removeEventListener('mousemove', onMoveMousemove)
  document.removeEventListener('mouseup', onMoveMouseup)
  document.removeEventListener('mousemove', onResizeMousemove)
  document.removeEventListener('mouseup', onResizeMouseup)
})
</script>

<style scoped>
.crop-editor {
  position: absolute;
  inset: 0;
  cursor: crosshair;
  user-select: none;
}

.crop-mask {
  position: absolute;
  background: rgba(0, 0, 0, 0.55);
  pointer-events: none;
}

.crop-box {
  position: absolute;
  border: 2px solid var(--n-primary-color, #63e2b7);
  box-sizing: border-box;
  cursor: move;
}

.crop-handle {
  position: absolute;
  width: 10px;
  height: 10px;
  background: var(--n-primary-color, #63e2b7);
  border: 1px solid #fff;
  border-radius: 2px;
}

.handle-nw { top: -5px; left: -5px; cursor: nw-resize; }
.handle-n  { top: -5px; left: calc(50% - 5px); cursor: n-resize; }
.handle-ne { top: -5px; right: -5px; cursor: ne-resize; }
.handle-e  { top: calc(50% - 5px); right: -5px; cursor: e-resize; }
.handle-se { bottom: -5px; right: -5px; cursor: se-resize; }
.handle-s  { bottom: -5px; left: calc(50% - 5px); cursor: s-resize; }
.handle-sw { bottom: -5px; left: -5px; cursor: sw-resize; }
.handle-w  { top: calc(50% - 5px); left: -5px; cursor: w-resize; }

.crop-draw-hint {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.5);
  pointer-events: none;
}
</style>
