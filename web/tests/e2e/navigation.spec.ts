import { test, expect } from '@playwright/test'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('/api/v1/languages', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ languages: ['text'] }),
      })
    )
  })

  test('shows New paste link on home page', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('link', { name: /new paste/i })).toBeVisible()
  })

  test('shows Log in link when not authenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('link', { name: /log in/i })).toBeVisible()
  })

  test('New paste link on viewer navigates to home', async ({ page }) => {
    await page.route('/api/v1/pastes/abc12', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'abc12',
          url: 'http://localhost:4173/abc12',
          raw_url: 'http://localhost:4173/api/v1/pastes/abc12/raw',
          content: 'hello',
          language: 'text',
          size_bytes: 5,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
    await page.goto('/abc12')
    await page.getByRole('link', { name: /new paste/i }).first().click()
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('textbox', { name: /content/i })).toBeVisible()
  })
})
