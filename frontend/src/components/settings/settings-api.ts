import {
  changeOwnPassword,
  requestEmailChange,
  updateBranding,
  updateOwnProfile,
  type ChangeOwnPasswordInput,
  type RequestEmailChangeInput,
  type UpdateBrandingInput,
  type UpdateOwnProfileInput,
} from "@/lib/queries"

export const changeOwnPasswordMutation = {
  mutationFn: (input: ChangeOwnPasswordInput) => changeOwnPassword(input),
}

export const requestEmailChangeMutation = {
  mutationFn: (input: RequestEmailChangeInput) => requestEmailChange(input),
}

export const updateOwnProfileMutation = {
  mutationFn: (input: UpdateOwnProfileInput) => updateOwnProfile(input),
}

export const updateBrandingMutation = {
  mutationFn: (input: UpdateBrandingInput) => updateBranding(input),
}
