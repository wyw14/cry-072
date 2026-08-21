import type { APIError } from '../domain'

const API_BASE = import.meta.env.VITE_API_BASE ?? '/api/v1'

export class RequestError extends Error {
  constructor(public status: number, public detail: APIError) {
    super(detail.message)
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  headers.set('X-Operator-ID', localStorage.getItem('operator_id') ?? 'operator-demo')
  headers.set('X-Operator-Role', localStorage.getItem('operator_role') ?? 'supervisor')
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers })
  if (!response.ok) {
    const detail = await response.json() as APIError
    throw new RequestError(response.status, detail)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function formatDate(value: string): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short', hour12: false }).format(new Date(value))
}
