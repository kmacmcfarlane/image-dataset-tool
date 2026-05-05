<template>
  <div
    class="toast-container"
    data-testid="toast-container"
  >
    <div
      v-for="toast in visibleToasts"
      :key="toast.id"
      class="toast-item"
      :class="`toast-${toast.level}`"
      :data-testid="`toast-${toast.level}`"
      role="status"
    >
      <span class="toast-message">{{ toast.message }}</span>
      <button
        class="toast-dismiss"
        aria-label="Dismiss notification"
        @click="store.dismissToast(toast.id)"
      >
        &times;
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useToastStore } from '../stores/toast'

const store = useToastStore()
const { visibleToasts } = storeToRefs(store)
</script>

<style scoped>
.toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 400px;
}

.toast-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 14px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.toast-info {
  background: var(--toast-info-bg);
  color: var(--toast-info-text);
}

.toast-warning {
  background: var(--toast-warning-bg);
  color: var(--toast-warning-text);
}

.toast-error {
  background: var(--toast-error-bg);
  color: var(--toast-error-text);
}

.toast-dismiss {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 16px;
  padding: 0 4px;
  margin-left: 12px;
  line-height: 1;
}
</style>
