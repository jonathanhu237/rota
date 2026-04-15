import { keepPreviousData, queryOptions } from "@tanstack/react-query"
import { isAxiosError } from "axios"

import api from "./axios"
import type { Pagination, Position, User, UserStatus } from "./types"

export type UsersResponse = {
  users: User[]
  pagination: Pagination
}

export type UserResponse = {
  user: User
}

export type PositionsResponse = {
  positions: Position[]
  pagination: Pagination
}

export type PositionResponse = {
  position: Position
}

export type UserPositionsResponse = {
  positions: Position[]
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

export type CreatePositionInput = {
  name: string
  description: string
}

export type UpdatePositionInput = {
  name: string
  description: string
}

export type ReplaceUserPositionsInput = {
  position_ids: number[]
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

export const positionsQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["positions", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<PositionsResponse>("/positions", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const allPositionsQueryOptions = () =>
  queryOptions({
    queryKey: ["positions", "all"],
    queryFn: async () => {
      const res = await api.get<PositionsResponse>("/positions", {
        params: {
          page: 1,
          page_size: 100,
        },
      })
      return res.data.positions
    },
  })

export const positionQueryOptions = (positionID: number) =>
  queryOptions({
    queryKey: ["positions", "detail", positionID],
    queryFn: async () => {
      const res = await api.get<PositionResponse>(`/positions/${positionID}`)
      return res.data.position
    },
    enabled: positionID > 0,
  })

export const userPositionsQueryOptions = (userID: number) =>
  queryOptions({
    queryKey: ["users", "positions", userID],
    queryFn: async () => {
      const res = await api.get<UserPositionsResponse>(`/users/${userID}/positions`)
      return res.data.positions
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

export async function replaceUserPositions(
  userID: number,
  positionIDs: number[],
) {
  await api.put<undefined, unknown, ReplaceUserPositionsInput>(
    `/users/${userID}/positions`,
    {
      position_ids: positionIDs,
    },
  )
}

export async function createPosition(input: CreatePositionInput) {
  const res = await api.post<PositionResponse>("/positions", input)
  return res.data.position
}

export async function updatePosition(
  positionID: number,
  input: UpdatePositionInput,
) {
  const res = await api.put<PositionResponse>(`/positions/${positionID}`, input)
  return res.data.position
}

export async function deletePosition(positionID: number) {
  await api.delete(`/positions/${positionID}`)
}
