<template>
  <div
    ref="rootEl"
    class="sample-thumbnail"
    data-testid="sample-thumbnail"
    :data-sample-id="sampleId"
    :class="{
      'sample-selected': selected,
      'sample-focused': focused,
      'sample-status-kept': status === 'kept',
      'sample-status-rejected': status === 'rejected',
      'sample-status-pending': status === 'pending',
    }"
  >
    <img
      v-if="isVisible && !imgError"
      :src="thumbnailPath"
      :alt="`Sample ${sampleId}`"
      class="thumb-img"
      loading="lazy"
      @error="onImgError"
    />
    <div v-else-if="imgError" class="thumb-placeholder thumb-error">
      <span>!</span>
    </div>
    <div v-else class="thumb-placeholder">
      <span />
    </div>

    <!-- Selection checkbox overlay -->
    <div
      v-if="selected"
      class="selection-check"
      data-testid="selection-check"
    >
      &#10003;
    </div>

    <!-- Status badge -->
    <span
      class="status-badge"
      :class="`status-${status}`"
      :data-testid="`status-badge-${sampleId}`"
    >
      {{ status }}
    </span>

    <!-- Duplicate badge -->
    <span
      v-if="isDuplicate"
      class="duplicate-badge"
      :data-testid="`duplicate-badge-${sampleId}`"
      :title="duplicateOf || 'Duplicate'"
    >
      DUP
    </span>
  </div>
</template>

<script setup lang="ts">
/**
 * SampleThumbnail: lazy-loaded thumbnail using IntersectionObserver.
 *
 * Events: none (purely presentational).
 * Props:
 * - sampleId: UUID of the sample
 * - thumbnailPath: URL to load thumbnail from
 * - status: current sample status (pending/kept/rejected)
 * - isDuplicate: whether the sample is marked duplicate
 * - duplicateOf: optional JSON array string of matched sample IDs
 * - selected: whether this sample is currently selected
 * - focused: whether this sample has keyboard focus
 */
import { ref, onMounted, onUnmounted } from 'vue'

defineProps<{
  sampleId: string
  thumbnailPath: string
  status: string
  isDuplicate: boolean
  duplicateOf?: string
  selected?: boolean
  focused?: boolean
}>()

const rootEl = ref<HTMLElement | null>(null)
const isVisible = ref(false)
const imgError = ref(false)

let observer: IntersectionObserver | null = null

function onImgError(): void {
  imgError.value = true
}

onMounted(() => {
  if (!rootEl.value) return

  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          isVisible.value = true
          // Once visible, no need to keep observing.
          observer?.disconnect()
          observer = null
          break
        }
      }
    },
    { rootMargin: '200px' }, // Pre-load 200px before visible
  )
  observer.observe(rootEl.value)
})

onUnmounted(() => {
  observer?.disconnect()
  observer = null
})
</script>

<style scoped>
.sample-thumbnail {
  width: 100%;
  height: 100%;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--n-color-embedded, #1a1a1a);
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}

/* AC: Visual indicators: green (kept), red (rejected), neutral (pending) */
.sample-status-kept {
  box-shadow: inset 0 0 0 3px rgba(50, 200, 100, 0.7);
}

.sample-status-rejected {
  box-shadow: inset 0 0 0 3px rgba(220, 60, 60, 0.7);
}

.sample-selected {
  box-shadow: inset 0 0 0 3px var(--n-primary-color, #63e2b7);
}

.sample-status-kept.sample-selected {
  box-shadow: inset 0 0 0 3px rgba(50, 200, 100, 0.9);
}

.sample-status-rejected.sample-selected {
  box-shadow: inset 0 0 0 3px rgba(220, 60, 60, 0.9);
}

.sample-focused {
  outline: 2px solid var(--n-primary-color, #63e2b7);
  outline-offset: -2px;
}

.thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.thumb-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--n-color-embedded, #2a2a2a);
}

.thumb-error {
  color: var(--n-text-color-error, #e88080);
  font-size: 24px;
}

.selection-check {
  position: absolute;
  top: 4px;
  left: 4px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--n-primary-color, #63e2b7);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.status-badge {
  position: absolute;
  bottom: 4px;
  left: 4px;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  pointer-events: none;
}

.status-pending {
  background: rgba(250, 200, 50, 0.85);
  color: #333;
}

.status-kept {
  background: rgba(50, 200, 100, 0.85);
  color: #fff;
}

.status-rejected {
  background: rgba(220, 60, 60, 0.85);
  color: #fff;
}

.duplicate-badge {
  position: absolute;
  top: 4px;
  right: 4px;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  background: rgba(160, 100, 220, 0.85);
  color: #fff;
  pointer-events: none;
}
</style>
