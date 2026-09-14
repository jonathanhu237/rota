import { describe, expect, it } from 'vitest'
import { createPublicationSchema } from './components/publications/publication-schemas'
import { createGiveDirectSchema, createSwapSchema } from './components/shift-changes/shift-change-schemas'
import { createTemplateSchema, createTemplateSlotPositionSchema, createTemplateSlotSchema } from './components/templates/template-schemas'

const t = (key: string) => key

describe('Rota form validation', () => {
  it('accepts a valid publication window and rejects out-of-order dates', () => {
    const schema = createPublicationSchema(t)
    const valid = schema.safeParse({
      template_id: 2,
      name: 'September rota',
      submission_start_at: '2026-09-01T09:00:00Z',
      submission_end_at: '2026-09-05T09:00:00Z',
      planned_active_from: '2026-09-07T09:00:00Z',
      planned_active_until: '2026-09-30T09:00:00Z',
    })
    expect(valid.success).toBe(true)

    const invalid = schema.safeParse({
      template_id: 2,
      name: 'September rota',
      submission_start_at: '2026-09-10T09:00:00Z',
      submission_end_at: '2026-09-05T09:00:00Z',
      planned_active_from: '2026-09-07T09:00:00Z',
      planned_active_until: '2026-09-30T09:00:00Z',
    })
    expect(invalid.success).toBe(false)
    if (!invalid.success) expect(invalid.error.issues[0]?.message).toBe('publications.validation.invalidWindow')
  })

  it('rejects malformed template slots and invalid attendance headcount', () => {
    const slot = createTemplateSlotSchema(t)
    expect(slot.safeParse({ weekdays: [1, 7], start_time: '09:00', end_time: '17:00' }).success).toBe(true)
    expect(slot.safeParse({ weekdays: [0], start_time: '09:00', end_time: '17:00' }).success).toBe(false)
    expect(slot.safeParse({ weekdays: [1], start_time: '17:00', end_time: '09:00' }).success).toBe(false)

    const position = createTemplateSlotPositionSchema(t)
    expect(position.safeParse({ position_id: 3, required_headcount: 1, attendance_responsible: true }).success).toBe(true)
    expect(position.safeParse({ position_id: 3, required_headcount: 2, attendance_responsible: true }).success).toBe(false)

    const template = createTemplateSchema(t)
    expect(template.safeParse({ name: 'Weekly', description: '' }).success).toBe(true)
    expect(template.safeParse({ name: ' ', description: '' }).success).toBe(false)
  })

  it('requires the counterpart details for direct and swap requests', () => {
    const direct = createGiveDirectSchema(t)
    expect(direct.safeParse({ counterpart_user_id: '019535d9-3df7-79fb-b466-fa907fa17f90' }).success).toBe(true)
    expect(direct.safeParse({ counterpart_user_id: '' }).success).toBe(false)

    const swap = createSwapSchema(t)
    expect(swap.safeParse({ counterpart_user_id: '019535d9-3df7-79fb-b466-fa907fa17f90', counterpart_assignment_id: 21 }).success).toBe(true)
    expect(swap.safeParse({ counterpart_user_id: '019535d9-3df7-79fb-b466-fa907fa17f90', counterpart_assignment_id: 0 }).success).toBe(false)
  })
})
