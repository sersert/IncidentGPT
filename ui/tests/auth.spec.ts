import { test, expect } from '@playwright/test'
import { mockDashboardStats } from '../src/api/mocks'

test('sign-in protects deep links, reports errors, and logout removes workspace access', async ({
  page,
}) => {
  let signedIn = false
  const workspaceRequests: string[] = []
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname.replace('/api/v1', '')
    if (path === '/auth/session') {
      if (request.method() === 'DELETE') {
        signedIn = false
        return route.fulfill({ status: 204 })
      }
      if (request.method() === 'POST') {
        const credentials = request.postDataJSON()
        signedIn =
          credentials.username === 'operator' &&
          credentials.password === 'correct-password'
      }
      return route.fulfill({
        status: signedIn ? 200 : 401,
        json: signedIn ? { username: 'operator' } : { error: 'unauthorized' },
      })
    }
    workspaceRequests.push(path)
    return route.fulfill({
      json: path === '/dashboard/stats' ? mockDashboardStats : [],
    })
  })
  await page.goto('/help')
  await expect(
    page.getByRole('heading', { name: 'Вход в IncidentGPT' }),
  ).toBeVisible()
  expect(workspaceRequests).toEqual([])
  await page.getByLabel('Логин', { exact: true }).fill('operator')
  await page.getByLabel('Пароль', { exact: true }).fill('incorrect-password')
  await page.getByRole('button', { name: 'Войти', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText(
    'Неверный логин или пароль',
  )
  await page.getByRole('button', { name: 'Показать пароль' }).click()
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveAttribute(
    'type',
    'text',
  )
  await page.getByLabel('Пароль', { exact: true }).fill('correct-password')
  await page.getByRole('button', { name: 'Войти', exact: true }).click()
  await expect(
    page.getByRole('button', { name: 'Выйти', exact: true }),
  ).toBeVisible()
  await expect(page).toHaveURL('/help')
  await page.getByRole('button', { name: 'Выйти', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Вход в IncidentGPT' }),
  ).toBeVisible()
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveValue('')
  await expect(
    page.getByRole('button', { name: 'Выйти', exact: true }),
  ).toHaveCount(0)
  await page.reload()
  await expect(
    page.getByRole('heading', { name: 'Вход в IncidentGPT' }),
  ).toBeVisible()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
})

test('an expired API session returns to sign-in', async ({ page }) => {
  let expired = false
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname.replace('/api/v1', '')
    // Expire only the resource refreshed by this test. Unrelated in-flight
    // operations/prediction requests must not sign out before the click.
    if (expired && path === '/dashboard/stats') return route.fulfill({ status: 401, body: 'session expired' })
    return route.fulfill({
      json:
        path === '/auth/session'
          ? { username: 'operator' }
          : path === '/dashboard/stats'
            ? mockDashboardStats
            : [],
    })
  })
  await Promise.all([
    page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/dashboard/stats' && response.status() === 200),
    page.goto('/'),
  ])
  await expect(
    page.getByRole('button', { name: 'Обновить', exact: true }),
  ).toBeVisible()
  expired = true
  await page.getByRole('button', { name: 'Обновить', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Вход в IncidentGPT' }),
  ).toBeVisible()
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveValue('')
})
