import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('BIA assessment page loads', async ({ page }) => {
  await login(page)
  await page.goto('/bia')
  await expect(page.locator('body')).not.toBeEmpty()
})
