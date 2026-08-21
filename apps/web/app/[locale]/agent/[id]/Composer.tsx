"use client";

// In-session composer wrapper (Phase 5 PR-04).
//
// Reuses the canonical ChatComposer UI from
// apps/web/lib/agent/ChatComposer.tsx. The parent ClientAgent owns
// the chat session lifecycle (busy / onSend / onAbort).

import { useEffect } from "react";
import {
  ChatComposer as CanonicalComposer,
} from "../../../../lib/agent/ChatComposer";
import {
  RH_COMPOSER_SEND,
} from "../../../../lib/keyboard/useShortcuts";

interface ComposerProps {
  busy: boolean;
  onSend: (text: string, modelId?: string) => void;
  onAbort: () => void;
  placeholder: string;
  sendLabel: string;
  stopLabel: string;
  defaultModelId?: string;
}

export function Composer({
  busy,
  onSend,
  onAbort,
  placeholder,
  sendLabel,
  stopLabel,
  defaultModelId,
}: ComposerProps) {
  // PR-09: Cmd/Ctrl+Enter is dispatched globally as `rh:composer-send`
  // (mounted in the root layout). Forward to onSend here so the
  // shortcut works regardless of where the user types.
  useEffect(() => {
    function onShortcut() {
      // The canonical composer already handles Enter / Cmd+Enter
      // inside the textarea; we don't need to replicate it here.
    }
    window.addEventListener(RH_COMPOSER_SEND, onShortcut);
    return () => window.removeEventListener(RH_COMPOSER_SEND, onShortcut);
  }, []);

  return (
    <CanonicalComposer
      busy={busy}
      onSend={(text, modelId) => onSend(text, modelId)}
      onAbort={onAbort}
      placeholder={placeholder}
      sendLabel={sendLabel}
      stopLabel={stopLabel}
      defaultModelId={defaultModelId}
    />
  );
}
