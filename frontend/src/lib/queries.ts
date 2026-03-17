import { queryOptions } from "@tanstack/react-query"
import { isAxiosError } from "axios"

import api from "./axios"
import type { User } from "./types"

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
