<template>
  <div class="sample-grid-view" data-testid="view-sample-grid">
    <div class="sample-grid-header">
      <h2>Sample Review Grid</h2>
      <div class="sample-grid-controls" data-testid="sample-grid-controls">
        <n-space align="center" :size="12">
          <n-text depth="3">{{ filteredCount }} / {{ totalCount }} samples</n-text>
          <n-divider vertical />
          <n-checkbox
            :checked="store.showPending"
            data-testid="filter-pending"
            @update:checked="(v: boolean) => store.setStatusFilter('pending', v)"
          >
            Pending
          </n-checkbox>
          <n-checkbox
            :checked="store.showKept"
            data-testid="filter-kept"
            @update:checked="(v: boolean) => store.setStatusFilter('kept', v)"
          >
            Kept
          </n-checkbox>
          <n-checkbox
            :checked="store.showRejected"
            data-testid="filter-rejected"
            @update:checked="(v: boolean) => store.setStatusFilter('rejected', v)"
          >
            Rejected
          </n-checkbox>
          <n-divider vertical />
          <n-checkbox
            :checked="store.showDuplicates"
            data-testid="filter-duplicates"
            @update:checked="(v: boolean) => store.setShowDuplicates(v)"
          >
            Show duplicates
          </n-checkbox>
        </n-space>
      </div>
    </div>

    <n-spin :show="store.loading" data-testid="sample-grid-loading">
      <div
        v-if="store.error"
        class="sample-grid-error"
        data-testid="sample-grid-error"
      >
        <n-text type="error">{{ store.error }}</n-text>
      </div>

      <div
        v-else-if="!store.loading && filteredCount === 0"
        class="sample-grid-empty"
        data-testid="sample-grid-empty"
      >
        <n-empty description="No samples match current filters" />
      </div>

      <div
        v-else
        ref="scrollContainer"
        class="sample-grid-scroll"
        data-testid="sample-grid-scroll"
        @scroll="onScroll"
      >
        <div :style="{ height: totalHeight + 'px', position: 'relative' }">
          <div
            class="sample-grid"
            :style="{ transform: `translateY(${offsetY}px)` }"
          >
            <div
              v-for="sample in visibleSamples"
              :key="sample.id"
              class="sample-card"
              :data-testid="`sample-card-${sample.id}`"
              :data-sample-id="sample.id"
            >
              <SampleThumbnail
                :sample-id="sample.id"
                :thumbnail-path="sample.thumbnail_path"
                :status="sample.status"
                :is-duplicate="sample.is_duplicate"
                :duplicate-of="sample.duplicate_of"
              />
            </div>
          </div>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NCheckbox, NDivider, NEmpty, NSpace, NSpin, NText } from 'naive-ui'
import { useSamplesStore } from '../stores/samples'
import SampleThumbnail from '../components/SampleThumbnail.vue'

const route = useRoute()
const store = useSamplesStore()

const scrollContainer = ref<HTMLElement | null>(null)

// Virtual scroll configuration.
const CARD_SIZE = 180 // px (card width/height including gap)
const BUFFER_ROWS = 3 // extra rows rendered above/below viewport

const scrollTop = ref(0)
const containerHeight = ref(800)

const filteredCount = computed(() => store.filteredSamples.length)
const totalCount = computed(() => store.totalCount)

const columnsPerRow = computed(() => {
  if (!scrollContainer.value) return 4
  const width = scrollContainer.value.clientWidth
  return Math.max(1, Math.floor(width / CARD_SIZE))
})

const totalRows = computed(() =>
  Math.ceil(filteredCount.value / columnsPerRow.value),
)

const totalHeight = computed(() => totalRows.value * CARD_SIZE)

const visibleStartRow = computed(() =>
  Math.max(0, Math.floor(scrollTop.value / CARD_SIZE) - BUFFER_ROWS),
)

const visibleEndRow = computed(() => {
  const rowsInView = Math.ceil(containerHeight.value / CARD_SIZE)
  return Math.min(totalRows.value, Math.floor(scrollTop.value / CARD_SIZE) + rowsInView + BUFFER_ROWS)
})

const offsetY = computed(() => visibleStartRow.value * CARD_SIZE)

const visibleSamples = computed(() => {
  const startIdx = visibleStartRow.value * columnsPerRow.value
  const endIdx = visibleEndRow.value * columnsPerRow.value
  return store.filteredSamples.slice(startIdx, endIdx)
})

function onScroll(): void {
  if (scrollContainer.value) {
    scrollTop.value = scrollContainer.value.scrollTop
  }
}

function updateContainerSize(): void {
  if (scrollContainer.value) {
    containerHeight.value = scrollContainer.value.clientHeight
  }
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  const sid = route.params.sid as string
  if (sid) {
    store.connectAndFetch(sid)
  }
})

onUnmounted(() => {
  store.disconnect()
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})

// Attach ResizeObserver when scroll container becomes available (conditionally rendered).
watch(scrollContainer, (el) => {
  if (el && !resizeObserver) {
    resizeObserver = new ResizeObserver(() => updateContainerSize())
    resizeObserver.observe(el)
    updateContainerSize()
  }
})

// Re-connect if subject changes (e.g., route param change).
watch(
  () => route.params.sid,
  (newSid) => {
    if (newSid && typeof newSid === 'string') {
      store.disconnect()
      store.connectAndFetch(newSid)
    }
  },
)
</script>

<style scoped>
.sample-grid-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
}

.sample-grid-header {
  margin-bottom: 16px;
}

.sample-grid-header h2 {
  margin: 0 0 8px 0;
}

.sample-grid-controls {
  margin-bottom: 8px;
}

.sample-grid-scroll {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.sample-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.sample-card {
  aspect-ratio: 1;
  border-radius: 6px;
  overflow: hidden;
  position: relative;
  background: var(--n-color-embedded);
  border: 1px solid var(--n-border-color);
}

.sample-grid-empty,
.sample-grid-error {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
</style>
