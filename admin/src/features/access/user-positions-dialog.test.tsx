import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { initializeI18n, i18n } from '@/shared/i18n'
import { server } from '@/test/msw'
import type { AccessUser } from './access-components'
import { UserPositionsDialog } from './user-positions-dialog'

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const user: AccessUser = {
  id: '019535d9-3df7-79fb-b466-fa907fa17f9e',
  name: 'Ada',
  email: 'ada@example.com',
  createdAt: '2026-01-01T00:00:00Z',
  authVersion: 1,
  disabled: false,
  roles: [],
}

describe('UserPositionsDialog', () => {
  beforeEach(async () => {
    await initializeI18n()
    await i18n.changeLanguage('en')
  })

  it('loads, edits, and saves a user qualification set', async () => {
    const replace = vi.fn()
    server.use(
      http.get('/api/rota/positions', () =>
        HttpResponse.json({ positions: [
          { id: 1, name: 'Nurse', description: '' },
          { id: 2, name: 'Doctor', description: '' },
        ] }),
      ),
      http.get(`/api/rota/users/${user.id}/positions`, () =>
        HttpResponse.json({ positions: [{ id: 1, name: 'Nurse', description: '' }] }),
      ),
      http.put(`/api/rota/users/${user.id}/positions`, async ({ request }) => {
        replace(await request.json())
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const close = vi.fn()
    render(
      <QueryClientProvider client={queryClient}>
        <UserPositionsDialog user={user} open canManage onClose={close} />
      </QueryClientProvider>,
    )

    const dialog = await screen.findByRole('dialog', { name: 'Qualifications' })
    const actor = userEvent.setup()
    const doctor = await within(dialog).findByRole('checkbox', { name: 'Doctor' })
    expect(within(dialog).getByRole('checkbox', { name: 'Nurse' })).toBeChecked()
    expect(doctor).not.toBeChecked()
    await actor.click(doctor)
    await actor.click(within(dialog).getByRole('button', { name: 'Save Qualifications' }))

    await waitFor(() => expect(replace).toHaveBeenCalledWith({ position_ids: [1, 2] }))
    expect(close).toHaveBeenCalledOnce()
  })

  it('does not render or fetch for a read-only viewer', () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const close = vi.fn()
    render(
      <QueryClientProvider client={queryClient}>
        <UserPositionsDialog user={user} open={false} canManage={false} onClose={close} />
      </QueryClientProvider>,
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
