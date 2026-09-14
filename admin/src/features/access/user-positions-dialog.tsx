import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRotaTranslation as useTranslation } from '@/features/rota/i18n'
import { allPositionsQueryOptions, replaceUserPositions, userPositionsQueryOptions } from '@/features/rota/queries'
import type { Position } from '@/features/rota/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { notifyRequestError, notifySuccess } from '@/shared/feedback'
import { getTranslatedApiError } from '@/features/rota/api-error'
import type { AccessUser } from './access-components'

export function UserPositionsDialog({
  user,
  open,
  canManage,
  onClose,
}: {
  user?: AccessUser
  open: boolean
  canManage: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const userID = user?.id ?? ''
  const positionsQuery = useQuery({
    ...allPositionsQueryOptions(),
    enabled: open && canManage,
  })
  const selectedQuery = useQuery({
    ...userPositionsQueryOptions(userID),
    enabled: open && canManage && userID !== '',
  })
  const [selectedIDs, setSelectedIDs] = useState<number[]>([])

  useEffect(() => {
    if (!open || !selectedQuery.data) return
    setSelectedIDs(selectedQuery.data.map((position) => position.id).sort((a, b) => a - b))
  }, [open, selectedQuery.data])

  const initialIDs = useMemo(
    () => (selectedQuery.data ?? []).map((position) => position.id).sort((a, b) => a - b),
    [selectedQuery.data],
  )
  const isDirty = selectedIDs.length !== initialIDs.length || selectedIDs.some((id, index) => id !== initialIDs[index])
  const mutation = useMutation({
    mutationFn: () => replaceUserPositions(userID, selectedIDs),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['users', 'positions', userID] })
      notifySuccess(t('users.qualifications.success.saved'))
      onClose()
    },
    onError: (error) => {
      notifyRequestError(error, t, {
        title: t('users.qualifications.title'),
        description: getTranslatedApiError(t, error, 'users.errors', 'users.errors.INTERNAL_ERROR'),
      })
    },
  })

  if (!user || !canManage) return null
  const loading = positionsQuery.isLoading || selectedQuery.isLoading
  const toggle = (position: Position, checked: boolean) => {
    setSelectedIDs((current) => {
      const next = checked ? [...current, position.id] : current.filter((id) => id !== position.id)
      return [...new Set(next)].sort((a, b) => a - b)
    })
  }

  return (
    <Dialog open={open} onOpenChange={(value) => { if (!value && !mutation.isPending) onClose() }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto" closeLabel={t('common.close')}>
        <DialogHeader>
          <DialogTitle>{t('users.qualifications.title')}</DialogTitle>
          <DialogDescription>{user.name} · {user.email}</DialogDescription>
        </DialogHeader>
        {loading ? <Skeleton className="h-40 w-full" /> : positionsQuery.isError || selectedQuery.isError ? (
          <p className="text-sm text-destructive">{t('users.qualifications.loadError')}</p>
        ) : positionsQuery.data?.length ? (
          <fieldset className="grid gap-2">
            <legend className="text-sm font-medium">{t('users.qualifications.description')}</legend>
            {positionsQuery.data.map((position) => (
              <label key={position.id} className="flex items-center gap-3 rounded-lg border p-3 text-sm">
                <Checkbox
                  checked={selectedIDs.includes(position.id)}
                  disabled={mutation.isPending}
                  onCheckedChange={(value) => toggle(position, value === true)}
                  aria-label={position.name}
                />
                <span className="font-medium">{position.name}</span>
                {position.description ? <Badge variant="outline" className="ml-auto max-w-[50%] truncate">{position.description}</Badge> : null}
              </label>
            ))}
          </fieldset>
        ) : <p className="text-sm text-muted-foreground">{t('users.qualifications.empty')}</p>}
        <DialogFooter>
          <DialogClose asChild><Button type="button" variant="outline" disabled={mutation.isPending}>{t('common.cancel')}</Button></DialogClose>
          <Button type="button" disabled={loading || !isDirty || mutation.isPending || positionsQuery.isError || selectedQuery.isError} onClick={() => mutation.mutate()}>
            {mutation.isPending ? t('users.qualifications.saving') : t('users.qualifications.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
