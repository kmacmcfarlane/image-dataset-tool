import { describe, it, expect, afterEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import AppSidebar from '../AppSidebar.vue'

enableAutoUnmount(afterEach)

describe('AppSidebar', () => {
  const mountOptions = {
    global: {
      stubs: { 'router-link': { template: '<a><slot /></a>', props: ['to'] } },
    },
  }

  it('renders collapsed by default', () => {
    const wrapper = mount(AppSidebar, mountOptions)
    expect(wrapper.find('[data-testid="app-sidebar"]').classes()).not.toContain('expanded')
  })

  it('expands on toggle click', async () => {
    const wrapper = mount(AppSidebar, mountOptions)
    await wrapper.find('[data-testid="sidebar-toggle"]').trigger('click')
    expect(wrapper.find('[data-testid="app-sidebar"]').classes()).toContain('expanded')
  })

  it('shows labels when expanded', async () => {
    const wrapper = mount(AppSidebar, mountOptions)
    expect(wrapper.text()).not.toContain('Projects')
    await wrapper.find('[data-testid="sidebar-toggle"]').trigger('click')
    expect(wrapper.text()).toContain('Projects')
    expect(wrapper.text()).toContain('Jobs')
    expect(wrapper.text()).toContain('Settings')
  })

  it('renders all expected nav items', () => {
    const wrapper = mount(AppSidebar, mountOptions)
    const expectedItems = ['projects', 'jobs', 'studies', 'queues', 'accounts', 'settings']
    for (const name of expectedItems) {
      expect(wrapper.find(`[data-testid="nav-${name}"]`).exists()).toBe(true)
    }
  })
})
