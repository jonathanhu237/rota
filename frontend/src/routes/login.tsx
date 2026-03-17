import { isAxiosError } from "axios"
import { createFileRoute, redirect } from "@tanstack/react-router"

import { LoginPage } from "@/components/login-page"
import api from "@/lib/axios"

export const Route = createFileRoute("/login")({
  beforeLoad: async () => {
    try {
      await api.get("/auth/me")
      throw redirect({ to: "/" })
    } catch (error) {
      if (isAxiosError(error) && error.response?.status === 401) {
        return
      }

      throw error
    }
  },
  component: LoginPage,
})
