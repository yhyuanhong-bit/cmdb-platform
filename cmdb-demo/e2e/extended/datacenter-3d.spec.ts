import { test, expect } from '@playwright/test'
import { login } from '../helpers/auth'

test('3D datacenter page loads', async ({ page }) => {
  await login(page)
  await page.goto('/racks/3d')
  await expect(page.locator('body')).not.toBeEmpty()
})
