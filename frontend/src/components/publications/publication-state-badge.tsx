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
  ACTIVE: "default",
  ENDED: "destructive",
}

type PublicationStateBadgeProps = {
  state: PublicationState
}

export function PublicationStateBadge({
  state,
}: PublicationStateBadgeProps) {
  const { t } = useTranslation()

  return (
    <Badge variant={variantByState[state]}>
      {t(getPublicationStateTranslationKey(state))}
    </Badge>
  )
}
