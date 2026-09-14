import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/rota/publications')({
  component: PublicationsLayout,
})

function PublicationsLayout() {
  return <Outlet />
}
