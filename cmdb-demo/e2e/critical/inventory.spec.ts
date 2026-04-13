import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('inventory tasks load', async ({ page }) => {
  await login(page)
  await page.goto('/inventory')
  await expect(page.locator('text=Inventory').first()).toBeVisible({ timeout: 10000 })
})
