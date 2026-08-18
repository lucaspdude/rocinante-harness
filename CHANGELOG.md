# Changelog

All notable changes to `rocinante-harness` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] — 2026-08-18

### Added

- **Login-driven provider auth (PR #1):** 5 new public endpoints under
  `/api/v1/login/*` (`providers`, `start/{provider}`, `{jobId}/stream`,
  `{jobId}/ack`, `{jobId}/status`). `NewOMPLoginProviders` shells out to
  `omp --mode rpc-ui` and parses `get_login_providers` JSONL live;
  `NewStaticLoginProviders` is the dev fallback when omp is missing.
  SSE events: `ready`, `ui_request`, `auth_complete`, `log`,
  keepalive every 15s.
- **Models catalog (PR #2):** `GET /api/v1/models/catalog` fronts
  `https://models.dev/api.json` with 1h cache, in-flight de-duplication,
  502 fallback. Each entry annotated with `selectable` from
  `get_available_models`, `auth_supported` from `get_login_providers`,
  and caches `cost_input`/`cost_output`/`cache_read`/`cache_write`,
  `max_tokens`, `context_window`, `reasoning`, `thinking_supported`.
- **Project registry (PR #3):** `<share-dir>/projects.json` CRUD,
  idempotent atomic save, `Register()`, `Update()`, `Hide()`,
  `ResolveByCwd`. Concurrent writers serialized via `sync.RWMutex`.
  `FindProjectByCwd` walks cwd, parents, and `.omp/project.json`.
- **Git clone SSE (PR #4):** `POST /api/v1/projects/clone` with SSH URL
  rejection, `folder_name` regex, 10 min timeout, parent writability
  check, live `progress` SSE.
- **CreateProjectDialog (PR #5):** 3-tab dialog (Folder / Clone / Empty);
  `EventSource` SSE progress to `/api/v1/projects/clone/{jobId}/stream`.
  After success, auto-wraps the cloned path as `project_path` for the
  new session.
- **Sidebar rewrite (PR #6):** project-grouped via `useProjects`;
  `rh:active-project-path` + `rh:sidebar-collapsed` persisted in
  localStorage; `ProjectPickerDialog` for new sessions.
- **File access (PR #7):** `GET /api/v1/files` listing, `GET /api/v1/files/content`
  raw with 1 MiB / 10 MiB caps, `GET /api/v1/cwd/browse`,
  `GET /api/v1/git/{repos,status}` with BFS depth ≤4 and skip-list
  `node_modules|target|dist|.venv|__pycache__|.cache|.next`.
- **File explorer (PR #8):** `FileExplorer` 1-level tree, `FileViewer`
  with `react-markdown` + `remark-gfm`, `TabBar` persistent
  per-project, `MarkdownBody` + `GitChangesPanel`.
- **RightSidebar (PR #9):** 3-state width machine (`collapsed` /
  `default` 360 px / `wide` 540 px), 2-rail (Files / Changes),
  persisted in localStorage.
- **i18n baseline (F9):** 137 keys in `en-US.json` + `pt-BR.json`
  mirroring the 08-i18n-baseline spec.

### Fixed

- PR-03/04/07 endpoints now require `AuthMW` (commits `8a09e6d`).
- `LoginProviderInfo.Auth` string → `ApiKeyEnv` + `CredentialProbeEnvs`
  + `SupportsLogin` + `Keyless` capabilities (F2).
- `ModelsDevEntry` gains `max_tokens` + cache cost + `reasoning` +
  `thinking_supported` + `auth_supported` (F5).
- `useFiles` only polls when `state.streaming === true` (F6).
- `POST /sessions` includes `is_orphan` + `project_path` (F7).
- `LoginJob.Auth` removed (F3 dead field).
- Login flow uses `omp --mode rpc-ui` JSONL ack roundtrip (F4).
- Login uses dynamic `NewOMPLoginProviders` (F1, replaces static fallback).

### Changed

- `LoginProviderInfo.Auth` string → 4-orthogonal-capability model (F2).
- `register()` now calls `fileAccess.QuietAllow(path)` for every
  registered project (PR #3 / PR #7 cross-wiring).

### Removed

- `LoginJob.Auth` field (was dead per F3).

## [0.1.0] — 2026-08-16

Phase 0-13 features shipped: bootstrap, omp bridge, RPC stream,
commands, auth store, auth api, front bootstrap + i18n, front chat,
front sidebar, front settings, launcher umbrella, installer + ssh.

[Unreleased]: https://github.com/lucaspdude/rocinante-harness/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/lucaspdude/rocinante-harness/releases/tag/v1.0.0
[0.1.0]: https://github.com/lucaspdude/rocinante-harness/releases/tag/v0.1.0
