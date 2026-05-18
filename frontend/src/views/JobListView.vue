<template>
  <div
    class="jobs-view"
    data-testid="view-job-list"
  >
    <div class="jobs-header">
      <h2 class="jobs-title">Job History</h2>
      <n-button
        size="small"
        data-testid="refresh-jobs-btn"
        @click="store.load()"
      >
        Refresh
      </n-button>
    </div>

    <!-- Job list table -->
    <n-data-table
      :columns="jobColumns"
      :data="displayJobs"
      :loading="store.loading"
      :bordered="true"
      size="small"
      :row-key="(row: JobResult) => row.id"
      data-testid="jobs-table"
    />

    <!-- Message panel -->
    <section
      v-if="store.selectedJobId"
      class="messages-section"
      data-testid="messages-panel"
    >
      <div class="messages-header">
        <h3 class="messages-title">
          Messages — <span class="job-id-label">{{ store.selectedJobId }}</span>
        </h3>
        <div class="messages-actions">
          <!-- Level filter toggle -->
          <n-button-group>
            <n-button
              v-for="lvl in (['error', 'warning', 'info'] as JobLevel[])"
              :key="lvl"
              size="small"
              :type="store.messageLevel === lvl ? 'primary' : 'default'"
              :data-testid="`level-filter-${lvl}`"
              @click="store.setMessageLevel(lvl)"
            >
              {{ lvl }}
            </n-button>
          </n-button-group>
          <n-button
            size="small"
            data-testid="close-messages-btn"
            @click="store.deselectJob()"
          >
            Close
          </n-button>
        </div>
      </div>

      <n-data-table
        :columns="messageColumns"
        :data="store.messages"
        :loading="store.messagesLoading"
        :bordered="true"
        size="small"
        :row-key="(row: JobMessage) => row.id"
        data-testid="messages-table"
      />

      <div
        v-if="store.messagesTotal > store.messages.length"
        class="messages-more"
        data-testid="messages-more"
      >
        Showing {{ store.messages.length }} of {{ store.messagesTotal }} messages
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, computed } from 'vue'
import { NButton, NButtonGroup, NDataTable, NProgress, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useJobsStore } from '../stores/jobs'
import { useSSE } from '../composables/useSSE'
import type { JobResult, JobMessage, JobLevel } from '../api/jobs'
import type { JobStateEvent, JobProgressEvent } from '../types/sse'

const store = useJobsStore()

// SSE for real-time updates.
const sse = useSSE()

function onJobProgress(data: unknown): void {
  const payload = data as JobProgressEvent
  store.applyProgressEvent({ id: payload.id, current: payload.current, total: payload.total, pct: payload.pct })
}

function onJobState(data: unknown): void {
  const payload = data as JobStateEvent
  store.applyStateEvent({ id: payload.id, type: payload.type, status: payload.status })
}

sse.on('job.progress', onJobProgress)
sse.on('job.state', onJobState)

onUnmounted(() => {
  sse.off('job.progress', onJobProgress)
  sse.off('job.state', onJobState)
})

onMounted(() => {
  store.load()
})

/**
 * Merge SSE live state into displayed job rows so running jobs reflect
 * real-time progress without requiring a full list reload.
 */
const displayJobs = computed<JobResult[]>(() =>
  store.jobs.map((j) => {
    const status = store.liveStatus[j.id] ?? j.status
    return { ...j, status }
  }),
)

function statusType(status: string): 'success' | 'error' | 'warning' | 'default' {
  switch (status) {
    case 'succeeded': return 'success'
    case 'failed':    return 'error'
    case 'running':   return 'warning'
    default:          return 'default'
  }
}

function levelType(level: string): 'error' | 'warning' | 'info' | 'default' {
  switch (level) {
    case 'error':   return 'error'
    case 'warning': return 'warning'
    case 'info':    return 'info'
    default:        return 'default'
  }
}

