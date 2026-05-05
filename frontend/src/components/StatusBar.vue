<template>
  <div
    class="status-bar"
    data-testid="status-bar"
  >
    <div class="status-left">
      <span
        class="connection-indicator"
        :class="connectionStatus"
        data-testid="connection-status"
      >
        <span class="status-dot" />
        {{ connectionLabel }}
      </span>
    </div>
    <div class="status-right">
      <span class="status-info">Image Dataset Tool</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SSEConnectionStatus } from '../types/sse'

const props = defineProps<{
  /** Current SSE connection status */
  connectionStatus: SSEConnectionStatus
}>()

const connectionLabel = computed(() => {
  switch (props.connectionStatus) {
    case 'connected':
      return 'Connected'
    case 'connecting':
      return 'Connecting...'
    case 'disconnected':
      return 'Disconnected'
    default:
      return 'Unknown'
  }
})
</script>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 12px;
  font-size: 12px;
  background: var(--statusbar-bg);
  border-top: 1px solid var(--border-color);
  color: var(--text-secondary);
  min-height: 24px;
}

.connection-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.connection-indicator.connected .status-dot {
  background: var(--status-connected);
}

.connection-indicator.connecting .status-dot {
  background: var(--status-connecting);
}

.connection-indicator.disconnected .status-dot {
  background: var(--status-disconnected);
}

.status-info {
  color: var(--text-secondary);
}
</style>
