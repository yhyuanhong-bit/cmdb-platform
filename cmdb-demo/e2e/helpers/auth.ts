import { Page } from '@playwright/test'

const API_URL = 'http://localhost:8080/api/v1'

export async function login(page: Page, username = 'admin', password = 'admin123') {
  // Login via API to get token
  const response = await page.request.post(`${API_URL}/auth/login`, {
    data: { username, password },
  })
  const json = await response.json()
  const token = json.data?.access_token

  if (!token) throw new Error('Login failed: no token returned')

  // Set token in sessionStorage so the app picks it up
  // The Zustand auth store uses persist key 'cmdb-auth' with sessionStorage
  await page.goto('/')
  await page.evaluate((t) => {
    sessionStorage.setItem('cmdb-auth', JSON.stringify({
      state: { accessToken: t, refreshToken: '', user: null, isAuthenticated: true },
      version: 0,
    }))
  }, token)

  // Reload to apply auth state
  await page.goto('/')
}
