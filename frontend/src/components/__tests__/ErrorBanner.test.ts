import { describe, it, expect, afterEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import ErrorBanner from '../ErrorBanner.vue'

enableAutoUnmount(afterEach)

describe('ErrorBanner', () => {
  it('shows when message is non-empty', () => {
    const wrapper = mount(ErrorBanner, { props: { message: 'Something went wrong' } })
    expect(wrapper.find('[data-testid="error-banner"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Something went wrong')
  })

  it('is hidden when message is empty', () => {
    const wrapper = mount(ErrorBanner, { props: { message: '' } })
    expect(wrapper.find('[data-testid="error-banner"]').exists()).toBe(false)
  })

  it('dismisses on close click and emits dismiss', async () => {
    const wrapper = mount(ErrorBanner, { props: { message: 'Error occurred' } })
    await wrapper.find('[data-testid="error-banner-dismiss"]').trigger('click')
    expect(wrapper.find('[data-testid="error-banner"]').exists()).toBe(false)
    expect(wrapper.emitted('dismiss')).toHaveLength(1)
  })
})
