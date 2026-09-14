import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '@/test/msw'
import {
  activatePublication,
  adminClearArrival,
  adminCreateOvertime,
  adminDeleteOvertime,
  adminUpdateOvertime,
  adminUpsertArrival,
  approveShiftChangeRequest,
  autoAssignPublication,
  cancelLeave,
  cancelShiftChangeRequest,
  createAssignment,
  createAvailabilitySubmission,
  createLeave,
  createPosition,
  createPublication,
  createShiftChangeRequest,
  createTemplate,
  createTemplateSlot,
  createTemplateSlotPosition,
  deleteAssignment,
  deleteAvailabilitySubmission,
  deletePosition,
  deletePublication,
  deleteTemplate,
  deleteTemplateSlot,
  deleteTemplateSlotPosition,
  endPublication,
  publishPublication,
  recordLeaderArrival,
  recordLeaderOvertime,
  rejectShiftChangeRequest,
  replaceAdminAvailability,
  replaceUserPositions,
  updateAttendanceSettings,
  updatePosition,
  updatePublication,
  updateTemplate,
  updateTemplateSlot,
  updateTemplateSlotPosition,
} from './queries'

const uuid = '019535d9-3df7-79fb-b466-fa907fa17f90'
const shiftInput = {
  slot_id: 11,
  assignment_id: 21,
  occurrence_date: '2026-09-14',
  user_id: uuid,
}

function responseBody() {
  return {
    position: { id: 3 },
    template: { id: 2 },
    slot: { id: 11 },
    publication: { id: 7 },
    shift: { slot_id: 11 },
    overtime: { id: 61 },
    request: { id: 41 },
    leave: { id: 31 },
    detail: { user: { id: uuid } },
  }
}

