<!--
  PR template of the monorepo `rocinante-harness`.
  Source of truth: docs/experiments/.github/pull_request_template.md
  (reference copy in the workspace docs).
-->

## Phase

- **Branch:** `feat/p<N>-<slug>`
- **Corresponding phase:** [`docs/experiments/phases/phase-N-slug.md`](../experiments/phases/phase-N-slug.md)
- **ADR gates expected:** [`docs/adr/0007-coding-standards.md`](../adr/0007-coding-standards.md)

## Summary

<2-4 lines: what this phase delivers.>

## Acceptance criteria (checklist from `phase-N-*.md`)

Literal copy of the "Critério de aceitação" section of the `phase-N-*.md`. Mark each item with `[x]` during implementation. Unchecked items cannot be merged.

<!--
Paste here:

- [ ] 1. ...
- [ ] 2. ...
-->

## Validation

Literal output **pasted** (not summarized) of:

```bash
bash ./scripts/ci.sh
```

`bash ./scripts/smoke-phase-<N>.sh` if the phase creates a smoke script.
Coverage targets (ADR 0007 §3) met in the touched packs.
Lint clean in all packs.

## Smoke runtime (if applicable)

Link smoke log (command + relevant output). For phases with
ssh-server (P12, P13) paste the handshake result.

## Known bugs

List the referenced bug-and-issues files
(`docs/experiments/bugs-and-issues/<slug>.md`). Each item with link
+ severity + workaround.

- (none if applicable)

## Process notes

- Branch will be deleted in the remote after the squash-merge (cleanup
  via the repo's branch protection).
- Conventional Commits: PR description uses the pack scopes
  (`api`, `web`, `harness`, `installer`, `contract`, `deps`,
  `ci`, `docs`) in the final commit title.
- PR diff total < 1500 LOC.

## Reviewer checklist

- [ ] Coherence with `phase-N-*.md` (no decisions outside of what was planned without opening a bug-and-issues).
- [ ] Lint + typecheck pass without warnings.
- [ ] Coverage meets the pack target (ADR 0007 §3).
- [ ] Phase smoke runs in < 5min without Docker.
- [ ] Path-prefix invariant (`/api/v1/...`) intact in all routes/endpoints created.
- [ ] No verbose comments; rationale links ADR/D-FRAME when applicable.
- [ ] No secrets/PII committed.
