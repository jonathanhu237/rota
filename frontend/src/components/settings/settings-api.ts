import {
  changeOwnPassword,
  updateOwnProfile,
  type ChangeOwnPasswordInput,
  type UpdateOwnProfileInput,
} from "@/lib/queries"

export const changeOwnPasswordMutation = {
  mutationFn: (input: ChangeOwnPasswordInput) => changeOwnPassword(input),
}

export const updateOwnProfileMutation = {
  mutationFn: (input: UpdateOwnProfileInput) => updateOwnProfile(input),
}
