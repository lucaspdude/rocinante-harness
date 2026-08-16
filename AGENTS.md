# AGENTS.md — workflow for contributors (human or agent)

This file is the canonical contributor workflow for the
`rocinante-harness` monorepo. It exists so that any agent (or
new contributor) picking up the project follows the same loop.

## Layout

```
apps/
  api/                 Go HTTP service (chi router + SSE + SQLite)
  web/                 Next.js 16 front-end
  harness/             roc-harness umbrella CLI launcher
packages/
  contract/            Shared TS endpoint + type definitions
installer/
  install.sh           curl | sh bootstrap
docs/
  deploy/              Caddyfile, Cloudflare Tunnel, security checklist
  adr/                 Architecture decision records
```

## Branch + PR workflow (mandatory)

Every change — bug fix, feature, refactor, CI tweak — goes through
a Pull Request against `main`. Direct pushes to `main` are not
allowed.

1. `git checkout main && git pull --ff-only origin main`
2. `git checkout -b <type>/<scope>-<slug>` (e.g. `feat/api-login`,
   `fix/installer-symlinks`, `chore/ci-pnpm-version`).
3. Implement + commit with Conventional Commits + scopes:
   `feat(api)`, `fix(web)`, `chore(deps)`, `docs(adr)`, etc.
4. Push the branch: `git push -u origin <branch>`.
5. Open a PR with `gh pr create`. The PR must reference the
   phase doc or the bug it addresses and copy the acceptance
   criteria as a checklist.
6. Wait for CI to pass. If CI fails, **fix the branch in place**
   (additional commit, or amend) — do not open a new PR.
7. Merge with `gh pr merge --squash --delete-branch`. Squash
   keeps `git log --first-parent main` aligned with the phase plan.
8. The `release-on-merge.yml` workflow watches `main` and bumps
   the version (`feat:` → minor, `feat!:` → major, anything else →
   patch), creates a tag, and dispatches `release.yml` which
   builds + attaches binaries to the GitHub Release.

**Never edit a file directly on `main`.** No matter how small the
change, it goes through a branch + PR.

## Git identity (do not commit as `Lucas <lucas@example.com>`)

This repo's `.git/config` **must not** carry a local `[user]`
override. The previous override (`name = Lucas`,
`email = lucas@example.com`) caused every commit to author as
the example address and forced GitHub to add a
`Co-authored-by: Lucas <lucas@example.com>` trailer. The global
config at `~/.gitconfig` is the canonical identity:

- `user.name = Lucas Pacheco`
- `user.email = lucaspdude@gmail.com`

If `git config --local --get user.email` ever returns
`lucas@example.com` (or any non-canonical value), fix it before
the next commit:

```bash
git config --local --unset user.name
git config --local --unset user.email
git config --get user.email   # must print lucaspdude@gmail.com
```

Do **not** add a per-repo `[user]` override when committing. The
global config is correct and per-repo overrides here are
considered a regression.

## Path-prefix invariant

Every public endpoint the front consumes is `/api/v1/...`. The
canonical list lives in `packages/contract/src/endpoints.ts`. Go
code reuses the same prefix via `apps/api/internal/api/router.go`.
If a new endpoint doesn't start with `/api/v1/`, the Phase review
flags it as a blocker.

## CI gate

`scripts/ci.sh` is the merge gate. It runs:

- `pnpm install --frozen-lockfile`
- `pnpm turbo run typecheck`, `lint`, `test`
- `go vet` + `go test -race` for each Go pack
- `next build` for the web
- `go build` for the api and harness binaries

Smoke tests (the per-phase `scripts/smoke-phase-NN.sh`) are gated
behind `ROCINANTE_SMOKE=1`. The CI runner does not have the `omp`
binary installed, so the default is off. Run smokes locally
after `apt install`ing omp or pointing `--omp-bin` at a local copy.

## Build artifacts

`scripts/build-harness.sh` cross-compiles the harness for
linux+darwin × amd64+arm64 + windows amd64. The output binaries
go to `apps/harness/bin/`. The api is built similarly by the
release workflow across the same matrix.

## Installer contract

`installer/install.sh` honours the following env vars (any
prefix `ROCHASSEN_*` is a legacy alias for the matching
`ROCINANTE_*`):

| Variable | Effect |
|---|---|
| `ROCINANTE_VERSION` | Pin to a specific release tag (default: latest) |
| `ROCINANTE_SKIP_INIT=1` | Skip the `api init` prompt |
| `ROCINANTE_SKIP_OMP=1` | Skip the best-effort omp install |
| `ROCINANTE_INSTALL_SERVICE=1` | Write + enable systemd units for the api |
| `ROCINANTE_REPO=owner/name` | Install from a fork |

The installer symlinks the platform-suffixed binaries to the
canonical names `api` and `roc-harness` so the rest of the
workflow never sees the GOOS/GOARCH tuple.

## Reporting defects

For issues that don't fit a fix-or-skip decision, add a file
under `docs/experiments/bugs-and-issues/<slug>.md` (template in
that folder's README) and link it from the PR description.
