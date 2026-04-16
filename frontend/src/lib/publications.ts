import type { PublicationState } from "./types"

export function getPublicationStateTranslationKey(state: PublicationState) {
  switch (state) {
    case "DRAFT":
      return "publications.state.draft"
    case "COLLECTING":
      return "publications.state.collecting"
    case "ASSIGNING":
      return "publications.state.assigning"
    case "ACTIVE":
      return "publications.state.active"
    case "ENDED":
      return "publications.state.ended"
  }
}

export type PublicationLifecycleAction = "activate" | "end"

export function getPublicationLifecycleAction(
  state: PublicationState,
): PublicationLifecycleAction | null {
  switch (state) {
    case "ASSIGNING":
      return "activate"
    case "ACTIVE":
      return "end"
    default:
      return null
  }
}

export type { PublicationState } from "./types"
