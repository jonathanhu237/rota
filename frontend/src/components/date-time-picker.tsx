import * as React from "react"
import { CalendarIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Input } from "@/components/ui/input"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { cn } from "@/lib/utils"

type DatePickerProps = {
  id?: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  placeholder?: string
  "aria-invalid"?: boolean
}

type TimePickerProps = Omit<
  React.ComponentProps<typeof Input>,
  "type" | "value" | "onChange"
> & {
  value: string
  onChange: (value: string) => void
}

type DateTimePickerProps = {
  id?: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  placeholder?: string
  timeLabel: string
  "aria-invalid"?: boolean
}

export function DatePicker({
  id,
  value,
  onChange,
  disabled,
  placeholder = "Select date",
  "aria-invalid": ariaInvalid,
}: DatePickerProps) {
  const [open, setOpen] = React.useState(false)
  const portalContainerRef = React.useRef<HTMLDivElement>(null)
  const selectedDate = parseDateValue(value)

  return (
    <div ref={portalContainerRef}>
      <Popover modal={false} open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              id={id}
              type="button"
              variant="outline"
              disabled={disabled}
              aria-invalid={ariaInvalid}
              className={cn(
                "w-full justify-start",
                !selectedDate && "text-muted-foreground",
              )}
            />
          }
        >
          <CalendarIcon data-icon="inline-start" />
          {selectedDate ? formatDisplayDate(selectedDate) : placeholder}
        </PopoverTrigger>
        <PopoverContent
          align="start"
          className="w-auto p-0"
          container={portalContainerRef}
        >
          <Calendar
            mode="single"
            selected={selectedDate}
            defaultMonth={selectedDate}
            onSelect={(date) => {
              onChange(date ? formatDateValue(date) : "")
              setOpen(false)
            }}
          />
        </PopoverContent>
      </Popover>
    </div>
  )
}

export function TimePicker({
  value,
  onChange,
  className,
  ...props
}: TimePickerProps) {
  return (
    <Input
      type="time"
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={className}
      {...props}
    />
  )
}

export function DateTimePicker({
  id,
  value,
  onChange,
  disabled,
  placeholder,
  timeLabel,
  "aria-invalid": ariaInvalid,
}: DateTimePickerProps) {
  const { date, time } = splitDateTimeValue(value)
  const [draftDate, setDraftDate] = React.useState(date)
  const [draftTime, setDraftTime] = React.useState(time)

  React.useEffect(() => {
    setDraftDate(date)
    setDraftTime(time)
  }, [date, time])

  return (
    <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_9rem]">
      <input
        type="datetime-local"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        data-testid={id ? `${id}-value` : undefined}
        aria-hidden="true"
        tabIndex={-1}
        className="sr-only"
      />
      <DatePicker
        id={id}
        value={draftDate}
        onChange={(nextDate) => {
          const nextTime = nextDate ? draftTime : ""
          setDraftDate(nextDate)
          setDraftTime(nextTime)
          onChange(nextDate && nextTime ? `${nextDate}T${nextTime}` : "")
        }}
        disabled={disabled}
        placeholder={placeholder}
        aria-invalid={ariaInvalid}
      />
      <TimePicker
        id={id ? `${id}-time` : undefined}
        value={draftTime}
        onChange={(nextTime) => {
          setDraftTime(nextTime)
          onChange(draftDate && nextTime ? `${draftDate}T${nextTime}` : "")
        }}
        disabled={disabled || !draftDate}
        aria-label={timeLabel}
        aria-invalid={ariaInvalid}
      />
    </div>
  )
}

function parseDateValue(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) {
    return undefined
  }

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const date = new Date(year, month - 1, day)

  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return undefined
  }

  return date
}

function splitDateTimeValue(value: string) {
  const match = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})$/.exec(value)
  if (!match || !parseDateValue(match[1])) {
    return { date: "", time: "" }
  }

  return { date: match[1], time: match[2] }
}

function formatDateValue(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

function formatDisplayDate(date: Date) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(date)
}
