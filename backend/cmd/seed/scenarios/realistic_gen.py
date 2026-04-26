#!/usr/bin/env python3
"""Generate the realistic seed scenario from the local availability CSV.

Emits ``backend/cmd/seed/scenarios/realistic.go`` with anonymized identities,
deterministic archetype assignment, and per-``(slot, weekday)`` availability
inserts that match the post-decouple-weekday-from-slot schema.

Run from anywhere: ``python3 backend/cmd/seed/scenarios/realistic_gen.py``.
The script reads ``processed.local.csv`` from the project root (gitignored)
and overwrites the sibling ``realistic.go`` file.
"""

from __future__ import annotations

import csv
import json
import random
import subprocess
from dataclasses import dataclass
from pathlib import Path


ASSIGNMENT_SEED = 20260427
EXPECTED_EMPLOYEES = 42
DROPPED_COLUMNS = {"NetID", "姓名", "邮箱", "角色"}
SOURCE_TIME_SLOT_HEADERS = [
    "09：00-10：00",
    "10：00-12：00",
    "13：30-16：10",
    "16：10-18：00",
    "19：00-21：00",
]
TARGET_ARCHETYPES = [
    ("FrontSenior", 10),
    ("FrontJunior", 22),
    ("FieldSenior", 4),
    ("FieldJunior", 6),
]
FRONT_ARCHETYPES = {"FrontSenior", "FrontJunior"}
SENIOR_ARCHETYPES = {
    "front": "FrontSenior",
    "field": "FieldSenior",
}


@dataclass(frozen=True)
class SourceEmployee:
    weekdays: tuple[tuple[int, ...], ...]


def main() -> None:
    script_path = Path(__file__).resolve()
    project_root = script_path.parents[4]
    source_path = project_root / "processed.local.csv"
    output_path = script_path.parent / "realistic.go"

    employees = read_source_employees(source_path)
    seed, archetypes = assign_archetypes(employees)
    inserted, dropped = count_domain_filtered_submissions(employees, archetypes)
    output_path.write_text(
        render_go(employees, archetypes, seed, inserted, dropped),
        encoding="utf-8",
    )
    subprocess.run(["gofmt", "-w", str(output_path)], check=True)

    print(f"generated {output_path.relative_to(project_root)}")
    print(f"assignment seed: {seed}")
    print(f"availability submissions after domain filter: {inserted}")
    print(f"domain-filtered source submissions dropped: {dropped}")


def read_source_employees(source_path: Path) -> list[SourceEmployee]:
    if not source_path.exists():
        raise FileNotFoundError(f"missing source CSV: {source_path}")

    with source_path.open(newline="", encoding="utf-8-sig") as csv_file:
        reader = csv.DictReader(csv_file)
        if reader.fieldnames is None:
            raise ValueError("source CSV has no header row")

        missing = sorted(DROPPED_COLUMNS.difference(reader.fieldnames))
        if missing:
            raise ValueError(f"source CSV missing identifying columns: {missing}")

        remaining_headers = [header for header in reader.fieldnames if header not in DROPPED_COLUMNS]
        if remaining_headers != SOURCE_TIME_SLOT_HEADERS:
            raise ValueError(
                "source CSV weekday columns changed: "
                f"got {remaining_headers}, want {SOURCE_TIME_SLOT_HEADERS}"
            )

        employees: list[SourceEmployee] = []
        for row_index, row in enumerate(reader, start=2):
            employees.append(
                SourceEmployee(
                    weekdays=tuple(
                        parse_weekdays(row[header], row_index, header)
                        for header in SOURCE_TIME_SLOT_HEADERS
                    )
                )
            )

    if len(employees) != EXPECTED_EMPLOYEES:
        raise ValueError(f"expected {EXPECTED_EMPLOYEES} employees, got {len(employees)}")

    return employees


def parse_weekdays(raw: str, row_index: int, header: str) -> tuple[int, ...]:
    raw = raw.strip()
    if raw == "":
        return ()

    weekdays: list[int] = []
    seen: set[int] = set()
    for part in raw.split(","):
        token = part.strip()
        if token == "":
            continue
        try:
            weekday = int(token)
        except ValueError as exc:
            raise ValueError(f"row {row_index} column {header}: invalid weekday {token!r}") from exc
        if weekday < 1 or weekday > 7:
            raise ValueError(f"row {row_index} column {header}: weekday {weekday} out of range")
        if weekday not in seen:
            weekdays.append(weekday)
            seen.add(weekday)

    return tuple(weekdays)


