import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useNavigate, useRouterState } from "@tanstack/react-router"
import {
  Briefcase,
  CalendarCheck,
  CalendarDays,
  CalendarRange,
  CalendarX,
  ChevronsUpDown,
  FileText,
  Home,
  Inbox,
  LogOut,
  Settings,
  SunMoon,
  Users,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { useTheme } from "@/components/theme-context"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import api from "@/lib/axios"
import {
  brandingFallback,
  brandingQueryOptions,
  currentUserQueryOptions,
  updateOwnProfile,
  unreadNotificationsQueryOptions,
} from "@/lib/queries"

export function AppSidebar() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const routerState = useRouterState()
  const { toggleThemePreference } = useTheme()

  const { data: user } = useQuery(currentUserQueryOptions)
  const { data: branding = brandingFallback } = useQuery(brandingQueryOptions)
  const productName = branding.product_name
  const productInitial = Array.from(productName.trim())[0]?.toUpperCase() ?? "R"
  const unreadCountQuery = useQuery(unreadNotificationsQueryOptions)
  const unreadCount = unreadCountQuery.data ?? 0
  const showUnreadBadge = !unreadCountQuery.isLoading && unreadCount > 0

  const logoutMutation = useMutation({
    mutationFn: () => api.post("/auth/logout"),
    onSuccess: () => {
      queryClient.clear()
      navigate({ to: "/login" })
    },
  })

  const toggleThemeMutation = useMutation({
    mutationFn: (themePreference: "light" | "dark") =>
      updateOwnProfile({ theme_preference: themePreference }),
    onSuccess: (updatedUser) => {
      queryClient.setQueryData(["auth", "me"], updatedUser)
    },
  })

  const handleToggleTheme = () => {
    const nextPreference = toggleThemePreference()
    toggleThemeMutation.mutate(nextPreference)
  }

  type NavItem = {
    title: string
    url: string
    icon: typeof Home
    badge?: number
  }

  const employeeItems: NavItem[] = [
    {
      title: t("sidebar.dashboard"),
      url: "/",
      icon: Home,
    },
    {
      title: t("sidebar.roster"),
      url: "/roster",
      icon: CalendarRange,
    },
    {
      title: t("sidebar.availability"),
      url: "/availability",
      icon: CalendarCheck,
    },
    {
      title: t("sidebar.requests"),
      url: "/requests",
      icon: Inbox,
      badge: showUnreadBadge ? unreadCount : undefined,
    },
    {
      title: t("sidebar.leaves"),
      url: "/leaves",
      icon: CalendarX,
    },
  ]

  const accountItems: NavItem[] = [
    {
      title: t("sidebar.settings"),
      url: "/settings",
      icon: Settings,
    },
  ]

  const adminItems: NavItem[] = [
    {
      title: t("sidebar.users"),
      url: "/users",
      icon: Users,
    },
    {
      title: t("sidebar.positions"),
      url: "/positions",
      icon: Briefcase,
    },
    {
      title: t("sidebar.templates"),
      url: "/templates",
      icon: CalendarDays,
    },
    {
      title: t("sidebar.publications"),
      url: "/publications",
      icon: FileText,
    },
  ]

  const renderItem = (item: NavItem) => {
    const isActive =
      routerState.location.pathname === item.url ||
      routerState.location.pathname.startsWith(`${item.url}/`)

    return (
      <SidebarMenuItem key={item.url}>
        <SidebarMenuButton
          render={<Link to={item.url} />}
          isActive={isActive}
          tooltip={item.title}
        >
          <item.icon />
          <span>{item.title}</span>
          {item.badge !== undefined && item.badge > 0 && (
            <span
              data-testid={`sidebar-badge-${item.url}`}
              className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-xs font-medium text-primary-foreground"
            >
              {item.badge}
            </span>
          )}
        </SidebarMenuButton>
      </SidebarMenuItem>
    )
  }

  // Get the initials from the user's name
  const initials =
    user?.name
      ?.split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2) ?? ""

  return (
    <Sidebar collapsible="icon" variant="floating">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" render={<Link to="/" />}>
              <div className="bg-primary text-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg text-sm font-bold">
                {productInitial}
              </div>
              <div className="flex flex-col gap-0.5 leading-none">
                <span className="font-semibold">{productName}</span>
                <span className="text-xs text-muted-foreground">
                  {t("sidebar.appDescription")}
                </span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>
            {t("sidebar.groups.mySchedule")}
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>{employeeItems.map(renderItem)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        {user?.is_admin && (
          <SidebarGroup>
            <SidebarGroupLabel>{t("sidebar.groups.manage")}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>{adminItems.map(renderItem)}</SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
        <SidebarGroup>
          <SidebarGroupLabel>{t("sidebar.groups.account")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>{accountItems.map(renderItem)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <SidebarMenuButton
                    size="lg"
                    className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                  />
                }
              >
                <Avatar className="h-8 w-8 rounded-lg">
                  <AvatarFallback className="rounded-lg">
                    {initials}
                  </AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-semibold">{user?.name}</span>
                  <span className="truncate text-xs text-muted-foreground">
                    {user?.email}
                  </span>
                </div>
                <ChevronsUpDown className="ml-auto size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent
                className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
                side="bottom"
                align="end"
                sideOffset={4}
              >
                <DropdownMenuLabel className="p-0 font-normal">
                  <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                    <Avatar className="h-8 w-8 rounded-lg">
                      <AvatarFallback className="rounded-lg">
                        {initials}
                      </AvatarFallback>
                    </Avatar>
                    <div className="grid flex-1 text-left text-sm leading-tight">
                      <span className="truncate font-semibold">
                        {user?.name}
                      </span>
                      <span className="truncate text-xs text-muted-foreground">
                        {user?.email}
                      </span>
                    </div>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={handleToggleTheme}
                  disabled={toggleThemeMutation.isPending}
                >
                  <SunMoon />
                  {t("sidebar.toggleTheme")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => navigate({ to: "/settings" })}>
                  <Settings />
                  {t("sidebar.settings")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => logoutMutation.mutate()}
                  disabled={logoutMutation.isPending}
                >
                  <LogOut />
                  {t("sidebar.logout")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
