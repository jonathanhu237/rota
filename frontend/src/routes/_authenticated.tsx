import { isAxiosError } from "axios"
import { createFileRoute, redirect, Outlet } from "@tanstack/react-router"

import api from "@/lib/axios"

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async () => {
    try {
      await api.get("/auth/me")
    } catch (error) {
      if (isAxiosError(error) && error.response?.status === 401) {
        throw redirect({ to: "/login" })
      }

      throw error
    }
  },
  component: () => <Outlet />,
})
