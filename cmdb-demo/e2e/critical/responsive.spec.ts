/**
 * Responsive breakpoint tests.
 *
 * These tests verify that the app shell renders without horizontal overflow
 * and that key navigation elements are accessible at each breakpoint.
 * They do NOT require a live backend — all API calls are stubbed.
 */
import { test, expect } from '@playwright/test'
import { login, stubApiCatchAll } from '../helpers/auth'

const BREAKPOINTS = [
  { name: 'mobile-375',  width: 375,  height: 812  },
  { name: 'tablet-768',  width: 768,  height: 1024 },
  { name: 'desktop-1024', width: 1024, height: 768 },
  { name: 'desktop-1440', width: 1440, height: 900 },
]

for (const bp of BREAKPOINTS) {
  test.describe(`Responsive — ${bp.name}`, () => {
    test.use({ viewport: { width: bp.width, height: bp.height } })

    test(`app shell loads without overflow at ${bp.width}px`, async ({ page }) => {
      await stubApiCatchAll(page)
      await login(page)
      await page.goto('/dashboard')

      // Check for horizontal overflow on the root element
      const hasOverflow = await page.evaluate(() => {
        const el = document.body
        return el.scrollWidth > el.clientWidth + 1 // 1px tolerance
      })
      expect(hasOverflow).toBe(false)
    })

    test(`body renders at ${bp.width}px without blank screen`, async ({ page }) => {
      await stubApiCatchAll(page)
      await login(page)
      await page.goto('/dashboard')

      const bodyText = await page.locator('body').innerText()
      // At minimum the page should render something
      expect(bodyText.trim().length).toBeGreaterThan(0)
    })

    test(`assets page renders at ${bp.width}px`, async ({ page }) => {
      await stubApiCatchAll(page)
      await login(page)
      await page.goto('/assets')

      await expect(page.locator('body')).not.toBeEmpty()

      const hasOverflow = await page.evaluate(() => {
        return document.body.scrollWidth > document.body.clientWidth + 1
      })
      expect(hasOverflow).toBe(false)
    })
  })
}

test.describe('Responsive — nav visibility', () => {
  test('sidebar or hamburger nav is visible on mobile (375px)', async ({ page }) => {
    test.setTimeout(20000)
    await page.setViewportSize({ width: 375, height: 812 })
    await stubApiCatchAll(page)
    await login(page)
    await page.goto('/dashboard')

    // Either a hamburger menu button or a sidebar nav element should be present
    const navElements = page.locator('nav, aside, [role="navigation"], button[aria-label*="menu" i]')
    await expect(navElements.first()).toBeVisible({ timeout: 10000 })
  })

  test('full nav is accessible on desktop (1440px)', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await stubApiCatchAll(page)
    await login(page)
    await page.goto('/dashboard')

    const nav = page.locator('nav, aside, [role="navigation"]').first()
    await expect(nav).toBeVisible({ timeout: 10000 })
  })

  test('login page has no horizontal overflow at 375px', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 })
    await page.goto('/login')

    const hasOverflow = await page.evaluate(() => {
      return document.body.scrollWidth > document.body.clientWidth + 1
    })
    expect(hasOverflow).toBe(false)
  })
})
