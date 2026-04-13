import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('alerts list loads', async ({ page }) => {
  await login(page)
  await page.goto('/monitoring')
  await expect(page.locator('text=Monitoring').first()).toBeVisible({ timeout: 10000 })
})
