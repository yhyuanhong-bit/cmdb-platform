/**
 * Smoke tests — verify the app shell loads and critical routes are reachable.
 * All API calls are intercepted so no backend is needed.
 */
import { test, expect } from '@playwright/test'
import { login, stubApiCatchAll } from '../helpers/auth'

const CRITICAL_ROUTES = [
  { path: '/dashboard',  label: 'Dashboard'  },
  { path: '/assets',     label: 'Assets'     },
  { path: '/monitoring', label: 'Monitoring' },
  { path: '/maintenance', label: 'Maintenance' },
  { path: '/audit',      label: 'Audit'      },
]

test.describe('Smoke — app shell', () => {
  test('root path loads without JS error', async ({ page }) => {
    const jsErrors: string[] = []
    page.on('pageerror', (err) => jsErrors.push(err.message))

    await stubApiCatchAll(page)
    await login(page)
    await page.goto('/')

    // Allow lazy-loaded routes to settle
    await page.waitForLoadState('networkidle').catch(() => {})

    // No unhandled JS errors from app code
    const criticalErrors = jsErrors.filter(
      (e) =>
        !e.includes('ResizeObserver') &&
        !e.includes('Non-Error promise') &&
        !e.includes('network')
    )
    expect(criticalErrors).toHaveLength(0)
  })

  test('login page is accessible at /login', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('input[type="password"]').first()).toBeVisible({ timeout: 8000 })
  })

  test('authenticated user is not redirected to login from root', async ({ page }) => {
    await stubApiCatchAll(page)
    await login(page)
    await page.goto('/')

    // Login form should not be presented to an authenticated user
    await page.waitForLoadState('networkidle').catch(() => {})
    const loginBtn = page.locator('button[type="submit"]:has-text("Login")').first()
    const isLoginVisible = await loginBtn.isVisible()
    expect(isLoginVisible).toBe(false)
  })
})

for (const route of CRITICAL_ROUTES) {
  test.describe(`Smoke — ${route.label}`, () => {
    test.beforeEach(async ({ page }) => {
      await stubApiCatchAll(page)
      await login(page)
    })

    test(`${route.path} renders without a blank white screen`, async ({ page }) => {
      await page.goto(route.path)
      await page.waitForLoadState('domcontentloaded')

      const bodyText = await page.locator('body').innerText()
      expect(bodyText.trim().length).toBeGreaterThan(0)
    })

    test(`${route.path} does not crash with an unhandled React error boundary`, async ({ page }) => {
      const jsErrors: string[] = []
      page.on('pageerror', (err) => jsErrors.push(err.message))

      await page.goto(route.path)
      await page.waitForLoadState('networkidle').catch(() => {})

      const criticalErrors = jsErrors.filter(
        (e) =>
          e.toLowerCase().includes('uncaught') ||
          e.toLowerCase().includes('error boundary') ||
          e.toLowerCase().includes('minified react error')
      )
      expect(criticalErrors).toHaveLength(0)
    })
  })
}
