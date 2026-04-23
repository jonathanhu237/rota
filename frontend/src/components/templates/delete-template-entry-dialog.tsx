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

type DeleteTemplateEntryDialogProps = {
  open: boolean
  isPending: boolean
  titleKey: string
  descriptionKey: string
  confirmKey: string
  pendingKey: string
  summary: string
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function DeleteTemplateEntryDialog({
  open,
  isPending,
  titleKey,
  descriptionKey,
  confirmKey,
  pendingKey,
  summary,
  onConfirm,
  onOpenChange,
}: DeleteTemplateEntryDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(titleKey)}</DialogTitle>
          <DialogDescription>
            {t(descriptionKey, { summary })}
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
            {isPending ? t(pendingKey) : t(confirmKey)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
