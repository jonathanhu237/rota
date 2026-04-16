import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { Publication } from "@/lib/types"

type EndPublicationDialogProps = {
  open: boolean
  publication?: Publication | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function EndPublicationDialog({
  open,
  publication,
  isPending,
  onConfirm,
  onOpenChange,
}: EndPublicationDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("publications.endDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("publications.endDialog.description", {
              name: publication?.name ?? "",
            })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={isPending}
          >
            {isPending
              ? t("publications.endDialog.submitting")
              : t("publications.endDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
