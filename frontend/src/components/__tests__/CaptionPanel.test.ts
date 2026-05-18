import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import CaptionPanel from '../CaptionPanel.vue'
import * as captionsApi from '../../api/captions'

enableAutoUnmount(afterEach)

// Mock the captions API module.
vi.mock('../../api/captions', () => ({
  listCaptions: vi.fn(),
  saveCaption: vi.fn(),
}))

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: { template: '<div />' } },
    { path: '/studies', name: 'studies', component: { template: '<div />' } },
  ],
})

describe('CaptionPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(captionsApi.listCaptions).mockResolvedValue({ captions: [] })
    vi.mocked(captionsApi.saveCaption).mockResolvedValue({
      id: 'cap1',
      sample_id: 'sample1',
      study_id: 'manual-study',
      text: 'Manual text',
      study: { id: 'manual-study', name: 'Manual', slug: 'manual', provider: 'manual', model: 'manual' },
      created_at: '2026-01-01',
    })
  })

  it('shows "No captions yet" when sample has no captions', async () => {
    const wrapper = mount(CaptionPanel, {
      props: { sampleId: 'sample1' },
      global: { plugins: [router] },
    })
    await vi.waitFor(() => {
      const emptyEl = wrapper.find('[data-testid="caption-empty"]')
      expect(emptyEl.exists()).toBe(true)
      expect(emptyEl.text()).toContain('No captions yet')
    })
  })

  it('shows link to studies view when no captions', async () => {
    const wrapper = mount(CaptionPanel, {
      props: { sampleId: 'sample1' },
      global: { plugins: [router] },
    })
    await vi.waitFor(() => {
      expect(wrapper.find('[data-testid="caption-studies-link"]').exists()).toBe(true)
    })
  })

  it('renders captions grouped by study', async () => {
    vi.mocked(captionsApi.listCaptions).mockResolvedValue({
      captions: [
        {
          id: 'c1',
          sample_id: 'sample1',
          study_id: 's1',
          text: 'GPT caption text',
          study: { id: 's1', name: 'GPT Study', slug: 'gpt-study', provider: 'openai', model: 'gpt-4' },
          created_at: '2026-01-01',
        },
      ],
    })

    const wrapper = mount(CaptionPanel, {
      props: { sampleId: 'sample1' },
      global: { plugins: [router] },
    })

    await vi.waitFor(() => {
      expect(wrapper.find('[data-testid="caption-group-gpt-study"]').exists()).toBe(true)
    })
    expect(wrapper.text()).toContain('GPT Study')
    expect(wrapper.text()).toContain('openai / gpt-4')
    expect(wrapper.text()).toContain('GPT caption text')
  })

  it('emits navigate when prev/next clicked', async () => {
    const wrapper = mount(CaptionPanel, {
      props: { sampleId: 'sample1' },
      global: { plugins: [router] },
    })

    await wrapper.find('[data-testid="caption-nav-prev"]').trigger('click')
    expect(wrapper.emitted('navigate')).toEqual([['prev']])

    await wrapper.find('[data-testid="caption-nav-next"]').trigger('click')
    expect(wrapper.emitted('navigate')).toEqual([['prev'], ['next']])
  })

  it('renders manual caption with editable textarea', async () => {
    vi.mocked(captionsApi.listCaptions).mockResolvedValue({
      captions: [
        {
          id: 'c1',
          sample_id: 'sample1',
          study_id: 's1',
          text: 'Original caption',
          study: { id: 's1', name: 'GPT Study', slug: 'gpt-study', provider: 'openai', model: 'gpt-4' },
          created_at: '2026-01-01',
        },
        {
          id: 'c2',
          sample_id: 'sample1',
          study_id: 's2',
          text: 'Manual edit',
          study: { id: 's2', name: 'Manual', slug: 'manual', provider: 'manual', model: 'manual' },
          created_at: '2026-01-01',
        },
      ],
    })

    const wrapper = mount(CaptionPanel, {
      props: { sampleId: 'sample1' },
      global: { plugins: [router] },
    })

    await vi.waitFor(() => {
      expect(wrapper.find('[data-testid="caption-group-manual"]').exists()).toBe(true)
    })
    // Manual caption has a textarea, not plain text.
    const textarea = wrapper.find('[data-testid="caption-text-manual"]')
    expect(textarea.element.tagName).toBe('TEXTAREA')
  })
})
