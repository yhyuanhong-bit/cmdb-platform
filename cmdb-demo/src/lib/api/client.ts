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

class ApiClient {
  private getToken(): string | null {
    return useAuthStore.getState().accessToken
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
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

    let json: any
    try {
      json = await res.json()
    } catch {
      if (!res.ok) {
        throw new ApiRequestError('UNKNOWN', res.statusText || 'Request failed', res.status)
      }
      return undefined as unknown as T
    }

    if (!res.ok || json.error) {
      const error = json.error || { code: 'UNKNOWN', message: res.statusText }

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

      throw new ApiRequestError(error.code, error.message, res.status)
    }

    return json
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

  async del(path: string): Promise<void> {
    await this.request(path, { method: 'DELETE' })
  }
}

export const apiClient = new ApiClient()
