import { createRouter, createWebHistory } from 'vue-router'
import { SETUP_COMPLETE_KEY } from '../constants'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup',
      name: 'setup',
      component: () => import('../views/SetupWizardView.vue'),
      meta: { breadcrumb: 'Setup', skipSetupGuard: true },
    },
    {
      path: '/',
      name: 'projects',
      component: () => import('../views/ProjectListView.vue'),
      meta: { breadcrumb: 'Projects' },
    },
    {
      path: '/projects/:id',
      name: 'project-detail',
      component: () => import('../views/ProjectDetailView.vue'),
      meta: { breadcrumb: 'Project' },
    },
    {
      path: '/projects/:id/subjects/:sid/samples',
      name: 'sample-grid',
      component: () => import('../views/SampleGridView.vue'),
      meta: { breadcrumb: 'Samples' },
    },
    {
      path: '/projects/:id/subjects/:sid/samples/:sampleId',
      name: 'sample-detail',
      component: () => import('../views/SampleDetailView.vue'),
      meta: { breadcrumb: 'Sample' },
    },
    {
      path: '/jobs',
      name: 'jobs',
      component: () => import('../views/JobListView.vue'),
      meta: { breadcrumb: 'Jobs' },
    },
    {
      path: '/studies',
      name: 'studies',
      component: () => import('../views/StudiesView.vue'),
      meta: { breadcrumb: 'Studies' },
    },
    {
      path: '/export',
      name: 'export',
      component: () => import('../views/ExportView.vue'),
      meta: { breadcrumb: 'Export' },
    },
    {
      path: '/queues',
      name: 'queues',
      component: () => import('../views/QueuesView.vue'),
      meta: { breadcrumb: 'Queues' },
    },
    {
      path: '/accounts',
      name: 'accounts',
      component: () => import('../views/AccountsView.vue'),
      meta: { breadcrumb: 'Accounts' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { breadcrumb: 'Settings' },
    },
  ],
})

// Navigation guard: redirect to /setup on first launch (until wizard is completed).
router.beforeEach((to) => {
  if (to.meta.skipSetupGuard) return true
  const done = localStorage.getItem(SETUP_COMPLETE_KEY)
  if (!done) return { name: 'setup' }
  return true
})

export default router
