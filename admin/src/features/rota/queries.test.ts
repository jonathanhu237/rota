import { QueryClient } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '@/test/msw'
import { createApiClient } from '@/shared/api/client'
import { currentUserQueryOptions, toRotaUser } from './queries'

describe('Rota current user capability mapping', () => {
  it.each([
    { permission: 'rota.read', canRead: true },
    { permission: 'rota.self', canRead: false },
  ])('maps $permission without granting management authority', async ({ permission, canRead }) => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    server.use(
      http.get('/api/auth/me', () =>
        HttpResponse.json({
          user: {
            id: '019535d9-3df7-79fb-b466-fa907fa17f9e',
            email: 'reader@example.com',
            name: 'Reader',
            locale: 'en',
          },
          roles: [],
          permissions: [permission],
          superAdmin: false,
        }),
      ),
    )

    await expect(
      queryClient.fetchQuery(currentUserQueryOptions(createApiClient())).then(toRotaUser),
    ).resolves.toMatchObject({
      is_admin: canRead,
      can_manage: false,
    })
  })

  it('treats Super Admin as a manager even without explicit Rota grants', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    server.use(
      http.get('/api/auth/me', () =>
        HttpResponse.json({
          user: {
            id: '019535d9-3df7-79fb-b466-fa907fa17f90',
            email: 'admin@example.com',
            name: 'Admin',
            locale: 'zh-CN',
          },
          roles: [],
          permissions: [],
          superAdmin: true,
        }),
      ),
    )

    await expect(
      queryClient.fetchQuery(currentUserQueryOptions(createApiClient())).then(toRotaUser),
    ).resolves.toMatchObject({
      is_admin: true,
      can_manage: true,
      language_preference: 'zh',
    })
  })
})
