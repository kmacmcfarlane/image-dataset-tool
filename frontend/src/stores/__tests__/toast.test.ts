import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useToastStore } from '../toast'

describe('useToastStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('defaults threshold to warning', () => {
    const store = useToastStore()
    expect(store.threshold).toBe('warning')
  })

  it('loads threshold from localStorage', () => {
    localStorage.setItem('toast-level-threshold', 'info')
    // Need a fresh pinia since store reads localStorage on creation
    setActivePinia(createPinia())
    const store = useToastStore()
    expect(store.threshold).toBe('info')
  })

  it('persists threshold to localStorage on setThreshold', () => {
    const store = useToastStore()
    store.setThreshold('error')
    expect(localStorage.getItem('toast-level-threshold')).toBe('error')
    expect(store.threshold).toBe('error')
  })

  it('ignores invalid localStorage values and defaults to warning', () => {
    localStorage.setItem('toast-level-threshold', 'invalid')
    setActivePinia(createPinia())
    const store = useToastStore()
    expect(store.threshold).toBe('warning')
  })

  it('filters visible toasts based on threshold', () => {
    const store = useToastStore()
    // Default threshold is warning
    store.addToast('info', 'info message')
    store.addToast('warning', 'warning message')
    store.addToast('error', 'error message')

    expect(store.toasts).toHaveLength(3)
    expect(store.visibleToasts).toHaveLength(2)
    expect(store.visibleToasts.map(t => t.level)).toEqual(['warning', 'error'])
  })

  it('shows all toasts when threshold is info', () => {
    const store = useToastStore()
    store.setThreshold('info')
    store.addToast('info', 'info message')
    store.addToast('warning', 'warning message')
    store.addToast('error', 'error message')

    expect(store.visibleToasts).toHaveLength(3)
  })

  it('shows only errors when threshold is error', () => {
    const store = useToastStore()
    store.setThreshold('error')
    store.addToast('info', 'info message')
    store.addToast('warning', 'warning message')
    store.addToast('error', 'error message')

    expect(store.visibleToasts).toHaveLength(1)
    expect(store.visibleToasts[0].level).toBe('error')
  })

  it('auto-dismisses toasts after 5 seconds', () => {
    const store = useToastStore()
    store.setThreshold('info')
    store.addToast('info', 'will be dismissed')

    expect(store.toasts).toHaveLength(1)
    vi.advanceTimersByTime(5000)
    expect(store.toasts).toHaveLength(0)
  })

  it('dismissToast removes specific toast by id', () => {
    const store = useToastStore()
    store.setThreshold('info')
    store.addToast('info', 'first')
    store.addToast('warning', 'second')

    const firstId = store.toasts[0].id
    store.dismissToast(firstId)

    expect(store.toasts).toHaveLength(1)
    expect(store.toasts[0].message).toBe('second')
  })

  it('clearAll removes all toasts', () => {
    const store = useToastStore()
    store.setThreshold('info')
    store.addToast('info', 'a')
    store.addToast('warning', 'b')
    store.addToast('error', 'c')

    store.clearAll()
    expect(store.toasts).toHaveLength(0)
  })
})
