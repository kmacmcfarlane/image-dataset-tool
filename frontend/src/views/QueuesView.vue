<template>
  <div
    class="queues-view"
    data-testid="view-queues"
  >
    <h2 class="queues-title">Queue Administration</h2>

    <!-- Consumer Stats Table -->
    <section class="stats-section">
      <div class="section-header">
        <h3>Consumer Statistics</h3>
        <n-button
          size="small"
          data-testid="refresh-stats-btn"
          @click="store.loadStats()"
        >
          Refresh
        </n-button>
      </div>
      <n-data-table
        :columns="statsColumns"
        :data="store.consumers"
        :loading="store.statsLoading"
        :bordered="true"
        size="small"
        data-testid="stats-table"
      />
    </section>

    <!-- Message Peek Section -->
    <section
      v-if="store.selectedSubject"
      class="messages-section"
    >
      <div class="section-header">
        <h3>Messages: {{ store.selectedSubject }}</h3>
        <div class="section-actions">
          <n-button
            size="small"
            type="error"
            data-testid="purge-btn"
            @click="showPurgeConfirm = true"
          >
            Purge All
          </n-button>
          <n-button
            size="small"
            data-testid="close-messages-btn"
            @click="store.selectedSubject = null"
          >
            Close
          </n-button>
        </div>
      </div>

      <n-data-table
        :columns="messageColumns"
        :data="store.messages"
        :loading="store.loading"
        :bordered="true"
        size="small"
        :row-key="(row: QueueMessage) => row.sequence"
        data-testid="messages-table"
      />

      <div
        v-if="store.totalMessages > store.pageSize"
        class="pagination"
      >
        <n-button
          size="small"
          :disabled="store.currentOffset === 0"
          data-testid="prev-page-btn"
          @click="store.loadMessages(store.selectedSubject!, store.currentOffset - store.pageSize)"
        >
          Previous
        </n-button>
        <span class="page-info">
          {{ store.currentOffset + 1 }}-{{ Math.min(store.currentOffset + store.pageSize, store.totalMessages) }}
          of {{ store.totalMessages }}
        </span>
        <n-button
          size="small"
          :disabled="store.currentOffset + store.pageSize >= store.totalMessages"
          data-testid="next-page-btn"
          @click="store.loadMessages(store.selectedSubject!, store.currentOffset + store.pageSize)"
        >
          Next
        </n-button>
      </div>
    </section>

    <!-- Purge Confirmation Modal -->
    <n-modal
      v-model:show="showPurgeConfirm"
      preset="dialog"
      type="warning"
      title="Confirm Purge"
      :content="`Are you sure you want to purge all messages from '${store.selectedSubject}'? This cannot be undone.`"
      positive-text="Purge"
      negative-text="Cancel"
      data-testid="purge-confirm-modal"
      @positive-click="handlePurge"
      @negative-click="showPurgeConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { h, ref, onMounted } from 'vue'
import { NButton, NDataTable, NModal } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useQueuesStore } from '../stores/queues'
import type { ConsumerStats, QueueMessage } from '../api/queues'

const store = useQueuesStore()
const showPurgeConfirm = ref(false)

onMounted(() => {
  store.loadStats()
})

const statsColumns: DataTableColumns<ConsumerStats> = [
  { title: 'Consumer', key: 'name' },
  { title: 'Subject', key: 'subject' },
  { title: 'Pending', key: 'pending' },
  { title: 'Ack Pending', key: 'ack_pending' },
  { title: 'Redelivered', key: 'redelivered' },
  { title: 'Waiting', key: 'waiting' },
  {
    title: 'Actions',
    key: 'actions',
    render(row: ConsumerStats) {
      return h(
        NButton,
        {
          size: 'small',
          onClick: () => store.loadMessages(row.subject),
          'data-testid': `peek-btn-${row.name}`,
        },
        { default: () => 'Peek' },
      )
    },
  },
]

const messageColumns: DataTableColumns<QueueMessage> = [
  { title: 'Seq', key: 'sequence', width: 80 },
  { title: 'Subject', key: 'subject' },
  {
    title: 'Data',
    key: 'data',
    ellipsis: { tooltip: true },
    width: 300,
  },
  { title: 'Timestamp', key: 'timestamp', width: 200 },
  {
    title: 'Actions',
    key: 'actions',
    width: 180,
    render(row: QueueMessage) {
      return h('div', { class: 'msg-actions' }, [
        h(
          NButton,
          {
            size: 'tiny',
            type: 'primary',
            onClick: () => handleRetry(row),
            'data-testid': `retry-btn-${row.sequence}`,
          },
          { default: () => 'Retry' },
        ),
        h(
          NButton,
          {
            size: 'tiny',
            type: 'error',
            onClick: () => handleDelete(row),
            'data-testid': `delete-btn-${row.sequence}`,
          },
          { default: () => 'Delete' },
        ),
      ])
    },
  },
]

async function handleRetry(msg: QueueMessage): Promise<void> {
  if (store.selectedSubject) {
    await store.retry(store.selectedSubject, msg.sequence)
  }
}

async function handleDelete(msg: QueueMessage): Promise<void> {
  if (store.selectedSubject) {
    await store.remove(store.selectedSubject, msg.sequence)
  }
}

async function handlePurge(): Promise<void> {
  if (store.selectedSubject) {
    await store.purge(store.selectedSubject)
    showPurgeConfirm.value = false
  }
}
</script>

<style scoped>
.queues-view {
  max-width: 1200px;
}

.queues-title {
  margin-bottom: 16px;
  color: var(--text-color);
}

.stats-section,
.messages-section {
  margin-bottom: 24px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-header h3 {
  color: var(--text-color);
  font-size: 14px;
  font-weight: 600;
}

.section-actions {
  display: flex;
  gap: 8px;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 12px;
}

.page-info {
  font-size: 13px;
  color: var(--text-secondary);
}

.msg-actions {
  display: flex;
  gap: 4px;
}
</style>
