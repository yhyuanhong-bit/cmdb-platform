import { Page } from '@playwright/test'

// Fake UUID used throughout tests — not a real tenant
export const FAKE_TENANT_ID = 'a0000000-0000-0000-0000-000000000001'
export const FAKE_ASSET_ID  = 'b1111111-1111-1111-1111-111111111111'
export const FAKE_USER_ID   = 'c2222222-2222-2222-2222-222222222222'

// Stub access token — not a real JWT, never sent to a real backend
const STUB_TOKEN = 'eyJhbGciOiJub25lIn0.e30.'

/**
 * Inject a fake auth session directly into sessionStorage so the Zustand
 * auth store sees an authenticated state without hitting the API.
 * This keeps tests hermetic — no real backend required.
 */
export async function injectAuth(
  page: Page,
  overrides: {
    username?: string
    email?: string
    tenantId?: string
    token?: string
  } = {}
) {
  const {
    username = 'admin@example.invalid',
    email    = 'admin@example.invalid',
    tenantId = FAKE_TENANT_ID,
    token    = STUB_TOKEN,
  } = overrides

  await page.goto('/')

  await page.evaluate(
    ({ t, u, em, tid, uid }) => {
      sessionStorage.setItem(
        'cmdb-auth',
        JSON.stringify({
          state: {
            accessToken: t,
            refreshToken: t,
            isAuthenticated: true,
            user: {
              id: uid,
              username: u,
              email: em,
              tenant_id: tid,
              roles: ['admin'],
            },
          },
          version: 0,
        })
      )
    },
    {
      t:   token,
      u:   username,
      em:  email,
      tid: tenantId,
      uid: FAKE_USER_ID,
    }
  )
}

/**
 * Navigate to a page and inject auth state, then wait for the page to settle.
 * Equivalent to `login(page)` used in legacy stubs — kept for backward compat.
 */
export async function login(page: Page, username = 'admin@example.invalid', _password = 'ignored') {
  await injectAuth(page, { username, email: username })
  await page.goto('/')
}

/** Build a reusable API-mock interceptor for the assets list.
 * The frontend reads `pagination.page_size` (matches ApiListResponse type);
 * older stubs used `limit` which broke list rendering after Wave H. */
export function stubAssetsList(page: Page, assets: object[] = []) {
  return page.route('**/api/v1/assets**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        data: assets,
        pagination: { total: assets.length, page: 1, page_size: 20, total_pages: 1 },
      }),
    })
  })
}

/** Build a reusable API-mock interceptor for a single asset GET. */
export function stubAsset(page: Page, asset: object) {
  return page.route(`**/api/v1/assets/${(asset as { id: string }).id}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data: asset }),
    })
  })
}

/** Stub a POST /assets to echo back the posted body with a generated id. */
export function stubCreateAsset(page: Page, returnedAsset: object) {
  return page.route('**/api/v1/assets', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: returnedAsset }),
      })
    } else {
      await route.continue()
    }
  })
}

/** Stub DELETE /assets/:id with 204 */
export function stubDeleteAsset(page: Page, id: string) {
  return page.route(`**/api/v1/assets/${id}`, async (route) => {
    if (route.request().method() === 'DELETE') {
      await route.fulfill({ status: 204 })
    } else {
      await route.continue()
    }
  })
}

/** Catch-all: return empty success for any unmatched API call */
export function stubApiCatchAll(page: Page) {
  return page.route('**/api/v1/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data: null }),
    })
  })
}
