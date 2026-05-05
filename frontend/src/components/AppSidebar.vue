<template>
  <nav
    class="app-sidebar"
    :class="{ expanded }"
    data-testid="app-sidebar"
  >
    <button
      class="sidebar-toggle"
      data-testid="sidebar-toggle"
      :aria-label="expanded ? 'Collapse sidebar' : 'Expand sidebar'"
      @click="expanded = !expanded"
    >
      <span class="toggle-icon">{{ expanded ? '\u25C0' : '\u25B6' }}</span>
    </button>
    <ul class="sidebar-nav">
      <li
        v-for="item in navItems"
        :key="item.to"
      >
        <router-link
          :to="item.to"
          class="sidebar-link"
          :data-testid="`nav-${item.name}`"
          :title="item.label"
        >
          <span class="sidebar-icon">{{ item.icon }}</span>
          <span
            v-if="expanded"
            class="sidebar-label"
          >{{ item.label }}</span>
        </router-link>
      </li>
    </ul>
  </nav>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const expanded = ref(false)

const navItems = [
  { name: 'projects', to: '/', icon: '\uD83D\uDCC1', label: 'Projects' },
  { name: 'jobs', to: '/jobs', icon: '\u2699', label: 'Jobs' },
  { name: 'studies', to: '/studies', icon: '\uD83D\uDCDD', label: 'Studies' },
  { name: 'queues', to: '/queues', icon: '\uD83D\uDCE8', label: 'Queues' },
  { name: 'accounts', to: '/accounts', icon: '\uD83D\uDC64', label: 'Accounts' },
  { name: 'settings', to: '/settings', icon: '\uD83D\uDD27', label: 'Settings' },
]
</script>

<style scoped>
.app-sidebar {
  width: 48px;
  min-width: 48px;
  height: 100%;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
  overflow: hidden;
}

.app-sidebar.expanded {
  width: 180px;
  min-width: 180px;
}

.sidebar-toggle {
  background: none;
  border: none;
  color: var(--text-color);
  cursor: pointer;
  padding: 8px;
  text-align: center;
  font-size: 12px;
}

.sidebar-nav {
  list-style: none;
  margin: 0;
  padding: 0;
}

.sidebar-link {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  color: var(--text-color);
  text-decoration: none;
  white-space: nowrap;
}

.sidebar-link:hover {
  background: var(--hover-bg);
}

.sidebar-link.router-link-active {
  background: var(--accent-bg);
  color: var(--accent-color);
}

.sidebar-icon {
  font-size: 18px;
  width: 24px;
  text-align: center;
  flex-shrink: 0;
}

.sidebar-label {
  font-size: 14px;
}
</style>
