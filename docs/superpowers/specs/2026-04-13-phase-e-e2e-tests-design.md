# v1.2 Phase E: Playwright E2E Tests + CI Design Spec

> Date: 2026-04-13
> Status: Draft
> Prereqs: Phase A-D complete, frontend running on port 5175, backend on 8080
> Scope: Playwright setup, 15 E2E tests, CI integration

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Test count | 15 (all pages) | Page-load tests cost ~3 lines each; provides regression safety even for hardcoded pages |
| Framework | Playwright | Industry standard, built-in screenshots/traces, multi-browser |
| CI | Add job to existing .github/workflows/ci.yml | Don't create new workflow; extend existing |
| Auth strategy | Shared auth helper (login once, reuse state) | Avoid login overhead per test |

---

## E1: Infrastructure

### Install

```bash
cd cmdb-demo
npm install -D @playwright/test
npx playwright install chromium
```

### Config

`cmdb-demo/playwright.config.ts`:
- baseURL: `http://localhost:5175`
- testDir: `./e2e`
- projects: chromium only (add firefox/webkit later if needed)
- webServer: start `npm run dev` automatically
- retries: 1 on CI
- screenshot: on failure
- trace: on first retry

### Auth Helper

`cmdb-demo/e2e/helpers/auth.ts`:
- Login via API (`POST /api/v1/auth/login`)
- Save auth state to `e2e/.auth/user.json`
- Tests use `storageState` to skip login

---

## E2: 5 Critical Path Tests

| # | Test | File | What it verifies |
|---|------|------|-----------------|
| 1 | Login → Dashboard | `e2e/critical/auth-dashboard.spec.ts` | Login form works, redirects to dashboard, dashboard data loads |
| 2 | Asset CRUD | `e2e/critical/asset-crud.spec.ts` | List assets, view detail, verify fields populated |
| 3 | Work Order lifecycle | `e2e/critical/work-order.spec.ts` | Create WO, approve, start work, complete |
| 4 | Inventory scan | `e2e/critical/inventory.spec.ts` | List tasks, view items, verify table renders |
| 5 | Alert acknowledge | `e2e/critical/alerts.spec.ts` | List alerts, acknowledge one, verify status changes |

Each test:
- Uses auth helper (pre-authenticated)
- Navigates to page
- Performs action
- Asserts visible outcome
- No mocks — hits real backend

---

## E3: 10 Extended Tests

| # | Test | File | What it verifies |
|---|------|------|-----------------|
| 6 | BIA assessment | `e2e/extended/bia.spec.ts` | Page loads, BIA table visible |
| 7 | Quality scan | `e2e/extended/quality.spec.ts` | Page loads, quality scores visible |
| 8 | Role permissions | `e2e/extended/permissions.spec.ts` | System settings → permissions tab, user list visible |
| 9 | 3D datacenter | `e2e/extended/datacenter-3d.spec.ts` | Page loads without crash |
| 10 | Location hierarchy | `e2e/extended/locations.spec.ts` | Navigate territory → region → city |
| 11 | Sync management | `e2e/extended/sync.spec.ts` | /system/sync loads, both tabs work |
| 12 | System settings | `e2e/extended/settings.spec.ts` | Settings page, 4 tabs switch correctly |
| 13 | Audit history | `e2e/extended/audit.spec.ts` | Audit list loads, entries visible |
| 14 | Predictive AI | `e2e/extended/predictive.spec.ts` | Page loads, hub visible |
| 15 | Energy monitor | `e2e/extended/energy.spec.ts` | Page loads, charts visible |

---

## E4: CI Integration

Add `e2e-tests` job to `.github/workflows/ci.yml`:

```yaml
e2e-tests:
  name: E2E Tests
  runs-on: ubuntu-latest
  needs: [go-backend]  # backend must build first
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with: { node-version: '20' }
    - run: npm ci
      working-directory: cmdb-demo
    - run: npx playwright install chromium --with-deps
      working-directory: cmdb-demo
    - run: npx playwright test
      working-directory: cmdb-demo
    - uses: actions/upload-artifact@v4
      if: failure()
      with:
        name: playwright-report
        path: cmdb-demo/playwright-report/
```

Note: E2E on CI requires backend + DB running. For now, mark the job with `if: false` (disabled) — enable when CI has Docker Compose setup. The tests are still valuable for local development.

---

## Files

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-demo/playwright.config.ts` | Playwright configuration |
| `cmdb-demo/e2e/helpers/auth.ts` | Login helper + storage state |
| `cmdb-demo/e2e/critical/auth-dashboard.spec.ts` | Login + dashboard test |
| `cmdb-demo/e2e/critical/asset-crud.spec.ts` | Asset list + detail test |
| `cmdb-demo/e2e/critical/work-order.spec.ts` | Work order lifecycle test |
| `cmdb-demo/e2e/critical/inventory.spec.ts` | Inventory list test |
| `cmdb-demo/e2e/critical/alerts.spec.ts` | Alert acknowledge test |
| `cmdb-demo/e2e/extended/bia.spec.ts` | BIA page load test |
| `cmdb-demo/e2e/extended/quality.spec.ts` | Quality page load test |
| `cmdb-demo/e2e/extended/permissions.spec.ts` | Permissions page test |
| `cmdb-demo/e2e/extended/datacenter-3d.spec.ts` | 3D datacenter load test |
| `cmdb-demo/e2e/extended/locations.spec.ts` | Location hierarchy nav test |
| `cmdb-demo/e2e/extended/sync.spec.ts` | Sync management test |
| `cmdb-demo/e2e/extended/settings.spec.ts` | System settings tabs test |
| `cmdb-demo/e2e/extended/audit.spec.ts` | Audit history test |
| `cmdb-demo/e2e/extended/predictive.spec.ts` | Predictive AI load test |
| `cmdb-demo/e2e/extended/energy.spec.ts` | Energy monitor load test |

### Modified Files

| File | Change |
|------|--------|
| `cmdb-demo/package.json` | Add @playwright/test dev dep |
| `.github/workflows/ci.yml` | Add e2e-tests job (disabled for now) |

---

## Acceptance Criteria

- [ ] 15 E2E tests all pass locally
- [ ] Tests produce screenshots on failure
- [ ] CI workflow has E2E job (disabled, ready to enable)
- [ ] Auth helper avoids login per test
