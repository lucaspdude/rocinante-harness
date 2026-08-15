# Contributing

## Commit conventions

Conventional Commits are mandatory. `<scope>` is the pack name:

- `feat(api):`, `fix(api):`, `test(api):`, `refactor(api):`
- `feat(web):`, `fix(web):`, `test(web):`, `refactor(web):`
- `feat(harness):`, `fix(harness):`, `test(harness):`
- `feat(installer):`, `fix(installer):`
- `feat(contract):`
- `chore(deps):`, `chore(ci):`
- `docs(adr):`, `docs(experiments):`

PR with 1 phase = 1 commit (squash merge) by default. The squash commit
title follows the same convention.

## Pull requests

Each phase ships as 1 PR against `main`. The branch is `feat/p<N>-<slug>`.
Branch is deleted after merge.

See `docs/experiments/phases/README.md` for the full workflow.
