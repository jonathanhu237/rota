import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { getPublicationStateTranslationKey } from "@/lib/publications"
import type { PublicationState } from "@/lib/types"

const variantByState: Record<
  PublicationState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  DRAFT: "secondary",
  COLLECTING: "default",
  ASSIGNING: "outline",
  PUBLISHED: "default",
  ACTIVE: "default",
  ENDED: "destructive",
}

// PUBLISHED uses a distinct blue tone to differentiate it from ACTIVE while still
// conveying that the roster is visible to employees.
const classNameByState: Partial<Record<PublicationState, string>> = {
  PUBLISHED: "bg-blue-500 text-white hover:bg-blue-500/90",
}

type PublicationStateBadgeProps = {
  state: PublicationState
}

export function PublicationStateBadge({
  state,
}: PublicationStateBadgeProps) {
  const { t } = useTranslation()

  return (
    <Badge variant={variantByState[state]} className={classNameByState[state]}>
      {t(getPublicationStateTranslationKey(state))}
    </Badge>
  )
}