def assign_archetypes(employees: list[SourceEmployee]) -> tuple[int, list[str]]:
    seed = ASSIGNMENT_SEED
    source_coverage = {
        "front": source_weekday_coverage(employees, range(4)),
        "field": source_weekday_coverage(employees, range(4, 5)),
    }

    for _ in range(10_000):
        assignments = [
            archetype
            for archetype, count in TARGET_ARCHETYPES
            for _ in range(count)
        ]
        rng = random.Random(seed)
        rng.shuffle(assignments)

        if senior_coverage_ok(employees, assignments, source_coverage):
            return seed, assignments

        seed += 1

    raise RuntimeError("could not assign archetypes satisfying senior coverage")


def source_weekday_coverage(employees: list[SourceEmployee], slot_indexes: range) -> set[int]:
    return {
        weekday
        for employee in employees
        for slot_index in slot_indexes
        for weekday in employee.weekdays[slot_index]
    }


def senior_coverage_ok(
    employees: list[SourceEmployee],
    assignments: list[str],
    source_coverage: dict[str, set[int]],
) -> bool:
    for domain, senior_archetype in SENIOR_ARCHETYPES.items():
        slot_indexes = range(4) if domain == "front" else range(4, 5)
        covered = {
            weekday
            for employee, archetype in zip(employees, assignments)
            if archetype == senior_archetype
            for slot_index in slot_indexes
            for weekday in employee.weekdays[slot_index]
        }
        if not source_coverage[domain].issubset(covered):
            return False
    return True


def count_domain_filtered_submissions(
    employees: list[SourceEmployee],
    archetypes: list[str],
) -> tuple[int, int]:
    inserted = 0
    dropped = 0
    for employee, archetype in zip(employees, archetypes):
        for slot_index, weekdays in enumerate(employee.weekdays):
            for _ in weekdays:
                if slot_matches_domain(slot_index, archetype):
                    inserted += 1
                else:
                    dropped += 1
    return inserted, dropped


def slot_matches_domain(slot_index: int, archetype: str) -> bool:
    is_front_slot = slot_index < 4
    is_front_employee = archetype in FRONT_ARCHETYPES
    return is_front_slot == is_front_employee


