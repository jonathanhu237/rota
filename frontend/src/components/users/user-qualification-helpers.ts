export function normalizeQualificationPositionIDs(positionIDs: number[]) {
  return Array.from(new Set(positionIDs)).sort((left, right) => left - right)
}

export function hasQualificationSelectionChanged(
  initialPositionIDs: number[],
  nextPositionIDs: number[],
) {
  const normalizedInitial = normalizeQualificationPositionIDs(initialPositionIDs)
  const normalizedNext = normalizeQualificationPositionIDs(nextPositionIDs)

  if (normalizedInitial.length !== normalizedNext.length) {
    return true
  }

  return normalizedInitial.some((positionID, index) => {
    return positionID !== normalizedNext[index]
  })
}

type ShouldInitializeQualificationSelectionInput = {
  open: boolean
  wasOpen: boolean
  userID: number
  initializedUserID: number | null
}

export function shouldInitializeQualificationSelection({
  open,
  wasOpen,
  userID,
  initializedUserID,
}: ShouldInitializeQualificationSelectionInput) {
  if (!open) {
    return false
  }

  if (!wasOpen) {
    return true
  }

  return initializedUserID !== userID
}
