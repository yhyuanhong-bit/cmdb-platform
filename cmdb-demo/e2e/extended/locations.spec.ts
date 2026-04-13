import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('location hierarchy loads', async ({ page }) => {
  await login(page)
  await page.goto('/locations')
  await expect(page.locator('body')).not.toBeEmpty()
})
