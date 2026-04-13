import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('energy monitor loads', async ({ page }) => {
  await login(page)
  await page.goto('/monitoring/energy')
  await expect(page.locator('body')).not.toBeEmpty()
})
