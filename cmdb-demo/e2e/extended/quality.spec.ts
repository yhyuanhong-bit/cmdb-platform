import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('quality dashboard loads', async ({ page }) => {
  await login(page)
  await page.goto('/quality')
  await expect(page.locator('body')).not.toBeEmpty()
})
