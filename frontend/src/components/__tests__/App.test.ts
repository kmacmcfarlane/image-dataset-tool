import { describe, it, expect, afterEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import App from '../../App.vue'

enableAutoUnmount(afterEach)

describe('App', () => {
  it('renders the application title', () => {
    const wrapper = mount(App, {
      global: {
        stubs: ['router-view'],
      },
    })
    expect(wrapper.text()).toContain('Image Dataset Tool')
  })
})
