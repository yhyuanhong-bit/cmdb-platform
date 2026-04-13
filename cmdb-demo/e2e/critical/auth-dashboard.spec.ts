import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('login and view dashboard', async ({ page }) => {
  await login(page)
  await page.goto('/dashboard')
  await expect(page.locator('text=Dashboard').first()).toBeVisible({ timeout: 10000 })
})
