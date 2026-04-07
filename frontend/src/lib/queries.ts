import { keepPreviousData, queryOptions } from "@tanstack/react-query"
import { isAxiosError } from "axios"

import api from "./axios"
import type { Pagination, User, UserStatus } from "./types"

export type UsersResponse = {
  users: User[]
  pagination: Pagination
}

export type UserResponse = {
  user: User
}

export type CreateUserInput = {
  email: string
  name: string
  password: string
  is_admin: boolean
}

export type UpdateUserInput = {
  email: string
  name: string
  is_admin: boolean
  version: number
}

export type UpdateUserPasswordInput = {
  password: string
  version: number
}

export type UpdateUserStatusInput = {
  status: UserStatus
  version: number
}

export const currentUserQueryOptions = queryOptions({
  queryKey: ["auth", "me"],
  queryFn: async () => {
    const res = await api.get<{ user: User }>("/auth/me")
    return res.data.user
  },
  retry: (failureCount, error) => {
    if (isAxiosError(error) && error.response?.status === 401) {
      return false
    }

    return failureCount < 2
  },
})

export const usersQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["users", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<UsersResponse>("/users", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const userQueryOptions = (userID: number) =>
  queryOptions({
    queryKey: ["users", "detail", userID],
    queryFn: async () => {
      const res = await api.get<UserResponse>(`/users/${userID}`)
      return res.data.user
    },
    enabled: userID > 0,
  })

export async function createUser(input: CreateUserInput) {
  const res = await api.post<UserResponse>("/users", input)
  return res.data.user
}

export async function updateUser(userID: number, input: UpdateUserInput) {
  const res = await api.put<UserResponse>(`/users/${userID}`, input)
  return res.data.user
}

export async function updateUserPassword(
  userID: number,
  input: UpdateUserPasswordInput,
) {
  const res = await api.patch<UserResponse>(`/users/${userID}/password`, input)
  return res.data.user
}

export async function updateUserStatus(
  userID: number,
  input: UpdateUserStatusInput,
) {
  const res = await api.patch<UserResponse>(`/users/${userID}/status`, input)
  return res.data.user
}
