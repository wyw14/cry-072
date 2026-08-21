import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { request } from '../api/client'
import type { Hazard, Page, QueueKind } from '../domain'

export const useHazardStore = defineStore('hazards', () => {
  const items = ref<Hazard[]>([])
  const selected = ref<Hazard | null>(null)
  const queue = ref<QueueKind>('focus')
  const loading = ref(false)
  const total = ref(0)
  const overdue = computed(() => items.value.filter((item) => new Date(item.due_at).getTime() < Date.now() && item.state !== 'closed').length)

  async function load(nextQueue: QueueKind = queue.value) {
    loading.value = true
    try {
      queue.value = nextQueue
      const page = await request<Page<Hazard>>(`/hazard-queues?queue=${nextQueue}&sort=due_at&page=1&page_size=100`)
      items.value = page.items
      total.value = page.total
    } finally { loading.value = false }
  }

  async function get(id: string) {
    loading.value = true
    try { selected.value = await request<Hazard>(`/hazards/${id}`); return selected.value }
    finally { loading.value = false }
  }

  async function assign(id: string, ownerID: string, version: number) {
    const updated = await request<Hazard>(`/hazards/${id}/assign`, { method: 'POST', headers: { 'If-Match': String(version) }, body: JSON.stringify({ owner_id: ownerID }) })
    selected.value = updated
    items.value = items.value.map((item) => item.id === id ? updated : item)
  }

  return { items, selected, queue, loading, total, overdue, load, get, assign }
})
