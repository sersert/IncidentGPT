import { expect, test } from '@playwright/test'

const snapshot = (enabled = true, revision = 1) => ({
  checked_at: new Date().toISOString(),
  control_available: true,
  control: { enabled, revision, updated_at: new Date().toISOString(), updated_by: 'operator' },
  worker: { managed: true, control_known: true, control: { enabled, revision }, active_provider_requests: 0 },
  components: [
    { id: 'model', name: 'Модель LLM', status: 'healthy', detail: 'Получен корректный ответ на реальный запрос', checked_at: new Date().toISOString(), latency_ms: 50 },
    { id: 'redis', name: 'Redis', status: 'unhealthy', detail: 'Redis не отвечает', checked_at: new Date().toISOString(), latency_ms: 0 },
    { id: 'mcp', name: 'База знаний MCP', status: 'not_configured', detail: 'Не настроен', checked_at: new Date().toISOString(), latency_ms: 0 },
  ],
})

// Workspace is behind AuthGate (main): answer the session probe so /operations renders.
test.beforeEach(async ({ page }) => {
  await page.route('**/auth-config.json', route => route.fulfill({ json: { mode: 'session' } }))
  await page.route('**/api/v1/auth/session', route => route.fulfill({ json: { username: 'operator' } }))
})

test('mobile monitoring, persisted toggle and explicit paid probe', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  let state = snapshot()
  let paidCalls = 0
  await page.route('**/api/v1/operations/**', async route => {
    const request = route.request()
    if (request.url().endsWith('/status')) return route.fulfill({ json: state })
    if (request.url().endsWith('/ai-control')) {
      const body = request.postDataJSON()
      expect(request.method()).toBe('PUT')
      expect(body.revision).toBe(state.control.revision)
      state = snapshot(body.enabled, body.revision + 1)
      return route.fulfill({ json: state.control })
    }
    paidCalls++
    return route.fulfill({ json: { success: true, latency_ms: 12 } })
  })
  await page.goto('/operations')
  await expect(page.getByRole('heading', { name: 'Состояние системы' })).toBeVisible()
  await expect(page.getByRole('article').filter({ hasText: 'Модель LLM' })).toContainText('Доступен')
  await expect(page.getByRole('article').filter({ hasText: 'Redis' })).toContainText('Ошибка')
  await page.getByRole('button', { name: 'Обновить', exact: true }).click()
  expect(paidCalls).toBe(0)
  await page.getByRole('button', { name: 'Выключить AI', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Включить AI', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: /Проверить модель/ })).toBeDisabled()
  await page.reload()
  await expect(page.getByText('Запрошена пауза обращений к модели.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Включить AI', exact: true }).click()
  await expect(page.getByRole('button', { name: /Проверить модель/ })).toBeEnabled()
  await page.getByRole('button', { name: /Проверить модель/ }).click()
  await expect(page.getByRole('status')).toContainText('Модель ответила за 12 мс')
  expect(paidCalls).toBe(1)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
})

test('failed refresh and stale result cannot show availability or permit mutation', async ({ page }) => {
  let failed = false
  const state = snapshot()
  state.components[0].checked_at = new Date(Date.now() - 360_000).toISOString()
  await page.route('**/api/v1/operations/status', route => failed
    ? route.fulfill({ status: 503, body: 'unavailable' })
    : route.fulfill({ json: state }))
  await page.goto('/operations')
  await expect(page.getByRole('article').filter({ hasText: 'Модель LLM' })).toContainText('Не проверен')
  failed = true
  await page.getByRole('button', { name: 'Обновить', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Обновление не удалось')
  await expect(page.getByRole('button', { name: 'Выключить AI', exact: true })).toBeDisabled()
  await expect(page.getByRole('article').filter({ hasText: 'Redis' })).toContainText('Не проверен')
})

test('rejected mutation shows error and retains confirmed state', async ({ page }) => {
  await page.route('**/api/v1/operations/status', route => route.fulfill({ json: snapshot() }))
  await page.route('**/api/v1/operations/ai-control', route => route.fulfill({ status: 409, body: 'revision conflict' }))
  await page.goto('/operations')
  await page.getByRole('button', { name: 'Выключить AI', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Изменение не сохранено')
  await expect(page.getByRole('button', { name: 'Выключить AI', exact: true })).toBeEnabled()
})

test('disabled feature and denied credentials do not display healthy lamps', async ({ page }) => {
  await page.route('**/api/v1/operations/status', route => route.fulfill({ status: 404, body: 'disabled' }))
  await page.goto('/operations')
  await expect(page.getByRole('alert')).toContainText('Мониторинг ещё не включён')
  await expect(page.getByRole('article')).toHaveCount(0)
  await page.route('**/api/v1/operations/status', route => route.fulfill({ status: 401, body: 'unauthorized' }))
  await page.getByRole('button', { name: 'Обновить', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Вход в IncidentGPT' })).toBeVisible()
  await expect(page.getByRole('article')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Выключить AI', exact: true })).toHaveCount(0)
})

test('operator can request pause before an unavailable worker recovers', async ({ page }) => {
  let state = snapshot()
  state.worker.managed = false
  state.worker.control_known = false
  await page.route('**/api/v1/operations/**', async route => {
    if (route.request().url().endsWith('/status')) return route.fulfill({ json: state })
    state = { ...state, control: { ...state.control, enabled: false, revision: 2 } }
    return route.fulfill({ json: state.control })
  })
  await page.goto('/operations')
  await page.getByRole('button', { name: 'Выключить AI', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Включить AI', exact: true })).toBeDisabled()
  await expect(page.getByText(/применение состояния пока не подтверждено/)).toBeVisible()
})
