<template>
  <n-config-provider :theme="null">
    <div
      id="app"
      class="app-root"
    >
      <ErrorBanner
        :message="errorMessage"
        @dismiss="errorMessage = ''"
      />
      <ToastContainer />
      <div class="app-body">
        <AppSidebar />
        <div class="app-main">
          <BreadcrumbBar />
          <div class="workspace-area">
            <div class="workspace-content">
              <router-view />
            </div>
            <RightPanel
              :visible="rightPanelVisible"
              @close="rightPanelVisible = false"
            />
          </div>
          <StatusBar :connection-status="sseStatus" />
        </div>
      </div>
    </div>
  </n-config-provider>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NConfigProvider } from 'naive-ui'
import AppSidebar from './components/AppSidebar.vue'
import BreadcrumbBar from './components/BreadcrumbBar.vue'
import StatusBar from './components/StatusBar.vue'
import ErrorBanner from './components/ErrorBanner.vue'
import ToastContainer from './components/ToastContainer.vue'
import RightPanel from './components/RightPanel.vue'
import { onApiError } from './api'
import type { SSEConnectionStatus } from './types/sse'

const errorMessage = ref('')
const rightPanelVisible = ref(false)
const sseStatus = ref<SSEConnectionStatus>('disconnected')

let unsubError: (() => void) | null = null

onMounted(() => {
  unsubError = onApiError((error) => {
    errorMessage.value = error.message
  })
})

onUnmounted(() => {
  unsubError?.()
})
</script>

<style>
/* Canonical CSS variables per DEVELOPMENT_PRACTICES 4.8 */
:root {
  --text-color: #1a1a2e;
  --text-secondary: #6b7280;
  --bg-color: #ffffff;
  --bg-secondary: #f9fafb;
  --sidebar-bg: #f3f4f6;
  --statusbar-bg: #f3f4f6;
  --accent-color: #3b82f6;
  --accent-bg: rgba(59, 130, 246, 0.1);
  --border-color: #e5e7eb;
  --hover-bg: rgba(0, 0, 0, 0.04);
  --error-bg: #fef2f2;
  --error-text: #991b1b;
  --error-border: #fecaca;
  --status-connected: #22c55e;
  --status-connecting: #f59e0b;
  --status-disconnected: #ef4444;
  --toast-info-bg: #eff6ff;
  --toast-info-text: #1e40af;
  --toast-warning-bg: #fffbeb;
  --toast-warning-text: #92400e;
  --toast-error-bg: #fef2f2;
  --toast-error-text: #991b1b;
}

@media (prefers-color-scheme: dark) {
  :root {
    --text-color: #e5e7eb;
    --text-secondary: #9ca3af;
    --bg-color: #1f2937;
    --bg-secondary: #111827;
    --sidebar-bg: #111827;
    --statusbar-bg: #111827;
    --accent-color: #60a5fa;
    --accent-bg: rgba(96, 165, 250, 0.15);
    --border-color: #374151;
    --hover-bg: rgba(255, 255, 255, 0.06);
    --error-bg: #450a0a;
    --error-text: #fca5a5;
    --error-border: #7f1d1d;
    --status-connected: #4ade80;
    --status-connecting: #fbbf24;
    --status-disconnected: #f87171;
    --toast-info-bg: #1e3a5f;
    --toast-info-text: #93c5fd;
    --toast-warning-bg: #451a03;
    --toast-warning-text: #fcd34d;
    --toast-error-bg: #450a0a;
    --toast-error-text: #fca5a5;
  }
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body {
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
    Ubuntu, Cantarell, sans-serif;
  background: var(--bg-color);
  color: var(--text-color);
}

#app {
  height: 100%;
}
</style>

<style scoped>
.app-root {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.app-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.app-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.workspace-area {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.workspace-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  min-width: 0;
}
</style>
