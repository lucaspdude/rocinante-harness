# Spike: @jeryfan/finder-ui vs Chonky vs custom picker

**Date:** 2026-08-21
**Author:** phase-5 PR-09 spike
**Outcome:** keep the existing custom Finder-style picker; do not adopt a third-party lib.

## Question

The user's screenshot complained that the picker was not "amigável" and
asked for a real folder browser, similar to the ompweb / abandoned
rocinante fork. Phase-4 PR-03 (commit `c62e2f`) shipped a custom Mac-Finder-style
picker; phase-5 PR-01 (v1.23.1) fixed the i18n namespace bug that made it
render literal keys. The PR-09 spec proposes switching to an external lib
for a richer UX.

## Spike procedure

In a worktree on branch `spike/chonky-vs-alt`:

1. `pnpm --filter web add chonky` — failed. Chonky v3 requires
   `react@^16 || ^17` but the harness runs React 19. Type declarations
   from Chonk's `@material-ui/*` deps collide with `@types/react@19` and
   the web bundle fails `tsc --noEmit`. Build is unblocked only by
   `--legacy-peer-deps`, which masks a real incompatibility.

2. `pnpm --filter web add @jeryfan/finder-ui` — succeeded. The lib
   declares React 18/19 peer deps, the harness builds clean, and the
   published bundle types resolve. However, integrating the full Finder
   component requires a zustand store, CodeMirror (for file preview),
   and marked (for markdown preview). Together those add ~150 KB minified
   and ~50 KB gzipped to the bundle that ships in the picker chunk,
   and the harness doesn't need drag-drop, multi-select, or an inline
   preview pane (the user already has a sidebar FileViewer that handles
   file contents).

3. The `Finder` named export expects an opinionated store layout
   (sidebar tabs, breadcrumbs, file actions, drag-drop) that doesn't map
   to the existing `CreateProjectDialog` + `ProjectSelectorBar` UI
   surfaces. A clean integration would require a parallel implementation
   alongside the existing modal — effectively doubling maintenance
   surface for a marginal UX gain.

## Decision

Keep the existing Finder-style picker (`DirectoryPicker.tsx` after
PR-01). The namespace fix and the i18n parity test (PR-03) give it the
translations it needs; the upgraded visual polish from an external lib
does not justify the bundle-weight and integration cost.

The picker can still be improved in follow-up work (drag-drop, file
preview, multi-select) without depending on a third-party package.

## Action item

Close PR-09 as a no-change spike commit. No code change.
