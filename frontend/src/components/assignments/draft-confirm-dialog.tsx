import { AlertTriangle } from "lucide-react"
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

export type DraftConfirmWarning = {
  id: string
  userName: string
  slotLabel: string
  positionName: string
}

type DraftConfirmDialogProps = {
  open: boolean
  warnings: DraftConfirmWarning[]
  isPending: boolean
  onCancel: () => void
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function DraftConfirmDialog({
  open,
  warnings,
  isPending,
  onCancel,
  onConfirm,
  onOpenChange,
}: DraftConfirmDialogProps) {
  const { t } = useTranslation()

  if (warnings.length === 0) {
    return null
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("assignments.drafts.confirmDialog.title")}
          </DialogTitle>
          <DialogDescription>
            {t("assignments.drafts.confirmDialog.description")}
          </DialogDescription>
        </DialogHeader>

        <div className="grid max-h-80 gap-3 overflow-y-auto">
          {warnings.map((warning) => (
            <div
              key={warning.id}
              className="grid gap-1 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm"
            >
              <div className="flex items-start gap-2 font-medium">
                <AlertTriangle className="mt-0.5 size-4 shrink-0 text-destructive" />
                <span>
                  {t("assignments.drafts.confirmDialog.warningTitle", {
                    user: warning.userName,
                    slot: warning.slotLabel,
                    position: warning.positionName,
                  })}
                </span>
              </div>
              <p className="pl-6 text-muted-foreground">
                {t("assignments.drafts.confirmDialog.warningReason", {
                  user: warning.userName,
                  position: warning.positionName,
                })}
              </p>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("assignments.drafts.cancel")}
          </Button>
          <Button type="button" onClick={onConfirm} disabled={isPending}>
            {t("assignments.drafts.confirmAndSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
