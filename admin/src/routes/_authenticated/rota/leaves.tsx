import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/rota/leaves')({
  component: LeavesLayout,
})

function LeavesLayout() {
  return <Outlet />
}
