import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('asset list loads and shows data', async ({ page }) => {
  await login(page)
  await page.goto('/assets')
  await expect(page.locator('text=Asset').first()).toBeVisible({ timeout: 10000 })
})
