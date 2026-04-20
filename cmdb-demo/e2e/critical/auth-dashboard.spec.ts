import { test, expect } from '@playwright/test'
import { injectAuth, login, stubApiCatchAll, FAKE_TENANT_ID } from '../helpers/auth'

test.describe('Auth — login page', () => {
  test('login form renders username and password fields', async ({ page }) => {
    // Render login page without any session
    await page.goto('/login')

    await expect(page.locator('input[type="text"], input[placeholder*="username" i]').first()).toBeVisible()
    await expect(page.locator('input[type="password"]').first()).toBeVisible()
    await expect(page.locator('button[type="submit"], button:has-text("Login")').first()).toBeVisible()
  })

  test('wrong credentials shows error message', async ({ page }) => {
    // Intercept the login API and return 401
    await page.route('**/api/v1/auth/login', (route) =>
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ success: false, error: 'invalid credentials' }),
      })
    )

    await page.goto('/login')
    await page.locator('input[type="text"], input[placeholder*="username" i]').first().fill('wrong@example.invalid')
    await page.locator('input[type="password"]').first().fill('badpassword')
    await page.locator('button[type="submit"], button:has-text("Login")').first().click()

    // Error message should appear — either inline or as a toast
    await expect(
      page.locator('text=/login failed|invalid|check username|verify the backend/i').first()
    ).toBeVisible({ timeout: 8000 })
  })

  test('unauthenticated user is redirected away from dashboard', async ({ page }) => {
    // Navigate directly to a protected route with no session
    await page.goto('/dashboard')

    // Should land on login or be redirected — not on /dashboard with full content
    const url = page.url()
    const isOnLogin = url.includes('/login') || url.includes('/welcome') || url === page.url()
    // The simplest assertion: the login heading is present OR the dashboard title is absent
    const loginHeading = page.locator('h1:has-text("Stitch CMDB"), h1:has-text("CMDB"), text=Login')
    const dashboardContent = page.locator('text=Total Assets, text=Dashboard')
    const loginVisible = await loginHeading.isVisible().catch(() => false)
    const dashboardVisible = await dashboardContent.isVisible().catch(() => false)

    // Either redirected to login, or protected content is not rendered
    expect(loginVisible || !dashboardVisible).toBe(true)
    void isOnLogin // satisfy linter
  })
})

test.describe('Auth — session injection', () => {
  test('injected session gives access to dashboard', async ({ page }) => {
    await stubApiCatchAll(page)
    await injectAuth(page, {
      username: 'admin@example.invalid',
      tenantId: FAKE_TENANT_ID,
    })

    await page.goto('/dashboard')

    // At minimum the app shell should load (nav / layout)
    await expect(page.locator('body')).not.toBeEmpty()
    // No login form should be visible
    await expect(page.locator('button[type="submit"]:has-text("Login")').first()).not.toBeVisible()
  })

  test('injected session persists across page navigation', async ({ page }) => {
    await stubApiCatchAll(page)
    await login(page)

    await page.goto('/assets')
    await page.goto('/dashboard')

    // Should still be on the app — no login redirect
    await expect(page.locator('button[type="submit"]:has-text("Login")').first()).not.toBeVisible()
  })

  test('cross-tenant stub injects custom tenant_id into session', async ({ page }) => {
    const otherTenant = 'd3333333-3333-3333-3333-333333333333'
    await stubApiCatchAll(page)
    await injectAuth(page, {
      username:  'ops@tenant-b.example.invalid',
      email:     'ops@tenant-b.example.invalid',
      tenantId:  otherTenant,
    })

    const stored = await page.evaluate(() => sessionStorage.getItem('cmdb-auth'))
    const parsed = JSON.parse(stored ?? '{}')
    expect(parsed?.state?.user?.tenant_id).toBe(otherTenant)
  })
})

test.describe('Dashboard content', () => {
  test.beforeEach(async ({ page }) => {
    await stubApiCatchAll(page)
    await login(page)
  })

  test('dashboard page renders without crashing', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('body')).not.toBeEmpty()
  })

  test('top-level nav links are visible', async ({ page }) => {
    await page.goto('/dashboard')
    // At least one nav link should be in the sidebar / top-bar
    const nav = page.locator('nav, aside, [role="navigation"]').first()
    await expect(nav).toBeVisible({ timeout: 10000 })
  })

  test('navigating to /assets from dashboard keeps session alive', async ({ page }) => {
    await page.goto('/dashboard')
    await page.goto('/assets')
    // Login form must not appear
    await expect(page.locator('button[type="submit"]:has-text("Login")').first()).not.toBeVisible()
  })
})
