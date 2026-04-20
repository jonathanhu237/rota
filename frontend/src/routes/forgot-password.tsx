import { isAxiosError } from "axios"
import { createFileRoute, redirect } from "@tanstack/react-router"

import { ForgotPasswordPage } from "@/components/auth/forgot-password-page"
import api from "@/lib/axios"

export const Route = createFileRoute("/forgot-password")({
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
  component: ForgotPasswordPage,
})
