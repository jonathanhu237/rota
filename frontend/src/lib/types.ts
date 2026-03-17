export type User = {
  id: number
  email: string
  name: string
  is_admin: boolean
  status: "active" | "disabled"
}
