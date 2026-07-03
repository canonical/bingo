import { test as base, expect, type Route } from '@playwright/test'

/**
 * Extends the base Playwright test with an automatic mock for GET /api/v1/me.
 * This prevents AuthGuard from redirecting to /auth/login during tests.
 * Auth-specific behaviour (the "Log in" nav link) is still tested via the
 * csrf_token cookie, which is absent in all test runs by default.
 */
export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route('/api/v1/me', (route: Route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ auth_enabled: false, authenticated: false }),
      })
    )
    await use(page)
  },
})

export { expect }
