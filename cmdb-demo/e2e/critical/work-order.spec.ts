import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('work order list loads', async ({ page }) => {
  await login(page)
  await page.goto('/maintenance')
  await expect(page.locator('text=Maintenance').first()).toBeVisible({ timeout: 10000 })
})
