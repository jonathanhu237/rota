# CI workflow syntax follow-up

Branch: `temvia-rota-rebuild`.
Failing commit: `f6ba431ca74aa10007b70363671167a136d17b4a`.
GitHub run: https://github.com/jonathanhu237/rota/actions/runs/34796609659

## Diagnosis

GitHub marks the workflow as failed with an empty jobs list; no product test job started. The last `run` field combined a YAML double-quoted scalar with a trailing shell argument:

```yaml
run: "${RUNNER_TEMP}/bin/govulncheck" ./...
```

Centaurus `mise exec --raw actionlint@1.7.12 -- actionlint -`, given the local workflow through stdin, reproduced `could not parse as YAML: did not find expected key`. The reported location precedes the malformed final field; the defect is the scalar syntax, not the Go installer command.

## Minimal correction and verification

Use a literal block so the shell receives the same intended quoted executable and argument:

```yaml
run: |
  "${RUNNER_TEMP}/bin/govulncheck" ./...
```

The corrected local file passed the same Centaurus actionlint command with exit 0 and no diagnostics, and `git diff --check` passed. No application code, tests, permissions, dependency versions, or CI gates were changed. This corrects a workflow-validation gap in the prior verification; source tests did not validate GitHub's YAML parser.

A new GitHub run is required after pushing; local/remote lint success is not a claim that the GitHub jobs have passed.
