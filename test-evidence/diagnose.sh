#!/usr/bin/env bash
# Diagnose phase-5 followup issues.
# Captures browser evidence for each reported problem.

set -euo pipefail

OUT_DIR=/Users/lucas/projects/vharnes/docs/mvp/phase-5-ui-ux/test-evidences
mkdir -p "$OUT_DIR"

cd /Users/lucas/projects/vharnes/rocinante-harness

echo "1. Modal height — SettingsModal"
# Open the browser, login, navigate to /settings
... (handled by the agent via xd://browser tool)

echo "2. Picker — 'No readable subdirectories' issue"
# Navigate to /agent, click + New project, click Pick a folder

echo "3. Session message persist + model persist"
# Send a message; check that the message shows in the session and the model is preserved

echo "4. Textarea auto-resize unbounded"
# Type a long message; check that the textarea doesn't grow past the card

echo "Done."
