import { test, expect } from './fixtures'
import type { Route } from '@playwright/test'

test.describe('Create paste flow', () => {
  test.beforeEach(async ({ page }) => {
    // Mock GET /api/v1/languages
    await page.route('/api/v1/languages', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ languages: ['text', 'python', 'go'] }),
      })
    )
    // Mock POST /api/v1/pastes
    await page.route('/api/v1/pastes', async (route: Route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            key: 'testk',
            url: 'http://localhost:4173/testk',
            raw_url: 'http://localhost:4173/api/v1/pastes/testk/raw',
            language: 'python',
            size_bytes: 12,
            expires_at: '2027-01-01T00:00:00Z',
            created_at: '2026-06-30T00:00:00Z',
          }),
        })
      }
      return route.continue()
    })
    // Mock GET /api/v1/pastes/testk (for redirect)
    await page.route('/api/v1/pastes/testk', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'testk',
          url: 'http://localhost:4173/testk',
          raw_url: 'http://localhost:4173/api/v1/pastes/testk/raw',
          content: 'print("hello")',
          language: 'python',
          size_bytes: 14,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
  })

  test('shows form on home page', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('textbox', { name: /content/i })).toBeVisible()
  })

  test('redirects to paste viewer after successful creation', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('textbox', { name: /content/i }).fill('print("hello")')
    await page.getByRole('button', { name: /create paste/i }).click()
    await expect(page).toHaveURL('/testk')
    await expect(page.getByText(/print/)).toBeVisible()
  })

  test('shows error notification when API returns 413', async ({ page }) => {
    await page.unroute('/api/v1/pastes')
    await page.route('/api/v1/pastes', (route) =>
      route.fulfill({
        status: 413,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'content_too_large', message: 'Paste exceeds limit.' } }),
      })
    )
    await page.goto('/')
    await page.getByRole('textbox', { name: /content/i }).fill('x')
    await page.getByRole('button', { name: /create paste/i }).click()
    await expect(page.getByRole('alert')).toBeVisible()
  })
})
