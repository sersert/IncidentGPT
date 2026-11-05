import { test as base, expect } from '@playwright/test'
import { mockDashboardStats, mockIncidentDetail } from '../src/api/mocks'

const incident = {
  ...mockIncidentDetail,
  id: 'INC-REVIEW-1',
  title: 'Review regression incident',
  status: 'active',
  alerts: [],
  assignee: null as string | null,
  analysis: {
    ...mockIncidentDetail.analysis!,
    similar_incidents: [{ id: 'INC-REVIEW-2', similarity: 80, date: '', title: 'Second incident', resolution: '' }],
  },
}
const acknowledged = { ...incident, id: 'INC-REVIEW-2', title: 'Acknowledged incident', status: 'acknowledged' }
const report = {
  report_id: 'report-1', revision: 1, markdown: '# Saved report', generated_at: '2026-01-01T00:00:00Z',
  review_status: 'draft', action_items: [],
  source: { composition_hash: 'fixture', resolution_revision: 0, lifecycle_revision: 0 },
}

type Fixture = {
  requests: string[]
  patches: Record<string, unknown>[]
}

const test = base.extend<{ apiFixture: Fixture }>({
  apiFixture: [async ({ page }, use) => {
    const fixture: Fixture = { requests: [], patches: [] }
    const unexpected: string[] = []
    const errors: string[] = []
    let assignee: string | null = null
    let assignmentAttempts = 0
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/auth-config.json', route => route.fulfill({ json: { mode: 'session' } }))
    await page.route('**/api/v1/**', async route => {
      const request = route.request()
      const url = new URL(request.url())
      const path = url.pathname.replace('/api/v1', '')
      if (path === '/operations/status') {
        await route.fulfill({ status: 404, body: 'disabled' })
        return
      }
      fixture.requests.push(`${request.method()} ${path}${url.search}`)
      let json: unknown
      if (path === '/auth/session') json = { username: 'preview-operator' }
      else if (path === '/dashboard/stats') json = mockDashboardStats
      else if (path === '/dashboard/active') json = [incident]
      else if (path === '/predictions') json = []
      else if (path === '/incidents') {
        const data = [incident, acknowledged].filter(item => !url.searchParams.get('status') || item.status === url.searchParams.get('status'))
        json = { data, total: data.length }
      } else if (/^\/incidents\/INC-REVIEW-[12]$/.test(path)) {
        if (request.method() === 'PATCH') {
          const patch = request.postDataJSON()
          fixture.patches.push(patch)
          if (patch.status || ++assignmentAttempts === 1) {
            await route.fulfill({ status: 503, body: 'Fixture mutation failed' })
            return
          }
          assignee = patch.assignee
        }
        json = path.endsWith('-2') ? acknowledged : { ...incident, assignee }
      } else if (path.endsWith('/analyses')) json = { items: [], jobs: [], enabled: false }
      else if (path.endsWith('/lifecycle')) json = { revision: 0 }
      else if (path.endsWith('/changes')) json = { items: [], status: 'disabled', next_offset: null, coverage: null }
      else if (path.endsWith('/investigations')) json = { diagnostics_enabled: true, operations: [], jobs: [] }
      else if (path === '/services/graph') json = { enabled: false, nodes: [], edges: [], status: 'disabled' }
      else if (path.endsWith('/resolution')) json = { resolution: null }
      else if (path === '/knowledge/search') json = { results: [], runbooks: [], runbook_status: 'disabled' }
      else if (path.endsWith('/postmortem') || path.endsWith('/handoff')) json = { items: [report], jobs: [], enabled: true }
      else {
        unexpected.push(`${request.method()} ${path}`)
        await route.fulfill({ status: 404, body: 'Unexpected fixture request' })
        return
      }
      await route.fulfill({ json })
    })
    await use(fixture)
    expect(unexpected).toEqual([])
    expect(errors).toEqual([])
  }, { auto: true }],
})

