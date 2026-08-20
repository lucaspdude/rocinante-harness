"use client";

import { useCallback, useEffect, useState } from "react";
import { ModelPicker } from "../../../../lib/models/ModelPicker";
import { RH_COMPOSER_SEND } from "../../../../lib/keyboard/useShortcuts";

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
  const [text, setText] = useState("");
  const [modelId, setModelId] = useState(defaultModelId ?? "");

  const submit = useCallback(() => {
    if (text.trim() === "") return;
    onSend(text, modelId || undefined);
    setText("");
  }, [text, modelId, onSend]);

  // PR-09: Cmd/Ctrl+Enter is handled by the global useShortcuts hook
  // mounted in the root layout, which dispatches `rh:composer-send`.
  // Listening here keeps the local onKeyDown textarea clean of
  // modifier logic and lets the same shortcut work for any future
  // composer mounted anywhere in the app.
  useEffect(() => {
    window.addEventListener(RH_COMPOSER_SEND, submit);
    return () => window.removeEventListener(RH_COMPOSER_SEND, submit);
  }, [submit]);

  return (
    <div className="flex flex-col gap-2">
      <ModelPicker value={modelId} onChange={setModelId} />
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={placeholder}
        rows={3}
        aria-label={placeholder}
        className="rh-input resize-none"
      />
      <div className="flex justify-end">
        {busy ? (
          <button
            type="button"
            onClick={onAbort}
            className="rh-button-danger"
          >
            {stopLabel}
          </button>
        ) : (
          <button
            type="button"
            onClick={submit}
            className="rh-button-primary"
          >
            {sendLabel}
          </button>
        )}
      </div>
    </div>
  );
}
