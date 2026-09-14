import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { ApiProblemError } from '@/shared/api/client'
import { clearAccessDrafts, useAccessDraftStore } from '@/features/access/drafts'
import { server } from '@/test/msw'
import { leavePoolQueryOptions } from '@/features/rota/queries'
import { createAssignment } from '@/features/rota/queries'
import { RotaApiError } from '@/features/rota/api'
import { createAppQueryClient } from './query-client'

describe('application query client', () => {
  it('clears access drafts and caches when a session expires', async () => {
    const queryClient = createAppQueryClient()
    useAccessDraftStore.getState().setOwner('019535d9-3df7-79fb-b466-fa907fa17f91')
    useAccessDraftStore.getState().setRoleCreate({ name: 'Auditor', description: '', permissions: [], submitting: false, conflict: false })
    queryClient.setQueryData(['access', 'users'], { users: ['stale'] })

    const error = new ApiProblemError({ type: '/problems/unauthenticated', title: 'expired', status: 401, code: 'unauthenticated' })
    await expect(queryClient.fetchQuery({ queryKey: ['access', 'expired'], queryFn: async () => { throw error } })).rejects.toBe(error)

    expect(queryClient.getQueryData(['access', 'users'])).toBeUndefined()
    expect(useAccessDraftStore.getState().ownerID).toBeUndefined()
    expect(useAccessDraftStore.getState().roleCreate).toBeUndefined()
    clearAccessDrafts()
  })

  it('clears private Rota data when the actual Rota query adapter returns 401', async () => {
    const queryClient = createAppQueryClient()
    queryClient.setQueryData(['leaves', 'pool', 'pending', 1, 20], { leaves: ['account-a'] })
    queryClient.setQueryData(['publications', 'detail', 7], { name: 'A only' })
    server.use(
      http.get('/api/rota/leaves/pool', () =>
        HttpResponse.json(
          { error: { code: 'UNAUTHENTICATED', message: 'Authentication required' } },
          { status: 401 },
        ),
      ),
    )

    await expect(queryClient.fetchQuery(leavePoolQueryOptions('pending', 1, 20))).rejects.toMatchObject({
      status: 401,
      code: 'UNAUTHENTICATED',
    })

    expect(queryClient.getQueryData(['leaves', 'pool', 'pending', 1, 20])).toBeUndefined()
    expect(queryClient.getQueryData(['publications', 'detail', 7])).toBeUndefined()
  })

  it('clears private Rota data after a write 401 but retains it for 403 and network failures', async () => {
    const queryClient = createAppQueryClient()
    const input = {
      user_id: '019535d9-3df7-79fb-b466-fa907fa17f90',
      slot_id: 11,
      weekday: 1,
      position_id: 3,
    }
    queryClient.setQueryData(['roster', 'current'], { publication: { name: 'A only' } })
    server.use(
      http.post('/api/rota/publications/7/assignments', () =>
        HttpResponse.json(
          { error: { code: 'UNAUTHENTICATED', message: 'Authentication required' } },
          { status: 401 },
        ),
      ),
    )
    const unauthenticatedMutation = queryClient.getMutationCache().build(queryClient, {
      mutationFn: () => createAssignment(7, input),
    })
    await expect(unauthenticatedMutation.execute(undefined)).rejects.toBeInstanceOf(RotaApiError)
    expect(queryClient.getQueryData(['roster', 'current'])).toBeUndefined()

    queryClient.setQueryData(['roster', 'current'], { publication: { name: 'still private' } })
    server.use(
      http.post('/api/rota/publications/7/assignments', () =>
        HttpResponse.json(
          { error: { code: 'FORBIDDEN', message: 'Forbidden' } },
          { status: 403 },
        ),
      ),
    )
    const forbiddenMutation = queryClient.getMutationCache().build(queryClient, {
      mutationFn: () => createAssignment(7, input),
    })
    await expect(forbiddenMutation.execute(undefined)).rejects.toMatchObject({ status: 403 })
    expect(queryClient.getQueryData(['roster', 'current'])).toEqual({ publication: { name: 'still private' } })

    server.use(
      http.post('/api/rota/publications/7/assignments', () => HttpResponse.error()),
    )
    const networkMutation = queryClient.getMutationCache().build(queryClient, {
      mutationFn: () => createAssignment(7, input),
    })
    await expect(networkMutation.execute(undefined)).rejects.toBeInstanceOf(Error)
    expect(queryClient.getQueryData(['roster', 'current'])).toEqual({ publication: { name: 'still private' } })
  })
})
