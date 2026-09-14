import { describe, expect, it, vi } from 'vitest'
import { createAppQueryClient } from '@/app/query-client'
import { currentUserQueryKey } from '@/features/auth/queries'
import type { User } from '@/shared/api/contracts'
import { Route } from './_authenticated'

const userA: User = {
  id: '019535d9-3df7-79fb-b466-fa907fa17f91',
  name: 'Account A',
  email: 'a@example.com',
  hasAvatar: false,
  permissions: ['rota.read'],
  superAdmin: false,
}

const userB: User = {
  id: '019535d9-3df7-79fb-b466-fa907fa17f90',
  name: 'Account B',
  email: 'b@example.com',
  hasAvatar: false,
  permissions: [],
  superAdmin: false,
}

const loader = Route.options.loader as unknown as (input: { context: { queryClient: ReturnType<typeof createAppQueryClient>; api: { me: () => Promise<User> } } }) => Promise<{ user: User }>

function seedPrivateData(queryClient: ReturnType<typeof createAppQueryClient>, value: string) {
  queryClient.setQueryData(['leaves', 'pool', 'pending', 1, 20], { owner: value })
  queryClient.setQueryData(['roster', 'current'], { owner: value })
  queryClient.setQueryData(['auth', 'current-user'], userA)
}

describe('authenticated Rota cache boundaries', () => {
  it('clears A data before a delayed B principal lookup and after permission changes', async () => {
    const queryClient = createAppQueryClient()
    seedPrivateData(queryClient, 'A')
    let resolvePrincipal!: (value: User) => void
    const api = { me: vi.fn(() => new Promise<User>((resolve) => { resolvePrincipal = resolve })) }
    const transition = loader({ context: { queryClient, api } })

    await vi.waitFor(() => expect(api.me).toHaveBeenCalledOnce())
    expect(queryClient.getQueryData(['leaves', 'pool', 'pending', 1, 20])).toBeUndefined()
    expect(queryClient.getQueryData(['roster', 'current'])).toBeUndefined()

    resolvePrincipal(userB)
    await expect(transition).resolves.toEqual({ user: userB })

    // A same-UUID role/permission revocation is also an ownership boundary;
    // the loader must not preserve management reads while replacing the principal.
    queryClient.setQueryData(currentUserQueryKey, userA)
    seedPrivateData(queryClient, 'A-after-role-change')
    const changed = loader({ context: { queryClient, api: { me: vi.fn().mockResolvedValue({ ...userA, permissions: [] }) } } })
    await expect(changed).resolves.toEqual({ user: { ...userA, permissions: [] } })
    expect(queryClient.getQueryData(['roster', 'current'])).toBeUndefined()
  })

  it('does not retain A data when B principal refresh fails or session expires', async () => {
    const queryClient = createAppQueryClient()
    seedPrivateData(queryClient, 'A')
    await expect(loader({ context: { queryClient, api: { me: vi.fn().mockRejectedValue(new Error('B unavailable')) } } })).rejects.toThrow('B unavailable')
    expect(queryClient.getQueryData(['leaves', 'pool', 'pending', 1, 20])).toBeUndefined()

  })
})
