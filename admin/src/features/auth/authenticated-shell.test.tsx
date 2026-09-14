import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthenticatedShell } from './authenticated-shell'
import { useAccessDraftStore } from '@/features/access/drafts'
import { clearProtectedRotaCache } from '@/features/rota/cache'
import { initializeI18n, i18n } from '@/shared/i18n'
import { ThemeProvider } from '@/shared/theme'
import type { ApiClient } from '@/shared/api/client'
import type { User } from '@/shared/api/contracts'

const { navigate } = vi.hoisted(() => ({ navigate: vi.fn() }))

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router')
  return {
    ...actual,
    Link: ({ children, to, ...props }: { children?: React.ReactNode; to?: string } & Record<string, unknown>) => (
      <a href={typeof to === 'string' ? to : '#'} {...props}>{children}</a>
    ),
    useLocation: () => ({ pathname: '/rota' }),
    useNavigate: () => navigate,
  }
})

const user: User = {
  id: '019535d9-3df7-79fb-b466-fa907fa17f91',
  name: 'Account A',
  email: 'a@example.com',
  hasAvatar: false,
  permissions: [],
  superAdmin: false,
}

function apiWithLogout(): ApiClient {
  return {
    getSetupStatus: vi.fn(),
    setup: vi.fn(),
    login: vi.fn(),
    me: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
    requestPasswordReset: vi.fn(),
    completePasswordReset: vi.fn(),
  }
}

describe('AuthenticatedShell navigation and logout boundaries', () => {
  beforeEach(async () => {
    navigate.mockReset()
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
    })) as unknown as typeof window.matchMedia
    await initializeI18n()
    await i18n.changeLanguage('en')
    useAccessDraftStore.getState().clearAll()
  })

  it.each([
    { locale: 'en' as const, home: 'Home', labels: ['Availability', 'Roster', 'Requests', 'Leaves', 'Attendance'], dashboard: 'Dashboard', personalSettings: 'Personal settings' },
    { locale: 'zh-CN' as const, home: '首页', labels: ['可用时间', '排班表', '调班申请', '请假', '考勤'], dashboard: '仪表盘', personalSettings: '个人设置' },
  ])('renders exact translated employee navigation in $locale', async ({ locale, home, labels, dashboard, personalSettings }) => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <AuthenticatedShell api={apiWithLogout()} user={{ ...user, locale }}>
            <div>protected page</div>
          </AuthenticatedShell>
        </QueryClientProvider>
      </ThemeProvider>,
    )
    expect(await screen.findByRole('link', { name: home })).toHaveAttribute('href', '/')
    const paths = ['/rota/availability', '/rota/roster', '/rota/requests', '/rota/leaves', '/rota/attendance']
    for (const [index, label] of labels.entries()) {
      expect(await screen.findByRole('link', { name: label })).toHaveAttribute('href', paths[index])
    }
    const navigation = screen.getByRole('navigation')
    expect(navigation).not.toHaveTextContent(dashboard)
    expect(navigation).not.toHaveTextContent(personalSettings)
    expect(navigation).not.toHaveTextContent(/returned an object|instead of string/)
  })

  it.each([
    { locale: 'en' as const, menuLabel: 'Menu', personalSettings: 'Personal settings' },
    { locale: 'zh-CN' as const, menuLabel: '菜单', personalSettings: '个人设置' },
  ])('keeps personal settings keyboard-accessible in the avatar menu in $locale', async ({ locale, menuLabel, personalSettings }) => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <AuthenticatedShell api={apiWithLogout()} user={{ ...user, locale }}>
            <div>protected page</div>
          </AuthenticatedShell>
        </QueryClientProvider>
      </ThemeProvider>,
    )

    const actor = userEvent.setup()
    await actor.click(screen.getByRole('button', { name: `Account A, ${menuLabel}` }))
    const settingsItem = screen.getByRole('menuitem', { name: personalSettings })
    expect(settingsItem).toHaveAttribute('href', '/personal-settings')
    await settingsItem.focus()
    await actor.keyboard('{Enter}')
    expect(screen.queryByRole('menuitem', { name: personalSettings })).toBeNull()
  })

  it('removes protected Rota data before navigating to login after logout', async () => {
    const api = apiWithLogout()
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    queryClient.setQueryData(['leaves', 'pool', 'pending', 1, 20], { owner: user.id })
    queryClient.setQueryData(['rota', 'auth', 'me'], { owner: user.id })
    useAccessDraftStore.getState().setOwner(user.id)

    render(
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <AuthenticatedShell api={api} user={user}>
            <div>protected page</div>
          </AuthenticatedShell>
        </QueryClientProvider>
      </ThemeProvider>,
    )

    expect(screen.queryByRole('link', { name: 'Dashboard' })).toBeNull()
    expect(screen.queryByRole('link', { name: 'Personal settings' })).toBeNull()
    expect(screen.getByRole('link', { name: 'Availability' })).toHaveAttribute('href', '/rota/availability')
    expect(screen.getByRole('link', { name: 'Roster' })).toHaveAttribute('href', '/rota/roster')
    expect(screen.getByRole('link', { name: 'Requests' })).toHaveAttribute('href', '/rota/requests')
    expect(screen.getByRole('link', { name: 'Leaves' })).toHaveAttribute('href', '/rota/leaves')
    expect(screen.getByRole('link', { name: 'Attendance' })).toHaveAttribute('href', '/rota/attendance')
    expect(screen.queryByRole('link', { name: /users/i })).toBeNull()
    expect(screen.queryByRole('link', { name: /roles/i })).toBeNull()

    const actor = userEvent.setup()
    await actor.click(screen.getByRole('button', { name: /Account A/ }))
    await actor.click(screen.getByRole('menuitem', { name: /log out/i }))

    await waitFor(() => expect(api.logout).toHaveBeenCalledOnce())
    await waitFor(() => expect(queryClient.getQueryData(['leaves', 'pool', 'pending', 1, 20])).toBeUndefined())
    expect(queryClient.getQueryData(['rota', 'auth', 'me'])).toBeUndefined()
    expect(navigate).toHaveBeenCalledWith({ to: '/login', replace: true })

    await clearProtectedRotaCache(queryClient)
  })
})
