import { expect, test, type Page } from '@playwright/test'

// scripts/test-business-browser.sh provisions these identities and assignments
// only in its disposable database. No real-user installation is supported.
const enabled = process.env.E2E_ROTA_BUSINESS === '1'
const adminEmail = 'admin@example.com'
const employeeA = 'employee-a@example.com'
const employeeB = 'employee-b@example.com'
const employeeBID = '019535d9-3df7-79fb-b466-fa907fa17f92'
const organization = '验收组织 & Operations'
const privateReason = 'Private employee A browser acceptance reason'
let leaveID: number

async function login(page: Page, email: string) {
  await page.goto('/login')
  await page.getByLabel(/^(Email|邮箱)$/).fill(email)
  await page.getByLabel(/^(Password|密码)$/, { exact: true }).fill('Admin1!x')
  await page.getByRole('button', { name: /^(Sign in|登录)$/, exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

async function api(page: Page, path: string, method = 'GET', data?: unknown, status = 200) {
  const response = await page.request.fetch(path, {
    method, data, headers: { Origin: new URL(page.url()).origin },
  })
  const body = await response.text()
  const result = body ? JSON.parse(body) : undefined
  expect(response.status(), `${method} ${path}: ${result?.error?.code ?? result?.code ?? ''}`).toBe(status)
  return result
}

async function logout(page: Page, name: string) {
  await page.getByRole('button', { name: new RegExp(`^${name},`) }).click()
  await page.getByRole('menuitem', { name: /^(Log out|退出登录|退出)$/ }).click()
  await expect(page).toHaveURL(/\/login$/)
}

async function messages(page: Page) {
  const origin = process.env.E2E_MAILPIT_API_URL
  if (!origin) throw new Error('Mailpit is required for business acceptance')
  const response = await page.request.get(`${origin}/api/v1/messages?limit=100`)
  expect(response.ok()).toBe(true)
  const summaries = (await response.json()).messages as { ID: string }[]
  return Promise.all(summaries.map(async ({ ID }) => {
    const detail = await page.request.get(`${origin}/api/v1/message/${ID}`)
    expect(detail.ok()).toBe(true)
    return detail.json() as Promise<{ Text: string; HTML: string; To: { Address: string }[] }>
  }))
}

test.use({ trace: 'off', screenshot: 'off', video: 'off' })

test.describe.serial('Rota migrated business acceptance', () => {
  test.skip(!enabled, 'Use scripts/test-business-browser.sh with a disposable installation.')
  test.setTimeout(120_000)

  test('organization identity persists in the template settings with revision and validation guards', async ({ page }) => {
    await login(page, adminEmail)
    await page.goto('/settings')
    const section = page.locator('section[aria-labelledby="identity-settings-title"]')
    await section.getByLabel('System name', { exact: true }).fill('Rota acceptance')
    await section.getByLabel('Organization name', { exact: true }).fill(organization)
    const save = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/settings/system-identity'))
    await section.getByRole('button', { name: 'Save', exact: true }).click()
    expect((await save).status()).toBe(200)
    await page.reload()
    await expect(section.getByLabel('Organization name', { exact: true })).toHaveValue(organization)
    const saved = await api(page, '/api/settings/system-identity')
    expect(saved.organizationName).toBe(organization)
    const publicIdentity = await api(page, '/api/public/system-identity')
    expect(publicIdentity.organizationName).toBe(organization)
    const stale = await page.request.put('/api/settings/system-identity', {
      headers: { Origin: new URL(page.url()).origin },
      multipart: { systemName: 'stale', organizationName: organization, iconAction: 'preserve', revision: String(saved.revision - 1) },
    })
    expect(stale.status()).toBe(409)
    await section.getByLabel('Organization name', { exact: true }).fill('x'.repeat(101))
    await expect(section.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()
    await page.reload()
    const settings = await api(page, '/api/settings/email')
    await api(page, '/api/settings/email', 'PUT', {
      host: 'mailpit', port: 1025, security: 'none', username: '', clearPassword: true,
      fromAddress: 'rota@example.com', fromName: 'Rota acceptance', defaultLocale: 'en',
      autoRetryCount: 0, retentionDays: 7, revision: settings.revision,
    })
    await api(page, '/api/auth/me/preferences', 'PUT', { locale: 'zh-CN' })
    await page.reload()
    await expect(section.getByLabel('组织名称', { exact: true })).toHaveValue(organization)
    await api(page, '/api/auth/me/preferences', 'PUT', { locale: 'en' })
  })

  test('no-grant employees reach all self-service routes; real leave/shift mails open the new routes', async ({ page, browser }, testInfo) => {
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await login(page, employeeA)
    const principal = await api(page, '/api/auth/me')
    expect(principal.permissions).toEqual(['rota.self'])
    expect(principal.permissions).not.toContain('rota.read')
    expect(principal.permissions).not.toContain('rota.manage')
    for (const path of ['/rota', '/rota/availability', '/rota/roster', '/rota/requests', '/rota/leaves', '/rota/attendance']) {
      await page.getByRole('navigation').locator(`a[href="${path}"]`).click()
      await expect(page).toHaveURL(new RegExp(`${path}$`))
      await expect(page.locator('main')).not.toContainText(/Page not found|Not Found|页面未找到/)
    }
    for (const path of ['/positions', '/templates', '/publications', '/publications/9001/attendance']) {
      await api(page, `/api/rota${path}`, 'GET', undefined, 403)
    }
    await api(page, '/api/rota/positions', 'POST', { name: 'denied' }, 403)
    await page.goto('/rota/publications')
    await expect(page).toHaveURL(/\/$/)
    const from = new Date().toISOString().slice(0, 10)
    const to = new Date(Date.now() + 6 * 86400_000).toISOString().slice(0, 10)
    const preview = await api(page, `/api/rota/users/me/leaves/preview?from=${from}&to=${to}`)
    const occurrence = preview.occurrences.find((item: { assignment_id: number }) => item.assignment_id === 9001)
    expect(occurrence).toBeDefined()
    const input = { assignment_id: 9001, occurrence_date: occurrence.occurrence_date, type: 'give_direct', counterpart_user_id: employeeBID, category: 'personal', reason: privateReason }
    const created = await api(page, '/api/rota/leaves', 'POST', input, 201)
    leaveID = created.leave.id
    await api(page, '/api/rota/leaves', 'POST', input, 409)
    const unchanged = await api(page, '/api/rota/roster/current')
    expect(JSON.stringify(unchanged)).toContain('Employee A')
    await page.goto(`/rota/leaves/${leaveID}`)
    await expect(page.getByText(privateReason, { exact: true })).toBeVisible()

    const secondContext = await browser.newContext({ baseURL: testInfo.project.use.baseURL })
    try {
      const second = await secondContext.newPage()
      await login(second, employeeB)
      const secondPreview = await api(second, `/api/rota/users/me/leaves/preview?from=${from}&to=${to}`)
      const secondOccurrence = secondPreview.occurrences.find((item: { assignment_id: number }) => item.assignment_id === 9002)
      // Baseline direct shift changes are pre-activation; active publications
      // use the leave workflow. Preserve that rejection instead of weakening it.
      const rejected = await api(second, '/api/rota/publications/9001/shift-changes', 'POST', {
        type: 'give_direct', requester_assignment_id: 9002, occurrence_date: secondOccurrence.occurrence_date,
        counterpart_user_id: principal.user.id,
      }, 409)
      expect(rejected.error.code).toBe('PUBLICATION_NOT_PUBLISHED')
      await expect.poll(async () => {
        const delivered = await messages(page)
        return delivered.filter(message => message.Text.includes(organization)).length
      }, { timeout: 30_000 }).toBeGreaterThanOrEqual(1)
      const delivered = await messages(page)
      const chinese = delivered.find(message => message.To.some(to => to.Address === employeeB) && message.Text.includes(`/rota/leaves/${leaveID}`))
      expect(chinese?.Text).toContain(organization)
      expect(chinese?.Text).toMatch(/[\u4e00-\u9fff]/)
      for (const [target, message, route] of [[second, chinese, `/rota/leaves/${leaveID}`]] as const) {
        expect(message).toBeDefined()
        const link = message!.Text.match(/https?:\/\/[^\s<>"']+\/rota\/(?:leaves\/\d+|requests)/)?.[0]
        expect(link).toBeDefined()
        await target.goto(link!)
        await expect(target).toHaveURL(new RegExp(`${route}$`))
        await expect(target.locator('main')).not.toContainText(/Page not found|Not Found|页面未找到/)
      }
    } finally { await secondContext.close() }
    expect(errors).toEqual([])
  })

  test('logout then another account with failed private refetch never renders the previous employee data', async ({ page }) => {
    await login(page, employeeA)
    await page.goto(`/rota/leaves/${leaveID}`)
    await expect(page.getByText(privateReason, { exact: true })).toBeVisible()
    await logout(page, 'Employee A')
    await login(page, employeeB)
    // B normally may see this direct request, but a failed B detail read must
    // not reuse any data loaded under A's previous session in the SPA.
    await page.route(`**/api/rota/leaves/${leaveID}`, route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'INTERNAL_ERROR' } }) }))
    await page.getByRole('navigation').locator('a[href="/rota/leaves"]').click()
    const failedRead = page.waitForResponse(response => response.url().endsWith(`/api/rota/leaves/${leaveID}`) && response.status() === 503)
    await page.locator(`a[href="/rota/leaves/${leaveID}"]`).first().click()
    await failedRead
    await expect(page).toHaveURL(new RegExp(`/rota/leaves/${leaveID}$`))
    await expect(page.getByText(privateReason, { exact: true })).toHaveCount(0)
    await expect(page.getByRole('navigation')).not.toContainText('employee-a@example.com')
  })

  test('employee role submits only its own qualified availability without management access', async ({ page }) => {
    await login(page, adminEmail)
    await api(page, '/api/rota/publications/9001/end', 'POST')
    const now = Date.now()
    const publication = await api(page, '/api/rota/publications', 'POST', {
      template_id: 9001, name: 'Availability acceptance',
      submission_start_at: new Date(now - 3600_000).toISOString(),
      submission_end_at: new Date(now + 30_000).toISOString(),
      planned_active_from: new Date(now + 7200_000).toISOString(),
      planned_active_until: new Date(now + 3 * 86400_000).toISOString(),
    }, 201)
    const id = publication.publication.id
    await logout(page, 'Browser Admin')
    await login(page, employeeA)
    await api(page, `/api/rota/publications/${id}/submissions`, 'POST', { slot_id: 9001, weekday: 1 }, 201)
    const own = await api(page, `/api/rota/publications/${id}/submissions/me`)
    expect(JSON.stringify(own)).toContain('9001')
    await api(page, `/api/rota/publications/${id}/submissions`, 'POST', { slot_id: 9001, weekday: 0 }, 400)
    await api(page, `/api/rota/publications/${id}/availability-submissions/${employeeBID}`, 'PUT', { cells: [] }, 403)
    await page.goto('/rota/availability')
    await expect(page.locator('main')).toContainText('Availability acceptance')
    await api(page, `/api/rota/publications/${id}/submissions/9001/1`, 'DELETE', undefined, 204)
    // Exercise the real timed collection -> assignment transition rather than
    // changing application clocks or bypassing the service's lifecycle checks.
    await expect.poll(async () => (await api(page, '/api/rota/publications/current')).publication.state, { timeout: 40_000 }).toBe('ASSIGNING')
    await logout(page, 'Employee A')
    await login(page, adminEmail)
    const occurrence = new Date(now + 2 * 86400_000)
    await api(page, `/api/rota/publications/${id}/assignments`, 'POST', {
      user_id: employeeBID, slot_id: 9002, weekday: occurrence.getUTCDay() || 7, position_id: 9001,
    }, 201)
    const board = await api(page, `/api/rota/publications/${id}/assignment-board`)
    const assigned = board.slots.flatMap((slot: { positions: { assignments: { assignment_id: number; user_id: string }[] }[] }) => slot.positions.flatMap(position => position.assignments)).find((assignment: { user_id: string }) => assignment.user_id === employeeBID)
    expect(assigned).toBeDefined()
    await api(page, `/api/rota/publications/${id}/publish`, 'POST')
    await logout(page, 'Browser Admin')
    await login(page, employeeB)
    await api(page, `/api/rota/publications/${id}/shift-changes`, 'POST', {
      type: 'give_direct', requester_assignment_id: assigned.assignment_id,
      occurrence_date: occurrence.toISOString().slice(0, 10),
      counterpart_user_id: '019535d9-3df7-79fb-b466-fa907fa17f91',
    }, 201)
    await expect.poll(async () => (await messages(page)).some(message => message.To.some(to => to.Address === employeeA) && message.Text.includes('/rota/requests')), { timeout: 30_000 }).toBe(true)
    const english = (await messages(page)).find(message => message.To.some(to => to.Address === employeeA) && message.Text.includes('/rota/requests'))!
    expect(english.Text).toContain(organization)
    expect(english.Text).toMatch(/Hi|Hello/)
    const link = english.Text.match(/https?:\/\/[^\s<>"']+\/rota\/requests/)?.[0]
    expect(link).toBeDefined()
    await logout(page, 'Employee B')
    await login(page, employeeA)
    await page.goto(link!)
    await expect(page).toHaveURL(/\/rota\/requests$/)
    await expect(page.locator('main')).not.toContainText(/Page not found|Not Found|页面未找到/)
  })
})
