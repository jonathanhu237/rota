export type UserStatus = "active" | "disabled"

export type User = {
  id: number
  email: string
  name: string
  is_admin: boolean
  status: UserStatus
  version: number
}

export type Pagination = {
  page: number
  page_size: number
  total: number
  total_pages: number
}