def render_go(
    employees: list[SourceEmployee],
    archetypes: list[str],
    assignment_seed: int,
    inserted_submissions: int,
    dropped_submissions: int,
) -> str:
    time_slots = [normalize_time_slot_header(header) for header in SOURCE_TIME_SLOT_HEADERS]
    employee_literals = "\n".join(
        render_employee_literal(index, employee, archetype)
        for index, (employee, archetype) in enumerate(zip(employees, archetypes), start=1)
    )
    time_slot_literals = "\n".join(
        f"\t{{Start: {go_string(start)}, End: {go_string(end)}}},"
        for start, end in time_slots
    )

    return f"""// Code generated by backend/cmd/seed/scenarios/realistic_gen.py; DO NOT EDIT.

package scenarios

import (
\t"context"
\t"database/sql"
\t"fmt"
\t"time"

\tseedinternal "github.com/jonathanhu237/rota/backend/cmd/seed/internal"
\t"github.com/jonathanhu237/rota/backend/internal/model"
)

const (
\trealisticAssignmentSeed                = {assignment_seed}
\trealisticExpectedAvailabilitySubmits   = {inserted_submissions}
\trealisticDomainFilteredSubmissionDrops = {dropped_submissions}
)

const (
\trealisticFrontLeadPosition = iota
\trealisticFrontAssistantPosition
\trealisticFieldLeadPosition
\trealisticFieldAssistantPosition
)

type realisticEmployee struct {{
\tSlug     string
\tDisplay  string
\tEmail    string
\tArch     archetype
\tWeekdays [5][]int
}}

type archetype int

const (
\tFrontSenior archetype = iota
\tFrontJunior
\tFieldSenior
\tFieldJunior
)

var realisticTimeSlots = [5]struct {{
\tStart string
\tEnd   string
}}{{
{time_slot_literals}
}}

var realisticEmployees = [42]realisticEmployee{{
{employee_literals}
}}

func RunRealistic(ctx context.Context, tx *sql.Tx, opts Options) error {{
\tif _, _, err := insertUsers(ctx, tx, opts, 0); err != nil {{
\t\treturn err
\t}}

\tpositionIDs, err := insertRealisticPositions(ctx, tx, opts.Now)
\tif err != nil {{
\t\treturn err
\t}}

\tuserIDs, err := insertRealisticEmployees(ctx, tx)
\tif err != nil {{
\t\treturn err
\t}}
\tif err := insertRealisticUserPositions(ctx, tx, userIDs, positionIDs); err != nil {{
\t\treturn err
\t}}

\ttemplateID, err := insertTemplate(ctx, tx, "Realistic Rota", true, opts.Now)
\tif err != nil {{
\t\treturn err
\t}}
\tslotIDs, err := insertRealisticSlots(ctx, tx, templateID, positionIDs, opts.Now)
\tif err != nil {{
\t\treturn err
\t}}

\tactiveFrom := opts.Now.Add(7 * 24 * time.Hour)
\tpublicationID, err := insertPublication(
\t\tctx,
\t\ttx,
\t\ttemplateID,
\t\t"Realistic Rota Week",
\t\tmodel.PublicationStateDraft,
\t\topts.Now.Add(-14*24*time.Hour),
\t\topts.Now.Add(-7*24*time.Hour),
\t\tactiveFrom,
\t\tactiveFrom.Add(7*24*time.Hour),
\t\tnil,
\t\topts.Now,
\t)
\tif err != nil {{
\t\treturn err
\t}}

\treturn insertRealisticAvailabilitySubmissions(ctx, tx, publicationID, userIDs, slotIDs, opts.Now)
}}

func insertRealisticPositions(ctx context.Context, tx *sql.Tx, now time.Time) ([4]int64, error) {{
\tdefinitions := [4]struct {{
\t\tName        string
\t\tDescription string
\t}}{{
\t\t{{Name: "前台负责人", Description: "Realistic front desk lead"}},
\t\t{{Name: "前台助理", Description: "Realistic front desk assistant"}},
\t\t{{Name: "外勤负责人", Description: "Realistic field lead"}},
\t\t{{Name: "外勤助理", Description: "Realistic field assistant"}},
\t}}

\tvar ids [4]int64
\tfor index, definition := range definitions {{
\t\tif err := tx.QueryRowContext(
\t\t\tctx,
\t\t\t`
\t\t\t\tINSERT INTO positions (name, description, created_at, updated_at)
\t\t\t\tVALUES ($1, $2, $3, $3)
\t\t\t\tRETURNING id;
\t\t\t`,
\t\t\tdefinition.Name,
\t\t\tdefinition.Description,
\t\t\tnow,
\t\t).Scan(&ids[index]); err != nil {{
\t\t\treturn ids, fmt.Errorf("insert realistic position %q: %w", definition.Name, err)
\t\t}}
\t}}
\treturn ids, nil
}}

func insertRealisticEmployees(ctx context.Context, tx *sql.Tx) ([42]int64, error) {{
\tvar ids [42]int64
\tfor index, employee := range realisticEmployees {{
\t\tid, err := seedinternal.InsertUser(ctx, tx, employee.Email, employee.Display, SeedPassword, false)
\t\tif err != nil {{
\t\t\treturn ids, fmt.Errorf("insert realistic employee %s: %w", employee.Slug, err)
\t\t}}
\t\tids[index] = id
\t}}
\treturn ids, nil
}}

func insertRealisticUserPositions(ctx context.Context, tx *sql.Tx, userIDs [42]int64, positionIDs [4]int64) error {{
\tfor employeeIndex, employee := range realisticEmployees {{
\t\tindexes, err := employee.Arch.positionIndexes()
\t\tif err != nil {{
\t\t\treturn fmt.Errorf("resolve positions for %s: %w", employee.Slug, err)
\t\t}}
\t\tfor _, positionIndex := range indexes {{
\t\t\tif _, err := tx.ExecContext(
\t\t\t\tctx,
\t\t\t\t`INSERT INTO user_positions (user_id, position_id) VALUES ($1, $2);`,
\t\t\t\tuserIDs[employeeIndex],
\t\t\t\tpositionIDs[positionIndex],
\t\t\t); err != nil {{
\t\t\t\treturn fmt.Errorf("qualify realistic employee %s for position index %d: %w", employee.Slug, positionIndex, err)
\t\t\t}}
\t\t}}
\t}}
\treturn nil
}}

func insertRealisticSlots(ctx context.Context, tx *sql.Tx, templateID int64, positionIDs [4]int64, now time.Time) ([5][8]int64, error) {{
\tvar slotIDs [5][8]int64
\tfor timeIndex, slot := range realisticTimeSlots {{
\t\tvar slotID int64
\t\tif err := tx.QueryRowContext(
\t\t\tctx,
\t\t\t`
\t\t\t\tINSERT INTO template_slots (template_id, start_time, end_time, created_at, updated_at)
\t\t\t\tVALUES ($1, $2, $3, $4, $4)
\t\t\t\tRETURNING id;
\t\t\t`,
\t\t\ttemplateID,
\t\t\tslot.Start,
\t\t\tslot.End,
\t\t\tnow,
\t\t).Scan(&slotID); err != nil {{
\t\t\treturn slotIDs, fmt.Errorf("insert realistic slot %s-%s: %w", slot.Start, slot.End, err)
\t\t}}

\t\tfor weekday := 1; weekday <= 7; weekday++ {{
\t\t\tif _, err := tx.ExecContext(
\t\t\t\tctx,
\t\t\t\t`INSERT INTO template_slot_weekdays (slot_id, weekday) VALUES ($1, $2);`,
\t\t\t\tslotID,
\t\t\t\tweekday,
\t\t\t); err != nil {{
\t\t\t\treturn slotIDs, fmt.Errorf("insert realistic slot-weekday slot=%d weekday=%d: %w", slotID, weekday, err)
\t\t\t}}
\t\t\tslotIDs[timeIndex][weekday] = slotID
\t\t}}

\t\tpositions := realisticSlotPositions(timeIndex)
\t\tfor _, position := range positions {{
\t\t\tif _, err := tx.ExecContext(
\t\t\t\tctx,
\t\t\t\t`
\t\t\t\t\tINSERT INTO template_slot_positions (slot_id, position_id, required_headcount, created_at, updated_at)
\t\t\t\t\tVALUES ($1, $2, $3, $4, $4);
\t\t\t\t`,
\t\t\t\tslotID,
\t\t\t\tpositionIDs[position.PositionIndex],
\t\t\t\tposition.RequiredHeadcount,
\t\t\t\tnow,
\t\t\t); err != nil {{
\t\t\t\treturn slotIDs, fmt.Errorf("insert realistic slot-position slot=%d position=%d: %w", slotID, position.PositionIndex, err)
\t\t\t}}
\t\t}}
\t}}
\treturn slotIDs, nil
}}

func realisticSlotPositions(timeIndex int) []positionHeadcount {{
\tif timeIndex < 4 {{
\t\treturn []positionHeadcount{{
\t\t\t{{PositionIndex: realisticFrontLeadPosition, RequiredHeadcount: 1}},
\t\t\t{{PositionIndex: realisticFrontAssistantPosition, RequiredHeadcount: 2}},
\t\t}}
\t}}
\treturn []positionHeadcount{{
\t\t{{PositionIndex: realisticFieldLeadPosition, RequiredHeadcount: 1}},
\t\t{{PositionIndex: realisticFieldAssistantPosition, RequiredHeadcount: 1}},
\t}}
}}

func insertRealisticAvailabilitySubmissions(
\tctx context.Context,
\ttx *sql.Tx,
\tpublicationID int64,
\tuserIDs [42]int64,
\tslotIDs [5][8]int64,
\tnow time.Time,
) error {{
\tinserted := 0
\tfor employeeIndex, employee := range realisticEmployees {{
\t\tfor timeIndex, weekdays := range employee.Weekdays {{
\t\t\tif !employee.Arch.matchesTimeSlot(timeIndex) {{
\t\t\t\tcontinue
\t\t\t}}
\t\t\tfor _, weekday := range weekdays {{
\t\t\t\tif weekday < 1 || weekday > 7 {{
\t\t\t\t\treturn fmt.Errorf("realistic employee %s has invalid weekday %d", employee.Slug, weekday)
\t\t\t\t}}
\t\t\t\tslotID := slotIDs[timeIndex][weekday]
\t\t\t\tif slotID == 0 {{
\t\t\t\t\treturn fmt.Errorf("missing realistic slot id for time index %d weekday %d", timeIndex, weekday)
\t\t\t\t}}
\t\t\t\tif _, err := tx.ExecContext(
\t\t\t\t\tctx,
\t\t\t\t\t`
\t\t\t\t\t\tINSERT INTO availability_submissions (
\t\t\t\t\t\t\tpublication_id,
\t\t\t\t\t\t\tuser_id,
\t\t\t\t\t\t\tslot_id,
\t\t\t\t\t\t\tweekday,
\t\t\t\t\t\t\tcreated_at
\t\t\t\t\t\t)
\t\t\t\t\t\tVALUES ($1, $2, $3, $4, $5);
\t\t\t\t\t`,
\t\t\t\t\tpublicationID,
\t\t\t\t\tuserIDs[employeeIndex],
\t\t\t\t\tslotID,
\t\t\t\t\tweekday,
\t\t\t\t\tnow,
\t\t\t\t); err != nil {{
\t\t\t\t\treturn fmt.Errorf("insert realistic availability employee=%s slot=%d weekday=%d: %w", employee.Slug, slotID, weekday, err)
\t\t\t\t}}
\t\t\t\tinserted++
\t\t\t}}
\t\t}}
\t}}
\tif inserted != realisticExpectedAvailabilitySubmits {{
\t\treturn fmt.Errorf("realistic availability count mismatch: inserted %d, expected %d", inserted, realisticExpectedAvailabilitySubmits)
\t}}
\treturn nil
}}

func (arch archetype) positionIndexes() ([]int, error) {{
\tswitch arch {{
\tcase FrontSenior:
\t\treturn []int{{realisticFrontLeadPosition, realisticFrontAssistantPosition}}, nil
\tcase FrontJunior:
\t\treturn []int{{realisticFrontAssistantPosition}}, nil
\tcase FieldSenior:
\t\treturn []int{{realisticFieldLeadPosition, realisticFieldAssistantPosition}}, nil
\tcase FieldJunior:
\t\treturn []int{{realisticFieldAssistantPosition}}, nil
\tdefault:
\t\treturn nil, fmt.Errorf("unknown archetype %d", arch)
\t}}
}}

func (arch archetype) matchesTimeSlot(timeIndex int) bool {{
\tswitch arch {{
\tcase FrontSenior, FrontJunior:
\t\treturn timeIndex < 4
\tcase FieldSenior, FieldJunior:
\t\treturn timeIndex == 4
\tdefault:
\t\treturn false
\t}}
}}
"""


def normalize_time_slot_header(header: str) -> tuple[str, str]:
    if "：" not in header:
        raise ValueError(f"expected full-width colon in time-slot header {header!r}")
    parts = header.replace("：", ":").split("-")
    if len(parts) != 2:
        raise ValueError(f"invalid time-slot header {header!r}")
    return parts[0], parts[1]


def render_employee_literal(index: int, employee: SourceEmployee, archetype: str) -> str:
    slug = f"employee{index:02d}"
    weekdays = "\n".join(
        f"\t\t\t{go_int_slice(values)},"
        for values in employee.weekdays
    )
    return f"""\t{{
\t\tSlug:    {go_string(slug)},
\t\tDisplay: {go_string(f"员工 {index}")},
\t\tEmail:   {go_string(f"{slug}@example.com")},
\t\tArch:    {archetype},
\t\tWeekdays: [5][]int{{
{weekdays}
\t\t}},
\t}},"""


def go_int_slice(values: tuple[int, ...]) -> str:
    if not values:
        return "[]int{}"
    return "[]int{" + ", ".join(str(value) for value in values) + "}"


def go_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


if __name__ == "__main__":
    main()
