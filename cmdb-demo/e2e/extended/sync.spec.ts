import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('sync management page loads with tabs', async ({ page }) => {
  await login(page)
  await page.goto('/system/sync')
  await expect(page.locator('text=Sync Status').first()).toBeVisible({ timeout: 10000 })
  await expect(page.locator('text=Conflicts').first()).toBeVisible()
})
