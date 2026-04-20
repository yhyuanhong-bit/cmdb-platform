import { test, expect } from '@playwright/test'
import { login, stubApiCatchAll } from '../helpers/auth'

const FAKE_TASK = {
  id: 'inv-task-e2e-0001',
  name: 'E2E Inventory Scan Q1',
  status: 'pending',
  asset_count: 5,
  created_at: '2026-01-01T00:00:00Z',
}

test.describe('Inventory — task list', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/inventory**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [FAKE_TASK],
          pagination: { total: 1, page: 1, limit: 20, total_pages: 1 },
        }),
      })
    )
    await stubApiCatchAll(page)
    await login(page)
  })

  test('inventory route loads without crashing', async ({ page }) => {
    await page.goto('/inventory')
    await expect(page.locator('body')).not.toBeEmpty()
  })

  test('inventory page contains expected text', async ({ page }) => {
    await page.goto('/inventory')
    await page.waitForLoadState('domcontentloaded')

    const bodyText = await page.locator('body').innerText()
    expect(bodyText.toLowerCase().match(/inventor|task|scan|asset/i)).not.toBeNull()
  })

  test('inventory page has no horizontal overflow', async ({ page }) => {
    await page.goto('/inventory')
    const hasOverflow = await page.evaluate(() => {
      return document.body.scrollWidth > document.body.clientWidth + 1
    })
    expect(hasOverflow).toBe(false)
  })
})
