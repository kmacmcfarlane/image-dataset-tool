import { describe, it, expect, afterEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import StatusBar from '../StatusBar.vue'

enableAutoUnmount(afterEach)

describe('StatusBar', () => {
  it.each([
    ['connected', 'Connected'],
    ['connecting', 'Connecting...'],
    ['disconnected', 'Disconnected'],
  ] as const)('shows "%s" status as "%s"', (status, label) => {
    const wrapper = mount(StatusBar, { props: { connectionStatus: status } })
    expect(wrapper.find('[data-testid="connection-status"]').text()).toContain(label)
  })

  it.each([
    ['connected'],
    ['connecting'],
    ['disconnected'],
  ] as const)('applies %s CSS class', (status) => {
    const wrapper = mount(StatusBar, { props: { connectionStatus: status } })
    expect(wrapper.find('[data-testid="connection-status"]').classes()).toContain(status)
  })
})
