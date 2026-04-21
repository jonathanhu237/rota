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

type PublishPublicationDialogProps = {
  open: boolean
  publication?: Publication | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function PublishPublicationDialog({
  open,
  publication,
  isPending,
  onConfirm,
  onOpenChange,
}: PublishPublicationDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("publications.publishDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("publications.publishDialog.description", {
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
            {t("publications.publishDialog.cancel")}
          </Button>
          <Button type="button" onClick={onConfirm} disabled={isPending}>
            {isPending
              ? t("publications.publishDialog.submitting")
              : t("publications.publishDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
