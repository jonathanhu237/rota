import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, Link } from "@tanstack/react-router"
import { ArrowLeft, Copy, Send } from "lucide-react"
import { useTranslation } from "react-i18next"

import { DatePicker } from "@/components/date-time-picker"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  createLeave,
  currentPublicationQueryOptions,
  leavePreviewQueryOptions,
  myLeavesQueryOptions,
  publicationMembersQueryOptions,
} from "@/lib/queries"
import type {
  LeaveCategory,
  LeavePreviewOccurrence,
  PublicationMember,
  ShiftChangeType,
} from "@/lib/types"

export const Route = createFileRoute("/_authenticated/leaves/new")({
  component: LeavePage,
})

type RowDraft = {
  type: Exclude<ShiftChangeType, "swap">
  counterpartUserID: string
  category: LeaveCategory
  reason: string
}

const defaultDraft: RowDraft = {
  type: "give_pool",
  counterpartUserID: "",
  category: "personal",
  reason: "",
}

export function LeavePage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const today = new Date().toISOString().slice(0, 10)
  const twoWeeks = addDays(today, 14)
  const [from, setFrom] = useState(today)
  const [to, setTo] = useState(twoWeeks)
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({})
  const [shareURLs, setShareURLs] = useState<string[]>([])

  const currentPublicationQuery = useQuery(currentPublicationQueryOptions)
  const publicationID = currentPublicationQuery.data?.id ?? 0
  const membersQuery = useQuery({
    ...publicationMembersQueryOptions(publicationID),
    enabled: publicationID > 0,
  })
  const previewQuery = useQuery(leavePreviewQueryOptions(from, to))
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  const memberOptions = useMemo(
    () => membersQuery.data ?? [],
    [membersQuery.data],
  )

  const createMutation = useMutation({
    mutationFn: async ({
      occurrence,
      draft,
    }: {
      occurrence: LeavePreviewOccurrence
      draft: RowDraft
    }) =>
      createLeave({
        assignment_id: occurrence.assignment_id,
        occurrence_date: occurrence.occurrence_date,
        type: draft.type,
        counterpart_user_id:
          draft.type === "give_direct" && draft.counterpartUserID !== ""
            ? Number(draft.counterpartUserID)
            : null,
        category: draft.category,
        reason: draft.reason,
      }),
    onSuccess: async (leave) => {
      setShareURLs((current) =>
        current.includes(leave.share_url)
          ? current
          : [...current, leave.share_url],
      )
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["me", "leaves"] }),
        queryClient.invalidateQueries({
          queryKey: myLeavesQueryOptions(1, 10).queryKey,
        }),
      ])
      toast({ variant: "default", description: t("leave.toast.created") })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "leave.errors",
          "leave.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const updateDraft = (key: string, patch: Partial<RowDraft>) => {
    setDrafts((current) => ({
      ...current,
      [key]: {
        ...(current[key] ?? defaultDraft),
        ...patch,
      },
    }))
  }

  const submitOccurrence = (occurrence: LeavePreviewOccurrence) => {
    const draft = drafts[rowKey(occurrence)] ?? defaultDraft
    createMutation.mutate({ occurrence, draft })
  }

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="grid gap-1">
              <CardTitle>{t("leave.title")}</CardTitle>
              <CardDescription>{t("leave.description")}</CardDescription>
            </div>
            <Link
              to="/leaves"
              className={buttonVariants({ variant: "outline" })}
            >
              <ArrowLeft />
              {t("leaves.backToHistory")}
            </Link>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-4 sm:grid-cols-[minmax(0,180px)_minmax(0,180px)]">
            <div className="grid gap-2">
              <Label htmlFor="leave-from">{t("leave.from")}</Label>
              <DatePicker
                id="leave-from"
                value={from}
                onChange={setFrom}
                placeholder={t("common.selectDate")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="leave-to">{t("leave.to")}</Label>
              <DatePicker
                id="leave-to"
                value={to}
                onChange={setTo}
                placeholder={t("common.selectDate")}
              />
            </div>
          </div>
          {shareURLs.length > 0 && (
            <div className="grid gap-2 rounded-lg border p-3">
              <div className="text-sm font-medium">{t("leave.shareUrls")}</div>
              <div className="grid gap-2">
                {shareURLs.map((url) => (
                  <div
                    key={url}
                    className="flex flex-wrap items-center gap-2 text-sm"
                  >
                    <a href={url} className="font-medium underline">
                      {url}
                    </a>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => void navigator.clipboard?.writeText(url)}
                    >
                      <Copy />
                      {t("leave.copy")}
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {previewQuery.isLoading || membersQuery.isLoading ? (
        <div className="grid gap-3">
          <Skeleton className="h-36 w-full" />
          <Skeleton className="h-36 w-full" />
        </div>
      ) : previewQuery.isError || membersQuery.isError ? (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {getTranslatedApiError(
                t,
                previewQuery.error ?? membersQuery.error,
                "leave.errors",
                "leave.errors.INTERNAL_ERROR",
              )}
            </div>
          </CardContent>
        </Card>
      ) : previewQuery.data && previewQuery.data.length > 0 ? (
        <div className="grid gap-3">
          {previewQuery.data.map((occurrence) => {
            const key = rowKey(occurrence)
            const draft = drafts[key] ?? defaultDraft
            return (
              <LeaveOccurrenceCard
                key={key}
                occurrence={occurrence}
                draft={draft}
                members={memberOptions}
                formatter={formatter}
                isPending={createMutation.isPending}
                onChange={(patch) => updateDraft(key, patch)}
                onSubmit={() => submitOccurrence(occurrence)}
              />
            )
          })}
        </div>
      ) : (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
              {t("leave.emptyPreview")}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function LeaveOccurrenceCard({
  occurrence,
  draft,
  members,
  formatter,
  isPending,
  onChange,
  onSubmit,
}: {
  occurrence: LeavePreviewOccurrence
  draft: RowDraft
  members: PublicationMember[]
  formatter: Intl.DateTimeFormat
  isPending: boolean
  onChange: (patch: Partial<RowDraft>) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  const requiresCounterpart = draft.type === "give_direct"
  const canSubmit = !requiresCounterpart || draft.counterpartUserID !== ""

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {occurrence.position.name} · {occurrence.occurrence_date}
        </CardTitle>
        <CardDescription>
          {formatter.format(new Date(occurrence.occurrence_start))} -{" "}
          {formatter.format(new Date(occurrence.occurrence_end))}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="grid gap-4 md:grid-cols-4">
          <div className="grid gap-2">
            <Label>{t("leave.typeLabel")}</Label>
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              value={draft.type}
              onChange={(event) =>
                onChange({
                  type: event.target.value as RowDraft["type"],
                  counterpartUserID: "",
                })
              }
            >
              <option value="give_pool">{t("leave.typeGivePool")}</option>
              <option value="give_direct">{t("leave.typeGiveDirect")}</option>
            </select>
          </div>
          <div className="grid gap-2">
            <Label>{t("leave.categoryLabel")}</Label>
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              value={draft.category}
              onChange={(event) =>
                onChange({ category: event.target.value as LeaveCategory })
              }
            >
              <option value="sick">{t("leave.categorySick")}</option>
              <option value="personal">{t("leave.categoryPersonal")}</option>
              <option value="bereavement">
                {t("leave.categoryBereavement")}
              </option>
            </select>
          </div>
          <div className="grid gap-2 md:col-span-2">
            <Label>{t("leave.counterpart")}</Label>
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm disabled:opacity-60"
              value={draft.counterpartUserID}
              disabled={!requiresCounterpart}
              onChange={(event) =>
                onChange({ counterpartUserID: event.target.value })
              }
            >
              <option value="">{t("leave.counterpartPlaceholder")}</option>
              {members.map((member) => (
                <option key={member.user_id} value={member.user_id}>
                  {member.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="grid gap-2">
          <Label>{t("leave.reason")}</Label>
          <Textarea
            value={draft.reason}
            onChange={(event) => onChange({ reason: event.target.value })}
            placeholder={t("leave.reasonPlaceholder")}
          />
        </div>
        <div>
          <Button
            type="button"
            disabled={isPending || !canSubmit}
            onClick={onSubmit}
          >
            <Send />
            {t("leave.submit")}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function rowKey(occurrence: LeavePreviewOccurrence) {
  return `${occurrence.assignment_id}:${occurrence.occurrence_date}`
}

function addDays(dateValue: string, days: number) {
  const date = new Date(`${dateValue}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}
