import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/rota')({
  component: RotaLayout,
})

function RotaLayout() {
  return <Outlet />
}
