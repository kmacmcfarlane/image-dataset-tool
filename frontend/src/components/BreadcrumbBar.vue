<template>
  <div
    class="breadcrumb-bar"
    data-testid="breadcrumb-bar"
  >
    <span
      v-for="(crumb, index) in breadcrumbs"
      :key="crumb.path"
      class="breadcrumb-item"
    >
      <router-link
        v-if="index < breadcrumbs.length - 1"
        :to="crumb.path"
        class="breadcrumb-link"
      >
        {{ crumb.label }}
      </router-link>
      <span
        v-else
        class="breadcrumb-current"
      >{{ crumb.label }}</span>
      <span
        v-if="index < breadcrumbs.length - 1"
        class="breadcrumb-separator"
      >/</span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

interface BreadcrumbItem {
  label: string
  path: string
}

const route = useRoute()

const breadcrumbs = computed<BreadcrumbItem[]>(() => {
  const items: BreadcrumbItem[] = []
  const matched = route.matched

  for (const record of matched) {
    const label = (record.meta.breadcrumb as string) || record.name?.toString() || ''
    // Build the actual path by resolving params
    let path = record.path
    for (const [key, value] of Object.entries(route.params)) {
      path = path.replace(`:${key}`, String(value))
    }
    items.push({ label, path })
  }

  return items
})
</script>

<style scoped>
.breadcrumb-bar {
  padding: 8px 16px;
  font-size: 13px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-color);
  color: var(--text-secondary);
}

.breadcrumb-link {
  color: var(--accent-color);
  text-decoration: none;
}

.breadcrumb-link:hover {
  text-decoration: underline;
}

.breadcrumb-current {
  color: var(--text-color);
  font-weight: 500;
}

.breadcrumb-separator {
  margin: 0 6px;
  color: var(--text-secondary);
}
</style>
