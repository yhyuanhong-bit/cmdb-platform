import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('predictive AI hub loads', async ({ page }) => {
  await login(page)
  await page.goto('/prediction')
  await expect(page.locator('body')).not.toBeEmpty()
})
