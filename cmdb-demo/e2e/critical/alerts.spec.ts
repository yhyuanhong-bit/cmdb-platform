import { test, expect } from '@playwright/test'
import { login, stubApiCatchAll } from '../helpers/auth'

const FAKE_ALERT = {
  id: 'alert-e2e-0001',
  name: 'E2E CPU High',
  asset_id: 'b1111111-1111-1111-1111-111111111111',
  severity: 'critical',
  status: 'active',
  message: 'CPU usage above threshold',
  created_at: '2026-01-01T00:00:00Z',
}

test.describe('Alerts — monitoring page', () => {
  test.beforeEach(async ({ page }) => {
    // Stub alerts list endpoint
    await page.route('**/api/v1/alerts**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [FAKE_ALERT],
          pagination: { total: 1, page: 1, limit: 20, total_pages: 1 },
        }),
      })
    )
    await page.route('**/api/v1/monitoring**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: [FAKE_ALERT] }),
      })
    )
    await stubApiCatchAll(page)
    await login(page)
  })

  test('monitoring route loads without crashing', async ({ page }) => {
    await page.goto('/monitoring')
    await expect(page.locator('body')).not.toBeEmpty()
  })

  test('page contains expected monitoring-related text', async ({ page }) => {
    await page.goto('/monitoring')
    await page.waitForLoadState('domcontentloaded')

    const bodyText = await page.locator('body').innerText()
    // The page must contain at least one monitoring-related keyword
    expect(
      bodyText.toLowerCase().match(/monitor|alert|severity|status|sensor/i)
    ).not.toBeNull()
  })

  test('no horizontal overflow on monitoring page', async ({ page }) => {
    await page.goto('/monitoring')
    const hasOverflow = await page.evaluate(() => {
      return document.body.scrollWidth > document.body.clientWidth + 1
    })
    expect(hasOverflow).toBe(false)
  })
})
