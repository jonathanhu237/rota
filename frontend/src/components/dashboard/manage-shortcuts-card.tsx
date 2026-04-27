import { Link } from "@tanstack/react-router"
import {
  Briefcase,
  CalendarDays,
  ChevronRight,
  FileText,
  Users,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

const shortcuts = [
  {
    key: "users",
    to: "/users",
    icon: Users,
  },
  {
    key: "positions",
    to: "/positions",
    icon: Briefcase,
  },
  {
    key: "templates",
    to: "/templates",
    icon: CalendarDays,
  },
  {
    key: "publications",
    to: "/publications",
    icon: FileText,
  },
] as const

export function ManageShortcutsCard() {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("dashboard.manage.title")}</CardTitle>
        <CardDescription>{t("dashboard.manage.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        {shortcuts.map((shortcut) => (
          <Link
            key={shortcut.key}
            to={shortcut.to}
            className={buttonVariants({ variant: "outline" })}
          >
            <shortcut.icon data-icon="inline-start" />
            {t(`dashboard.manage.links.${shortcut.key}`)}
            <ChevronRight data-icon="inline-end" />
          </Link>
        ))}
      </CardContent>
    </Card>
  )
}
