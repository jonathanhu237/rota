---
name: openspec-apply-change
description: Implement tasks from an OpenSpec change. Use when the user wants to start implementing, continue implementation, or work through tasks.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.3.1"
---

Implement tasks from an OpenSpec change.

**Input**: Optionally specify a change name. If omitted, check if it can be inferred from conversation context. If vague or ambiguous you MUST prompt for available changes.

**Steps**

1. **Select the change**

   If a name is provided, use it. Otherwise:
   - Infer from conversation context if the user mentioned a change
   - Auto-select if only one active change exists
   - If ambiguous, run `openspec list --json` to get available changes and use the **AskUserQuestion tool** to let the user select

   Always announce: "Using change: <name>" and how to override (e.g., `/opsx:apply <other>`).

1a. **Ensure the apply runs on a `change/<change-name>` worktree (rota project rule).**

   This project requires apply to run on a branch named `change/<change-name>`, not on `main`. Bootstrap the worktree yourself if needed, do not ask the user to do it manually.

   Detection step:

   ```bash
   expected="change/<change-name>"
   current="$(git rev-parse --abbrev-ref HEAD)"
   ```

   Branch the work: choose ONE of (a), (b), (c) based on `current` and the working tree state.

   - **(a) `current == expected`**: already on the right branch. Continue to Step 2.

   - **(b) `current == main` AND `git status --porcelain` is empty (or contains only the change's `openspec/changes/<change-name>/` artifacts)**: bootstrap a fresh worktree, then `cd` into it. After this block your cwd MUST be the new worktree; all later steps run there.

     ```bash
     repo_root="$(git rev-parse --show-toplevel)"
     repo_name="$(basename "$repo_root")"
     worktree_dir="${repo_root}/../${repo_name}-<change-name>"

     if [ -e "$worktree_dir" ]; then
       echo "✗ worktree path already exists: $worktree_dir"
       echo "  remove it first (git worktree remove $worktree_dir) or pick a different change name"
       exit 1
     fi

     git -C "$repo_root" worktree add "$worktree_dir" -b "$expected"
     # .env is gitignored; copy it forward best-effort so the new worktree can run docker compose / smoke tests
     cp "$repo_root/.env" "$worktree_dir/.env" 2>/dev/null || echo "  note: no .env to copy; you may need to populate $worktree_dir/.env before integration tests"
     cd "$worktree_dir"
     ```

     Announce to the user: `"Bootstrapped worktree at <worktree_dir> on branch <expected>. Continuing apply there."`

   - **(c) anything else** (e.g., `current` is some other branch, or the working tree has unrelated uncommitted changes): refuse and stop. Print:

     ```
     ✗ cannot bootstrap worktree from current state.
       current branch: <current>
       expected branch: change/<change-name>
       working tree status: <output of `git status -s | head -5`>

     Resolve manually before re-running. Common fixes:
       - if you're already in a worktree for a different change, cd to the main checkout (the one on branch main) and re-run codex exec there
       - if the main working tree has uncommitted unrelated changes, commit or stash them first
       - if you intended a serial in-place flow (no worktree), checkout the branch yourself: git checkout -b change/<change-name>

     See AGENTS.md "Apply runs on a feature branch" for the full rule.
     ```

     Exit. Do not continue.

2. **Check status to understand the schema**
   ```bash
   openspec status --change "<name>" --json
   ```
   Parse the JSON to understand:
   - `schemaName`: The workflow being used (e.g., "spec-driven")
   - Which artifact contains the tasks (typically "tasks" for spec-driven, check status for others)

3. **Get apply instructions**

   ```bash
   openspec instructions apply --change "<name>" --json
   ```

   This returns:
   - `contextFiles`: artifact ID -> array of concrete file paths (varies by schema - could be proposal/specs/design/tasks or spec/tests/implementation/docs)
   - Progress (total, complete, remaining)
   - Task list with status
   - Dynamic instruction based on current state

   **Handle states:**
   - If `state: "blocked"` (missing artifacts): show message, suggest using openspec-continue-change
   - If `state: "all_done"`: congratulate, suggest archive
   - Otherwise: proceed to implementation

4. **Read context files**

   Read every file path listed under `contextFiles` from the apply instructions output.
   The files depend on the schema being used:
   - **spec-driven**: proposal, specs, design, tasks
   - Other schemas: follow the contextFiles from CLI output

5. **Show current progress**

   Display:
   - Schema being used
   - Progress: "N/M tasks complete"
   - Remaining tasks overview
   - Dynamic instruction from CLI

6. **Implement tasks (loop until done or blocked)**

   For each pending task:
   - Show which task is being worked on
   - Make the code changes required
   - Keep changes minimal and focused
   - Mark task complete in the tasks file: `- [ ]` → `- [x]`
   - Continue to next task

   **Pause if:**
   - Task is unclear → ask for clarification
   - Implementation reveals a design issue → suggest updating artifacts
   - Error or blocker encountered → report and wait for guidance
   - User interrupts

7. **On completion or pause, show status**

   Display:
   - Tasks completed this session
   - Overall progress: "N/M tasks complete"
   - If all done: suggest archive
   - If paused: explain why and wait for guidance

**Output During Implementation**

```
## Implementing: <change-name> (schema: <schema-name>)

Working on task 3/7: <task description>
[...implementation happening...]
✓ Task complete

Working on task 4/7: <task description>
[...implementation happening...]
✓ Task complete
```

**Output On Completion**

```
## Implementation Complete

**Change:** <change-name>
**Schema:** <schema-name>
**Progress:** 7/7 tasks complete ✓

### Completed This Session
- [x] Task 1
- [x] Task 2
...

All tasks complete! Ready to archive this change.
```

**Output On Pause (Issue Encountered)**

```
## Implementation Paused

**Change:** <change-name>
**Schema:** <schema-name>
**Progress:** 4/7 tasks complete

### Issue Encountered
<description of the issue>

**Options:**
1. <option 1>
2. <option 2>
3. Other approach

What would you like to do?
```

**Guardrails**
- Keep going through tasks until done or blocked
- Always read context files before starting (from the apply instructions output)
- If task is ambiguous, pause and ask before implementing
- If implementation reveals issues, pause and suggest artifact updates
- Keep code changes minimal and scoped to each task
- Update task checkbox immediately after completing each task
- Pause on errors, blockers, or unclear requirements - don't guess
- Use contextFiles from CLI output, don't assume specific file names

**Fluid Workflow Integration**

This skill supports the "actions on a change" model:

- **Can be invoked anytime**: Before all artifacts are done (if tasks exist), after partial implementation, interleaved with other actions
- **Allows artifact updates**: If implementation reveals design issues, suggest updating artifacts - not phase-locked, work fluidly
