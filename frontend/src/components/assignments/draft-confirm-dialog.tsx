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
  unqualifiedDrafts: DraftConfirmWarning[]
  unsubmittedDrafts: DraftConfirmWarning[]
  isPending: boolean
  onCancel: () => void
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function DraftConfirmDialog({
  open,
  unqualifiedDrafts,
  unsubmittedDrafts,
  isPending,
  onCancel,
  onConfirm,
  onOpenChange,
}: DraftConfirmDialogProps) {
  const { t } = useTranslation()

  if (unqualifiedDrafts.length === 0 && unsubmittedDrafts.length === 0) {
    return null
  }
  const titleKey =
    unqualifiedDrafts.length > 0 && unsubmittedDrafts.length > 0
      ? "assignments.drafts.confirmDialog.titleBoth"
      : unsubmittedDrafts.length > 0
        ? "assignments.drafts.confirmDialog.titleUnsubmitted"
        : "assignments.drafts.confirmDialog.title"

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(titleKey)}</DialogTitle>
          <DialogDescription>
            {t("assignments.drafts.confirmDialog.description")}
          </DialogDescription>
        </DialogHeader>

        <div className="grid max-h-80 gap-3 overflow-y-auto">
          {unqualifiedDrafts.length > 0 && (
            <DraftConfirmSection
              tone="red"
              title={t("assignments.drafts.confirmDialog.unqualifiedSection", {
                count: unqualifiedDrafts.length,
              })}
              warnings={unqualifiedDrafts}
              reasonKey="assignments.drafts.confirmDialog.warningReason"
            />
          )}
          {unsubmittedDrafts.length > 0 && (
            <DraftConfirmSection
              tone="amber"
              title={t("assignments.drafts.confirmDialog.unsubmittedSection", {
                count: unsubmittedDrafts.length,
              })}
              warnings={unsubmittedDrafts}
              reasonKey="assignments.drafts.confirmDialog.unsubmittedReason"
            />
          )}
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

function DraftConfirmSection({
  tone,
  title,
  warnings,
  reasonKey,
}: {
  tone: "red" | "amber"
  title: string
  warnings: DraftConfirmWarning[]
  reasonKey: string
}) {
  const { t } = useTranslation()
  const isRed = tone === "red"

  return (
    <section
      className={
        isRed
          ? "grid gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-3"
          : "grid gap-2 rounded-lg border border-amber-300 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950/25"
      }
    >
      <div className="flex items-center gap-2 text-sm font-medium">
        <AlertTriangle
          className={
            isRed
              ? "size-4 shrink-0 text-destructive"
              : "size-4 shrink-0 text-amber-500"
          }
        />
        <span>{title}</span>
      </div>
      <div className="grid gap-2">
        {warnings.map((warning) => (
          <div key={warning.id} className="grid gap-1 text-sm">
            <div className="font-medium">
              {t("assignments.drafts.confirmDialog.warningTitle", {
                user: warning.userName,
                slot: warning.slotLabel,
                position: warning.positionName,
              })}
            </div>
            <p className="text-muted-foreground">
              {t(reasonKey, {
                user: warning.userName,
                position: warning.positionName,
              })}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}
