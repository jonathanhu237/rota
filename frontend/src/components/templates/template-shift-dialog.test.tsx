import { within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import type { TemplateShiftFormValues } from "./template-schemas"
import { TemplateShiftDialog } from "./template-shift-dialog"

const { form } = vi.hoisted(() => ({
  form: (() => {
    let values: Partial<TemplateShiftFormValues> = {
      weekday: 2,
      start_time: "09:00",
      end_time: "10:00",
      position_id: 0,
      required_headcount: 1,
    }
    let errors: Record<string, string> = {}

    return {
      reset() {
        values = {
          weekday: 2,
          start_time: "09:00",
          end_time: "10:00",
          position_id: 0,
          required_headcount: 1,
        }
        errors = {}
      },
      setErrors(nextErrors: Record<string, string>) {
        errors = { ...nextErrors }
      },
      useFormMock(options?: {
        defaultValues?: Partial<TemplateShiftFormValues>
      }) {
        values = {
          weekday: 2,
          start_time: "09:00",
          end_time: "10:00",
          position_id: 0,
          required_headcount: 1,
          ...(options?.defaultValues ?? {}),
        }

        return {
          register(
            name: keyof TemplateShiftFormValues,
            registerOptions?: { valueAsNumber?: boolean },
          ) {
            return {
              name,
              onBlur: () => undefined,
              onChange: (event: { currentTarget: { value: string } }) => {
                const nextValue = registerOptions?.valueAsNumber
                  ? Number(event.currentTarget.value)
                  : event.currentTarget.value

                values = {
                  ...values,
                  [name]: nextValue,
                }
              },
              ref: () => undefined,
            }
          },
          handleSubmit(
            callback: (submittedValues: TemplateShiftFormValues) => void,
          ) {
            return (event?: { preventDefault?: () => void }) => {
              event?.preventDefault?.()

              if (Object.keys(errors).length === 0) {
                callback(values as TemplateShiftFormValues)
              }
            }
          },
          reset(nextValues: Partial<TemplateShiftFormValues>) {
            values = {
              weekday: 2,
              start_time: "09:00",
              end_time: "10:00",
              position_id: 0,
              required_headcount: 1,
              ...nextValues,
            }
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
      },
    }
  })(),
}))

vi.mock("react-hook-form", async () => {
  const actual = await vi.importActual<typeof import("react-hook-form")>(
    "react-hook-form",
  )

  return {
    ...actual,
    useForm: form.useFormMock,
  }
})

beforeEach(() => {
  form.reset()
})

const positions = [
  {
    id: 7,
    name: "Front Desk",
    description: "",
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
  },
]

describe("TemplateShiftDialog", () => {
  it("renders all fields", () => {
    const { getByLabelText } = renderWithProviders(
      <TemplateShiftDialog
        initialWeekday={2}
        isPending={false}
        mode="create"
        open
        positions={positions}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
        shift={null}
      />,
    )

    expect(getByLabelText("templates.shift.weekday")).toBeInTheDocument()
    expect(getByLabelText("templates.shift.startTime")).toBeInTheDocument()
    expect(getByLabelText("templates.shift.endTime")).toBeInTheDocument()
    expect(getByLabelText("templates.shift.position")).toBeInTheDocument()
    expect(getByLabelText("templates.shift.requiredHeadcount")).toBeInTheDocument()
  })

  it("submits valid values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateShiftDialog
        initialWeekday={2}
        isPending={false}
        mode="create"
        open
        positions={positions}
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
        shift={null}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.selectOptions(
      within(dialog).getByLabelText("templates.shift.position"),
      "7",
    )
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(onSubmit).toHaveBeenCalledWith({
      weekday: 2,
      start_time: "09:00",
      end_time: "10:00",
      position_id: 7,
      required_headcount: 1,
    })
  })

  it("shows a validation error when end time is not after start time", () => {
    form.setErrors({
      end_time: "templates.validation.invalidShiftTime",
    })

    const onSubmit = vi.fn()
    renderWithProviders(
      <TemplateShiftDialog
        initialWeekday={2}
        isPending={false}
        mode="create"
        open
        positions={positions}
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
        shift={null}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    expect(
      within(dialog).getByText("templates.validation.invalidShiftTime"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("shows a validation error when required headcount is below one", () => {
    form.setErrors({
      required_headcount: "templates.validation.invalidHeadcount",
    })

    const onSubmit = vi.fn()
    renderWithProviders(
      <TemplateShiftDialog
        initialWeekday={2}
        isPending={false}
        mode="create"
        open
        positions={positions}
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
        shift={null}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    expect(
      within(dialog).getByText("templates.validation.invalidHeadcount"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
