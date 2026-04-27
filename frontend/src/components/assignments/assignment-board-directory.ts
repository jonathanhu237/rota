import type { AssignmentBoardEmployee } from "@/lib/types"

export type Employee = {
  user_id: number
  name: string
  email: string
  position_ids: Set<number>
}

export function deriveEmployeeDirectory(
  employees: AssignmentBoardEmployee[],
): Map<number, Employee> {
  const directory = new Map<number, Employee>()

  for (const employee of employees) {
    directory.set(employee.user_id, {
      user_id: employee.user_id,
      name: employee.name,
      email: employee.email,
      position_ids: new Set(employee.position_ids),
    })
  }

  return directory
}
