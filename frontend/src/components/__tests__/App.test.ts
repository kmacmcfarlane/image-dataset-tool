import { describe, it, expect, afterEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import App from '../../App.vue'

enableAutoUnmount(afterEach)

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div>Home</div>' } }],
  })
}

describe('App', () => {
  it('renders the layout structure', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [createPinia(), createTestRouter()],
        stubs: {
          'n-config-provider': { template: '<div><slot /></div>' },
        },
      },
    })
    expect(wrapper.find('[data-testid="app-sidebar"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="breadcrumb-bar"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="status-bar"]').exists()).toBe(true)
  })

  it('shows disconnected status by default', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [createPinia(), createTestRouter()],
        stubs: {
          'n-config-provider': { template: '<div><slot /></div>' },
        },
      },
    })
    const statusBar = wrapper.find('[data-testid="connection-status"]')
    expect(statusBar.text()).toContain('Disconnected')
  })
})
