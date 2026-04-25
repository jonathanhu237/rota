---
name: openspec-archive-change
description: Archive a completed change in the experimental workflow. Use when the user wants to finalize and archive a change after implementation is complete.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.3.1"
---

Archive a completed change in the experimental workflow.

**Input**: Optionally specify a change name. If omitted, check if it can be inferred from conversation context. If vague or ambiguous you MUST prompt for available changes.

**Steps**

1. **If no change name provided, prompt for selection**

   Run `openspec list --json` to get available changes. Use the **AskUserQuestion tool** to let the user select.

   Show only active changes (not already archived).
   Include the schema used for each change if available.

   **IMPORTANT**: Do NOT guess or auto-select a change. Always let the user choose.

2. **Check artifact completion status**

   Run `openspec status --change "<name>" --json` to check artifact completion.

   Parse the JSON to understand:
   - `schemaName`: The workflow being used
   - `artifacts`: List of artifacts with their status (`done` or other)

   **If any artifacts are not `done`:**
   - Display warning listing incomplete artifacts
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

3. **Check task completion status**

   Read the tasks file (typically `tasks.md`) to check for incomplete tasks.

   Count tasks marked with `- [ ]` (incomplete) vs `- [x]` (complete).

   **If incomplete tasks found:**
   - Display warning showing count of incomplete tasks
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

   **If no tasks file exists:** Proceed without task-related warning.

4. **Assess delta spec sync state**

   Check for delta specs at `openspec/changes/<name>/specs/`. If none exist, proceed without sync prompt.

   **If delta specs exist:**
   - Compare each delta spec with its corresponding main spec at `openspec/specs/<capability>/spec.md`
   - Determine what changes would be applied (adds, modifications, removals, renames)
   - Show a combined summary before prompting

   **Prompt options:**
   - If changes needed: "Sync now (recommended)", "Archive without syncing"
   - If already synced: "Archive now", "Sync anyway", "Cancel"

   If user chooses sync, use Task tool (subagent_type: "general-purpose", prompt: "Use Skill tool to invoke openspec-sync-specs for change '<name>'. Delta spec analysis: <include the analyzed delta spec summary>"). Proceed to archive regardless of choice.

5. **Perform the archive**

   Create the archive directory if it doesn't exist:
   ```bash
   mkdir -p openspec/changes/archive
   ```

   Generate target name using current date: `YYYY-MM-DD-<change-name>`

   **Check if target already exists:**
   - If yes: Fail with error, suggest renaming existing archive or using different date
   - If no: Move the change directory to archive

   ```bash
   mv openspec/changes/<name> openspec/changes/archive/YYYY-MM-DD-<name>
   ```

6. **Commit the archived change on the current branch (rota project rule).**

   Compose a Conventional Commits message that reflects the change (`feat`, `fix`, `refactor`, `chore`, `docs`) with a meaningful body summarizing what changed. Stage only paths the change touched — typically some subset of `backend/`, `frontend/`, `migrations/`, `openspec/specs/`, and the new `openspec/changes/archive/YYYY-MM-DD-<change-name>/` directory. Use the project's `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>` trailer.

   Commit. Do NOT push yet — pushing happens after we know whether we're on a feature branch or `main`.

7. **If on a `change/*` branch: merge to main and clean up the worktree (rota project rule).**

   ```bash
   current="$(git rev-parse --abbrev-ref HEAD)"
   ```

   - **If `current` is `main`** (legacy serial flow): `git push origin main` and stop. Skip the rest of this step.

   - **If `current` matches `change/*`**:

     1. Push the feature branch first so CI runs on it (optional but lets us catch failures before merging):
        ```bash
        git push origin "$current"
        ```

     2. Find the main checkout (the worktree on branch `main`):
        ```bash
        main_path="$(git worktree list --porcelain | awk '/^worktree / {p=$2} /^branch refs\/heads\/main$/ {print p; exit}')"
        ```

     3. Merge into `main` from the main checkout, with a merge commit (no fast-forward) so the branch's history is preserved:
        ```bash
        git -C "$main_path" merge --no-ff "$current" -m "Merge branch '$current' into main"
        git -C "$main_path" push origin main
        ```

     4. Wait for CI on `main` to complete (the user has a Stop-hook notification configured; `gh run watch <id>` is fine, or use a polling background command). Report the result.

     5. Delete the feature branch and remove the worktree:
        ```bash
        # The worktree is the directory the archive was performed from.
        worktree_path="$(git rev-parse --show-toplevel)"
        git -C "$main_path" worktree remove "$worktree_path"
        git -C "$main_path" branch -d "$current"
        # Best-effort delete the remote branch too:
        git -C "$main_path" push origin --delete "$current" 2>/dev/null || true
        ```

     6. Switch the working directory of subsequent shell commands to `$main_path` (the worktree we were in is gone).

8. **Display summary** including the merge result.

**Output On Success (with merge)**

```
## Archive Complete + Merged

**Change:** <change-name>
**Schema:** <schema-name>
**Archived to:** openspec/changes/archive/YYYY-MM-DD-<name>/
**Specs:** ✓ Synced to main specs
**Branch:** change/<change-name> merged into main and removed
**Worktree:** ../<repo>-<change-name>/ removed
**CI on main:** ✓ green (or ✗ failed — investigate)

All artifacts complete. All tasks complete.
```

**Output On Success (legacy serial, no branch)**

```
## Archive Complete

**Change:** <change-name>
**Schema:** <schema-name>
**Archived to:** openspec/changes/archive/YYYY-MM-DD-<name>/
**Specs:** ✓ Synced to main specs
**Branch:** stayed on main (no feature branch)

All artifacts complete. All tasks complete.
```

**Guardrails**
- Always prompt for change selection if not provided
- Use artifact graph (openspec status --json) for completion checking
- Don't block archive on warnings - just inform and confirm
- Preserve .openspec.yaml when moving to archive (it moves with the directory)
- Show clear summary of what happened
- If sync is requested, use openspec-sync-specs approach (agent-driven)
- If delta specs exist, always run the sync assessment and show the combined summary before prompting
- The merge step (step 7) is rota-project-specific. Other projects using this skill template can drop it.
