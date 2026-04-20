import { test, expect } from '@playwright/test'
import { login, stubApiCatchAll } from '../helpers/auth'

const FAKE_WORK_ORDER = {
  id: 'wo-e2e-0001',
  title: 'E2E Replace Faulty PSU',
  status: 'open',
  priority: 'high',
  asset_id: 'b1111111-1111-1111-1111-111111111111',
  created_at: '2026-01-01T00:00:00Z',
}

test.describe('Work orders — maintenance page', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/work-orders**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [FAKE_WORK_ORDER],
          pagination: { total: 1, page: 1, limit: 20, total_pages: 1 },
        }),
      })
    )
    await page.route('**/api/v1/maintenance**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: [FAKE_WORK_ORDER] }),
      })
    )
    await stubApiCatchAll(page)
    await login(page)
  })

  test('maintenance route loads without crashing', async ({ page }) => {
    await page.goto('/maintenance')
    await expect(page.locator('body')).not.toBeEmpty()
  })

  test('maintenance page contains expected text', async ({ page }) => {
    await page.goto('/maintenance')
    await page.waitForLoadState('domcontentloaded')

    const bodyText = await page.locator('body').innerText()
    expect(bodyText.toLowerCase().match(/maintenan|work.?order|task|repair|schedul/i)).not.toBeNull()
  })

  test('maintenance page has no horizontal overflow', async ({ page }) => {
    await page.goto('/maintenance')
    const hasOverflow = await page.evaluate(() => {
      return document.body.scrollWidth > document.body.clientWidth + 1
    })
    expect(hasOverflow).toBe(false)
  })
})
