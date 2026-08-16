# Changelog

All notable changes to `rocinante-harness` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Tailwind v4 styling for login, onboarding, settings, sidebar, chat,
  and home pages (`apps/web/app/globals.css`, all page components
  under `apps/web/app/[locale]/...`).
- Settings tabs: General / Account / Devices
  (`apps/web/app/[locale]/settings/page.tsx`).
- `apps/api/cmd/api/main.go --version` prints Go / OS / arch info.
- `scripts/ci.sh --coverage` flag for coverage reporting.
- `scripts/smoke-e2e.sh` driver for Playwright suites.

### Changed

- `apps/web/app/layout.tsx` imports `globals.css` (was unstyled).
- `apps/web/app/[locale]/login/page.tsx`, `onboarding/page.tsx`,
  `settings/page.tsx`, `agent/[id]/ClientAgent.tsx`,
  `agent/Sidebar.tsx`, `agent/[id]/Composer.tsx`,
  `agent/[id]/MessageList.tsx`, `app/[locale]/page.tsx` all
  migrated to Tailwind classes.

## [0.1.0] — 2026-08-16

### Added

- Phase 0-13 features shipped: bootstrap, omp bridge, RPC stream,
  commands, auth store, auth api, front bootstrap + i18n, front chat,
  front sidebar, front settings, launcher umbrella, installer + ssh.
- Installer via `curl | sh` downloads the api, harness, and web bundle
  from the GitHub release.
- systemd units for the api and web.
- Caddy + Cloudflare Tunnel deploy recipes in `docs/deploy/`.

[Unreleased]: https://github.com/lucaspdude/rocinante-harness/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/lucaspdude/rocinante-harness/releases/tag/v0.1.0