test('assignment starts clean after a failed status change and preserves input on failure', async ({ page, apiFixture }) => {
  await page.goto(`/incidents/${incident.id}`)
  await page.getByRole('button', { name: 'Принять в работу', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Изменения не сохранены')
  await page.getByRole('button', { name: 'Назначить', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('alert')).toHaveCount(0)
  await dialog.getByLabel('Кто занимается').fill('preview-operator')
  await dialog.getByRole('button', { name: 'Сохранить', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('Назначение не сохранено')
  await expect(dialog.getByLabel('Кто занимается')).toHaveValue('preview-operator')
  await dialog.getByRole('button', { name: 'Сохранить', exact: true }).click()
  await expect(dialog).not.toBeVisible()
  await expect(page.getByRole('button', { name: 'Назначен: preview-operator' })).toBeVisible()
  expect(apiFixture.patches).toEqual([
    { status: 'acknowledged' }, { assignee: 'preview-operator' }, { assignee: 'preview-operator' },
  ])
})

test('unvisited workspaces make no requests and visited workspaces retain all drafts', async ({ page, apiFixture }) => {
  await page.goto(`/incidents/${incident.id}`)
  await expect(page.getByRole('heading', { name: 'События жизненного цикла' })).toBeVisible()
  expect(apiFixture.requests.some(url => /changes|investigations|graph|resolution|knowledge|postmortem/.test(url))).toBe(false)

  await page.getByRole('tab', { name: 'Расследование', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Обновить контекст', exact: true })).toBeEnabled()
  await page.getByLabel('Вопрос или причина проверки').fill('Investigation draft')
  expect(apiFixture.requests.some(url => url.includes('/investigations'))).toBe(true)
  expect(apiFixture.requests.some(url => /resolution|knowledge|postmortem/.test(url))).toBe(false)

  await page.getByRole('tab', { name: 'Решения и runbooks', exact: true }).click()
  await page.getByRole('button', { name: 'Записать / исправить решение' }).click()
  await page.getByRole('textbox', { name: 'Подтверждённая причина', exact: true }).fill('Resolution draft')
  expect(apiFixture.requests.some(url => url.includes('/postmortem'))).toBe(false)

  await page.getByRole('tab', { name: 'Отчёты и передача', exact: true }).click()
  await page.getByRole('button', { name: 'Редактировать', exact: true }).click()
  await page.getByLabel('Markdown отчёта').fill('# Unsaved report draft')
  await page.getByRole('button', { name: 'Включить тёмную тему' }).click()
  await expect(page.getByLabel('Markdown отчёта')).toHaveValue('# Unsaved report draft')

  await page.getByRole('tab', { name: 'Обзор и анализ', exact: true }).click()
  await page.goBack()
  await expect(page.getByLabel('Markdown отчёта')).toHaveValue('# Unsaved report draft')
  await page.getByRole('tab', { name: 'Расследование', exact: true }).click()
  await expect(page.getByLabel('Вопрос или причина проверки')).toHaveValue('Investigation draft')
  await page.getByRole('tab', { name: 'Решения и runbooks', exact: true }).click()
  await expect(page.getByRole('textbox', { name: 'Подтверждённая причина', exact: true })).toHaveValue('Resolution draft')
  await expect(page.getByRole('tab', { name: 'Решения и runbooks', exact: true })).toHaveAttribute('aria-selected', 'true')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('deep links and keyboard navigation load only visited panels and reset on a different incident', async ({ page, apiFixture }) => {
  await page.goto(`/incidents/${incident.id}?view=reports`)
  await page.getByRole('button', { name: 'Редактировать', exact: true }).click()
  await page.getByLabel('Markdown отчёта').fill('First incident draft')
  expect(apiFixture.requests.some(url => /analyses|lifecycle|changes|investigations|resolution|knowledge/.test(url))).toBe(false)
  await page.getByRole('tab', { name: 'Отчёты и передача', exact: true }).focus()
  await page.keyboard.press('Home')
  await expect(page.getByRole('tab', { name: 'Обзор и анализ', exact: true })).toBeFocused()
  await expect(page.getByRole('heading', { name: 'События жизненного цикла' })).toBeVisible()
  await page.getByRole('link').filter({ hasText: 'INC-REVIEW-2' }).click()
  await expect(page).toHaveURL(`/incidents/${acknowledged.id}`)
  expect(apiFixture.requests.some(url => url.includes(`${acknowledged.id}/postmortem`))).toBe(false)
  await page.getByRole('tab', { name: 'Отчёты и передача', exact: true }).click()
  await page.getByRole('button', { name: 'Редактировать', exact: true }).click()
  await expect(page.getByLabel('Markdown отчёта')).toHaveValue(report.markdown)
})

test('the overview queue link includes acknowledged incidents', async ({ page, apiFixture }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Вся очередь' }).click()
  await expect(page).toHaveURL('/incidents')
  await expect(page.getByRole('heading', { name: acknowledged.title, exact: true })).toBeVisible()
  const request = apiFixture.requests.find(url => url.startsWith('GET /incidents?'))!
  expect(new URL(request.replace('GET ', ''), 'http://fixture').searchParams.has('status')).toBe(false)
})

for (const saved of [null, 'light'] as const) {
  test(`theme initializes before the bundle under script-src self (saved: ${saved})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' })
    if (saved) await page.addInitScript(value => localStorage.setItem('incidentgpt-theme', value), saved)
    await page.route('**/assets/*.js', route => route.abort())
    await page.route('http://127.0.0.1:4173/', async route => {
      const response = await route.fetch()
      await route.fulfill({ response, headers: { ...response.headers(), 'Content-Security-Policy': "script-src 'self'; object-src 'none'; base-uri 'self'" } })
    })
    await page.goto('/')
    await expect(page.locator('html')).toHaveAttribute('data-theme', saved ?? 'dark')
    await expect(page.locator('#root')).toBeEmpty()
  })
}
