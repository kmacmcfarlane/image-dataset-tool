import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type ToastLevel = 'info' | 'warning' | 'error'

export interface ToastMessage {
  id: number
  level: ToastLevel
  message: string
  timestamp: number
}

const TOAST_THRESHOLD_KEY = 'toast-level-threshold'
const LEVEL_ORDER: Record<ToastLevel, number> = {
  info: 0,
  warning: 1,
  error: 2,
}

function loadThreshold(): ToastLevel {
  const stored = localStorage.getItem(TOAST_THRESHOLD_KEY)
  if (stored && (stored === 'info' || stored === 'warning' || stored === 'error')) {
    return stored
  }
  return 'warning'
}

/**
 * Toast notification store.
 * Threshold is persisted in localStorage (default: warning).
 * Only toasts at or above the threshold level are shown.
 */
export const useToastStore = defineStore('toast', () => {
  const threshold = ref<ToastLevel>(loadThreshold())
  const toasts = ref<ToastMessage[]>([])
  let nextId = 1

  const visibleToasts = computed(() =>
    toasts.value.filter(t => LEVEL_ORDER[t.level] >= LEVEL_ORDER[threshold.value])
  )

  function setThreshold(level: ToastLevel): void {
    threshold.value = level
    localStorage.setItem(TOAST_THRESHOLD_KEY, level)
  }

  function addToast(level: ToastLevel, message: string): void {
    const toast: ToastMessage = {
      id: nextId++,
      level,
      message,
      timestamp: Date.now(),
    }
    toasts.value.push(toast)

    // Auto-remove after 5s
    setTimeout(() => {
      dismissToast(toast.id)
    }, 5000)
  }

  function dismissToast(id: number): void {
    const idx = toasts.value.findIndex(t => t.id === id)
    if (idx >= 0) {
      toasts.value.splice(idx, 1)
    }
  }

  function clearAll(): void {
    toasts.value = []
  }

  return {
    threshold,
    toasts,
    visibleToasts,
    setThreshold,
    addToast,
    dismissToast,
    clearAll,
  }
})
