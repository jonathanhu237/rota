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

type DeleteTemplateShiftDialogProps = {
  open: boolean
  isPending: boolean
  summary: string
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function DeleteTemplateShiftDialog({
  open,
  isPending,
  summary,
  onConfirm,
  onOpenChange,
}: DeleteTemplateShiftDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("templates.deleteShiftDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("templates.deleteShiftDialog.description", {
              summary,
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
              ? t("templates.deleteShiftDialog.submitting")
              : t("templates.deleteShiftDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
