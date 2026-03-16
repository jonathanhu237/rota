import { createFileRoute } from "@tanstack/react-router"

export const Route = createFileRoute("/_authenticated/")({
  component: () => (
    <div className="flex h-screen items-center justify-center">
      <h1 className="text-2xl font-bold">Welcome to Rota</h1>
    </div>
  ),
})
