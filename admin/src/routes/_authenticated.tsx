import { createFileRoute, isRedirect, Outlet, redirect } from '@tanstack/react-router'
import { SessionMonitor } from '@/features/auth/session-monitor'
import { AuthenticatedShell } from '@/features/auth/authenticated-shell'
import { SessionError } from '@/features/auth/session-error'
import { currentUserOptions, currentUserQueryKey } from '@/features/auth/queries'
import type { User } from '@/shared/api/contracts'
import { isUnauthenticated } from '@/shared/api/problems'
import { clearAccessDrafts } from '@/features/access/drafts'
import { restoreGuestLocale } from '@/shared/i18n'
import { clearProtectedRotaCache } from '@/features/rota/cache'

export const Route = createFileRoute('/_authenticated')({
  loader: async ({ context }) => {
    try {
      const previousUser = context.queryClient.getQueryData<User>(currentUserQueryKey)
      // A route transition can discover a different account only after the
      // authoritative principal request returns. Clear protected reads before
      // every principal lookup so persisted or orphaned data cannot survive a
      // delayed, failed, or account-changing transition.
      await clearProtectedRotaCache(context.queryClient)
      const user = await context.queryClient.fetchQuery(currentUserOptions(context.api))
      if (previousUser && principalChanged(previousUser, user)) {
        await clearProtectedRotaCache(context.queryClient)
      }
      return { user }
    } catch (error) {
      if (isUnauthenticated(error)) {
        clearAccessDrafts()
        void restoreGuestLocale()
        context.queryClient.removeQueries({ queryKey: currentUserQueryKey })
        context.queryClient.removeQueries({ queryKey: ['access'] })
        context.queryClient.removeQueries({ queryKey: ['personal-profile'] })
        context.queryClient.removeQueries({ queryKey: ['operational-warnings'] })
        context.queryClient.removeQueries({ queryKey: ['operation-log-status'] })
        context.queryClient.removeQueries({ queryKey: ['operation-logs'] })
        context.queryClient.removeQueries({ queryKey: ['operation-log'] })
        context.queryClient.removeQueries({ queryKey: ['settings', 'operation-log-retention'] })
        await clearProtectedRotaCache(context.queryClient)
        throw redirect({ to: '/login', replace: true })
      }
      if (isRedirect(error)) throw error
      throw error
    }
  },
  errorComponent: ({ error }) => <SessionError error={error} />,
  component: AuthenticatedRoute,
})

function principalChanged(previous: User, current: User): boolean {
  if (previous.id !== current.id || previous.superAdmin !== current.superAdmin) return true
  const previousPermissions = [...(previous.permissions ?? [])].sort().join("\u0000")
  const currentPermissions = [...(current.permissions ?? [])].sort().join("\u0000")
  return previousPermissions !== currentPermissions
}

function AuthenticatedRoute() {
  const { user } = Route.useLoaderData()
  const { api } = Route.useRouteContext()
  return <AuthenticatedShell api={api} user={user}><SessionMonitor api={api} userID={user.id} /><Outlet /></AuthenticatedShell>
}
