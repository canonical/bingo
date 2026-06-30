import { test, expect } from '@playwright/test'

test.describe('View paste', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('/api/v1/pastes/abc12', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'abc12',
          url: 'http://localhost:4173/abc12',
          raw_url: 'http://localhost:4173/api/v1/pastes/abc12/raw',
          content: 'def greet():\n    return "hello"',
          language: 'python',
          title: 'Greeting',
          size_bytes: 30,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
  })

  test('shows paste title and content', async ({ page }) => {
    await page.goto('/abc12')
    await expect(page.getByText('Greeting')).toBeVisible()
    await expect(page.getByText(/def greet/)).toBeVisible()
  })

  test('shows language label', async ({ page }) => {
    await page.goto('/abc12')
    await expect(page.getByText(/python/i)).toBeVisible()
  })

  test('shows View raw link pointing to raw URL', async ({ page }) => {
    await page.goto('/abc12')
    const rawLink = page.getByRole('link', { name: /view raw/i })
    await expect(rawLink).toBeVisible()
    await expect(rawLink).toHaveAttribute('href', /\/api\/v1\/pastes\/abc12\/raw/)
  })

  test('shows not-found message for expired/missing paste', async ({ page }) => {
    await page.route('/api/v1/pastes/nope', (route) =>
      route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'paste_not_found', message: 'not found' } }),
      })
    )
    await page.goto('/nope')
    await expect(page.getByText(/not found/i)).toBeVisible()
  })
})
