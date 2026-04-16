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

type ActivatePublicationDialogProps = {
  open: boolean
  publication?: Publication | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function ActivatePublicationDialog({
  open,
  publication,
  isPending,
  onConfirm,
  onOpenChange,
}: ActivatePublicationDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("publications.activateDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("publications.activateDialog.description", {
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
          <Button type="button" onClick={onConfirm} disabled={isPending}>
            {isPending
              ? t("publications.activateDialog.submitting")
              : t("publications.activateDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
