import { useAuthStore } from '../../stores/authStore'

const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

export class ApiRequestError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number
  ) {
    super(message)
    this.name = 'ApiRequestError'
  }
}

const RATE_LIMIT_MAX_RETRIES = 3
const RATE_LIMIT_BASE_DELAY_MS = 250

function parseRetryAfter(headerValue: string | null): number | null {
  if (!headerValue) return null
  const seconds = Number(headerValue)
  if (Number.isFinite(seconds) && seconds >= 0) return seconds * 1000
  const date = Date.parse(headerValue)
  if (!Number.isNaN(date)) return Math.max(0, date - Date.now())
  return null
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

class ApiClient {
  private getToken(): string | null {
    return useAuthStore.getState().accessToken
  }

  private async request<T>(path: string, options: RequestInit = {}, attempt = 0): Promise<T> {
    const token = this.getToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const res = await fetch(`${BASE_URL}${path}`, {
      ...options,
      headers,
    })

    if (res.status === 429 && attempt < RATE_LIMIT_MAX_RETRIES) {
      const retryAfter = parseRetryAfter(res.headers.get('Retry-After'))
      const backoff = RATE_LIMIT_BASE_DELAY_MS * Math.pow(2, attempt)
      const jitter = Math.random() * RATE_LIMIT_BASE_DELAY_MS
      await sleep(retryAfter ?? backoff + jitter)
      return this.request<T>(path, options, attempt + 1)
    }

    let json: { data?: unknown; error?: { code?: string; message?: string } } | null
    try {
      json = (await res.json()) as typeof json
    } catch {
      if (!res.ok) {
        throw new ApiRequestError('UNKNOWN', res.statusText || 'Request failed', res.status)
      }
      return undefined as unknown as T
    }

    if (!res.ok || json?.error) {
      const error = json?.error ?? { code: 'UNKNOWN', message: res.statusText }

      if (res.status === 401 && error.code === 'INVALID_TOKEN') {
        const refreshed = await useAuthStore.getState().refreshTokens()
        if (refreshed) {
          return this.request<T>(path, options)
        }
        useAuthStore.getState().logout()
      }

      if (res.status === 503 && error.code === 'SYNC_IN_PROGRESS') {
        window.dispatchEvent(new CustomEvent('sync-in-progress'))
      }

      throw new ApiRequestError(error.code ?? 'UNKNOWN', error.message ?? 'Request failed', res.status)
    }

    // Dev-only: warn if response lacks expected envelope shape
    if (import.meta.env.DEV && json !== null && typeof json === 'object') {
      if (!('data' in json) && !('error' in json) && !Array.isArray(json)) {
        console.warn(`[API] Unexpected response shape for ${path}:`, Object.keys(json).slice(0, 5))
      }
    }

    return json as T
  }

  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    const query = params ? '?' + new URLSearchParams(params).toString() : ''
    return this.request<T>(path + query)
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async del(path: string): Promise<void> {
    await this.request(path, { method: 'DELETE' })
  }
}

export const apiClient = new ApiClient()