describe('Rota lifecycle API contracts', () => {
  it('preserves canonical routes for management, attendance, availability, leave, and request workflows', async () => {
    const requests: Array<{ method: string; path: string; body: unknown }> = []
    server.use(
      http.all(/\/api\/rota\/.*/, async ({ request }) => {
        let body: unknown
        try { body = await request.clone().json() } catch { body = undefined }
        requests.push({ method: request.method, path: new URL(request.url).pathname, body })
        return HttpResponse.json(responseBody())
      }),
    )

    await replaceUserPositions(uuid, [3, 1])
    await createPosition({ name: 'Nurse', description: '' })
    await updatePosition(3, { name: 'Senior Nurse', description: '' })
    await deletePosition(3)
    await createTemplate({ name: 'Weekly', description: '' })
    await updateTemplate(2, { name: 'Weekly v2', description: '' })
    await deleteTemplate(2)
    await createTemplateSlot(2, { weekdays: [1], start_time: '09:00', end_time: '10:00' })
    await updateTemplateSlot(2, 11, { weekdays: [1], start_time: '09:00', end_time: '10:00' })
    await deleteTemplateSlot(2, 11)
    await createTemplateSlotPosition(2, 11, { position_id: 3, required_headcount: 1, attendance_responsible: true })
    await updateTemplateSlotPosition(2, 11, 12, { position_id: 3, required_headcount: 1, attendance_responsible: true })
    await deleteTemplateSlotPosition(2, 11, 12)

    const dates = {
      template_id: 2,
      name: 'September rota',
      submission_start_at: '2026-09-01T09:00:00Z',
      submission_end_at: '2026-09-05T09:00:00Z',
      planned_active_from: '2026-09-07T09:00:00Z',
      planned_active_until: '2026-09-30T09:00:00Z',
    }
    await createPublication(dates)
    await updatePublication(7, { name: 'Updated rota' })
    await publishPublication(7)
    await activatePublication(7)
    await endPublication(7)
    await deletePublication(7)
    await createAssignment(7, { user_id: uuid, slot_id: 11, weekday: 1, position_id: 3 })
    await autoAssignPublication(7)
    await deleteAssignment(7, 21)
    await replaceAdminAvailability(7, uuid, [{ slot_id: 11, weekday: 1 }])
    await createAvailabilitySubmission(7, 11, 1)
    await deleteAvailabilitySubmission(7, 11, 1)

    await recordLeaderArrival({ publication_id: 7, slot_id: 11, assignment_id: 21, occurrence_date: '2026-09-14', user_id: uuid })
    await recordLeaderOvertime({ publication_id: 7, slot_id: 11, occurrence_date: '2026-09-14', user_id: uuid, hours: 1, note: 'handover' })
    await adminUpsertArrival(7, { ...shiftInput, arrived_at: '2026-09-14T09:05:00Z' })
    await adminClearArrival(7, 51)
    await adminCreateOvertime(7, { slot_id: 11, occurrence_date: '2026-09-14', user_id: uuid, hours: 1, note: 'handover' })
    await adminUpdateOvertime(7, 61, { hours: 2, note: 'extended' })
    await adminDeleteOvertime(7, 61)
    await updateAttendanceSettings(7, 24)

    await createShiftChangeRequest(7, { type: 'give_pool', requester_assignment_id: 21, occurrence_date: '2026-09-14' })
    await approveShiftChangeRequest(7, 41)
    await rejectShiftChangeRequest(7, 41)
    await cancelShiftChangeRequest(7, 41)
    await createLeave({ assignment_id: 21, occurrence_date: '2026-09-14', type: 'give_pool', category: 'sick' })
    await cancelLeave(31)

    expect(requests.map(({ method, path }) => `${method} ${path}`)).toEqual([
      `PUT /api/rota/users/${uuid}/positions`,
      'POST /api/rota/positions',
      'PUT /api/rota/positions/3',
      'DELETE /api/rota/positions/3',
      'POST /api/rota/templates',
      'PUT /api/rota/templates/2',
      'DELETE /api/rota/templates/2',
      'POST /api/rota/templates/2/slots',
      'PATCH /api/rota/templates/2/slots/11',
      'DELETE /api/rota/templates/2/slots/11',
      'POST /api/rota/templates/2/slots/11/positions',
      'PATCH /api/rota/templates/2/slots/11/positions/12',
      'DELETE /api/rota/templates/2/slots/11/positions/12',
      'POST /api/rota/publications',
      'PATCH /api/rota/publications/7',
      'POST /api/rota/publications/7/publish',
      'POST /api/rota/publications/7/activate',
      'POST /api/rota/publications/7/end',
      'DELETE /api/rota/publications/7',
      'POST /api/rota/publications/7/assignments',
      'POST /api/rota/publications/7/auto-assign',
      'DELETE /api/rota/publications/7/assignments/21',
      `PUT /api/rota/publications/7/availability-submissions/${uuid}`,
      'POST /api/rota/publications/7/submissions',
      'DELETE /api/rota/publications/7/submissions/11/1',
      'POST /api/rota/attendance/arrivals',
      'POST /api/rota/attendance/overtime',
      'PUT /api/rota/publications/7/attendance/arrivals',
      'DELETE /api/rota/publications/7/attendance/arrivals/51',
      'POST /api/rota/publications/7/attendance/overtime',
      'PATCH /api/rota/publications/7/attendance/overtime/61',
      'DELETE /api/rota/publications/7/attendance/overtime/61',
      'PATCH /api/rota/publications/7/attendance/settings',
      'POST /api/rota/publications/7/shift-changes',
      'POST /api/rota/publications/7/shift-changes/41/approve',
      'POST /api/rota/publications/7/shift-changes/41/reject',
      'POST /api/rota/publications/7/shift-changes/41/cancel',
      'POST /api/rota/leaves',
      'POST /api/rota/leaves/31/cancel',
    ])

    expect(requests[0].body).toEqual({ position_ids: [3, 1] })
    expect(requests[22].body).toEqual({ submissions: [{ slot_id: 11, weekday: 1 }] })
    expect(requests[29].body).toMatchObject({ publication_id: 7, slot_id: 11, hours: 1 })
  })
})
