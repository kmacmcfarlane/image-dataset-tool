<template>
  <div
    v-if="visible"
    class="error-banner"
    role="alert"
    data-testid="error-banner"
  >
    <span class="error-message">{{ message }}</span>
    <button
      class="error-dismiss"
      data-testid="error-banner-dismiss"
      aria-label="Dismiss error"
      @click="dismiss"
    >
      &times;
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  /** Error message to display */
  message: string
}>()

/**
 * Emits:
 * - dismiss: fired when the user clicks the dismiss button. No payload.
 */
const emit = defineEmits<{
  dismiss: []
}>()

const visible = ref(!!props.message)

watch(() => props.message, (val) => {
  visible.value = !!val
})

function dismiss(): void {
  visible.value = false
  emit('dismiss')
}
</script>

<style scoped>
.error-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--error-bg);
  color: var(--error-text);
  font-size: 14px;
  border-bottom: 1px solid var(--error-border);
}

.error-dismiss {
  background: none;
  border: none;
  color: var(--error-text);
  cursor: pointer;
  font-size: 18px;
  padding: 0 4px;
  line-height: 1;
}
</style>