const jobColumns: DataTableColumns<JobResult> = [
  {
    title: 'Type',
    key: 'type',
    width: 180,
    render(row: JobResult) {
      return h('span', { class: 'job-type', 'data-testid': `job-type-${row.id}` }, row.type)
    },
  },
  {
    title: 'Status',
    key: 'status',
    width: 120,
    render(row: JobResult) {
      const liveStatus = store.liveStatus[row.id] ?? row.status
      return h(
        NTag,
        {
          type: statusType(liveStatus),
          size: 'small',
          'data-testid': `job-status-${row.id}`,
        },
        { default: () => liveStatus },
      )
    },
  },
  {
    title: 'Progress',
    key: 'progress',
    width: 200,
    render(row: JobResult) {
      const live = store.liveProgress[row.id]
      if (live) {
        return h(NProgress, {
          type: 'line',
          percentage: Math.round(live.pct),
          showIndicator: true,
          'data-testid': `job-progress-${row.id}`,
        })
      }
      const total = row.total_items ?? 0
      if (total > 0) {
        const pct = Math.round((row.completed_items / total) * 100)
        return h(NProgress, {
          type: 'line',
          percentage: pct,
          showIndicator: true,
          'data-testid': `job-progress-${row.id}`,
        })
      }
      return h('span', { class: 'text-secondary' }, `${row.completed_items} done`)
    },
  },
  {
    title: 'Errors',
    key: 'failed_items',
    width: 80,
    render(row: JobResult) {
      return h(
        'span',
        {
          class: row.failed_items > 0 ? 'error-count error-count--nonzero' : 'error-count',
          'data-testid': `job-errors-${row.id}`,
        },
        String(row.failed_items),
      )
    },
  },
  {
    title: 'Started',
    key: 'started_at',
    width: 180,
    render(row: JobResult) {
      return h('span', {}, formatDateTime(row.started_at))
    },
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 120,
    render(row: JobResult) {
      return h(
        NButton,
        {
          size: 'small',
          onClick: () => store.selectJob(row.id),
          'data-testid': `view-messages-btn-${row.id}`,
        },
        { default: () => 'Messages' },
      )
    },
  },
]

const messageColumns: DataTableColumns<JobMessage> = [
  {
    title: 'Time',
    key: 'created_at',
    width: 180,
    render(row: JobMessage) {
      return h('span', {}, formatDateTime(row.created_at))
    },
  },
  {
    title: 'Level',
    key: 'level',
    width: 90,
    render(row: JobMessage) {
      return h(
        NTag,
        { type: levelType(row.level), size: 'small', 'data-testid': `msg-level-${row.id}` },
        { default: () => row.level },
      )
    },
  },
  {
    title: 'Account',
    key: 'account_handle',
    width: 130,
    render(row: JobMessage) {
      return h('span', {}, row.account_handle ?? '—')
    },
  },
  {
    title: 'Sample',
    key: 'sample_ref',
    width: 130,
    render(row: JobMessage) {
      return h('span', {}, row.sample_ref ?? '—')
    },
  },
  {
    title: 'Message',
    key: 'message',
    ellipsis: { tooltip: true },
    render(row: JobMessage) {
      return h('span', { 'data-testid': `msg-text-${row.id}` }, row.message)
    },
  },
]

function formatDateTime(iso: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}
</script>

<style scoped>
.jobs-view {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.jobs-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.jobs-title {
  margin: 0;
  flex: 1;
  color: var(--text-color);
}

.messages-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-top: 1px solid var(--border-color);
  padding-top: 12px;
}

.messages-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.messages-title {
  margin: 0;
  flex: 1;
  font-size: 1rem;
  color: var(--text-color);
}

.job-id-label {
  font-family: monospace;
  font-size: 0.85em;
  color: var(--text-secondary);
}

.messages-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.messages-more {
  font-size: 0.85rem;
  color: var(--text-secondary);
  text-align: right;
}

.error-count {
  color: var(--text-secondary);
}

.error-count--nonzero {
  color: #d03050;
  font-weight: 600;
}
</style>
