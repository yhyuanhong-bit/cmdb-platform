export interface ApiMeta {
  request_id: string
}

export interface ApiResponse<T> {
  data: T
  meta: ApiMeta
}

export interface ApiListResponse<T> {
  data: T[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
  meta: ApiMeta
}

export interface ApiError {
  error: {
    code: string
    message: string
  }
  meta: ApiMeta
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface CurrentUser {
  id: string
  username: string
  display_name: string
  email: string
  permissions: Record<string, string[]>
}
