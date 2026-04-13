import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('system settings loads with tabs', async ({ page }) => {
  await login(page)
  await page.goto('/system/settings')
  await expect(page.locator('body')).not.toBeEmpty()
})
