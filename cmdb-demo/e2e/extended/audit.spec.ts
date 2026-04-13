import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('audit history loads', async ({ page }) => {
  await login(page)
  await page.goto('/audit')
  await expect(page.locator('body')).not.toBeEmpty()
})
