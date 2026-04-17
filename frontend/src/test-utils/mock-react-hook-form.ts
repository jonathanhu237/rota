type ErrorMap = Record<string, string>
type ValuesMap = Record<string, unknown>

export function createMockReactHookForm<TValues extends ValuesMap>(
  defaultValues: Partial<TValues> = {},
) {
  let values: Partial<TValues> = { ...defaultValues }
  let errors: ErrorMap = {}

  function setValues(nextValues: Partial<TValues>) {
    values = { ...defaultValues, ...nextValues }
  }

  function setErrors(nextErrors: ErrorMap) {
    errors = { ...nextErrors }
  }

  function reset() {
    values = { ...defaultValues }
    errors = {}
  }

  function useFormMock(options?: { defaultValues?: Partial<TValues> }) {
    setValues(options?.defaultValues ?? defaultValues)

    return {
      register(name: string, registerOptions?: { valueAsNumber?: boolean }) {
        return {
          name,
          onBlur: () => undefined,
          onChange: (event: { currentTarget: { value: string } }) => {
            const rawValue = event.currentTarget.value
            const nextValue = registerOptions?.valueAsNumber
              ? Number(rawValue)
              : rawValue

            values = {
              ...values,
              [name]: nextValue,
            }
          },
          ref: () => undefined,
        }
      },
      handleSubmit(
        callback: (submittedValues: TValues) => void,
      ) {
        return (event?: { preventDefault?: () => void }) => {
          event?.preventDefault?.()

          if (Object.keys(errors).length === 0) {
            callback(values as TValues)
          }
        }
      },
      reset: (nextValues: Partial<TValues>) => {
        setValues(nextValues)
      },
      trigger: async () => true,
      formState: {
        get errors() {
          return Object.fromEntries(
            Object.entries(errors).map(([key, message]) => [
              key,
              { message },
            ]),
          )
        },
      },
    }
  }

  return {
    reset,
    setErrors,
    setValues,
    useFormMock,
  }
}
