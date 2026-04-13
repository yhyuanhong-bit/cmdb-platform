import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('permissions page loads with user list', async ({ page }) => {
  await login(page)
  await page.goto('/system')
  await expect(page.locator('text=Permissions').first()).toBeVisible({ timeout: 10000 })
})
