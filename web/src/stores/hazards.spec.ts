import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useHazardStore } from './hazards'

describe('hazard store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('localStorage', { getItem: () => null, setItem: () => undefined, removeItem: () => undefined, clear: () => undefined })
  })

  it('loads focus queue without mixing normal records', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ items: [{ id: 'h1', queue: 'focus', state: 'handling', due_at: '2026-08-22T00:00:00Z' }], total: 1, page: 1, page_size: 100, total_pages: 1 }),
    })
    vi.stubGlobal('fetch', fakeFetch)
    const store = useHazardStore()
    await store.load('focus')
    expect(store.items).toHaveLength(1)
    expect(store.queue).toBe('focus')
    expect(String(fakeFetch.mock.calls[0][0])).toContain('queue=focus')
    vi.unstubAllGlobals()
  })
})
