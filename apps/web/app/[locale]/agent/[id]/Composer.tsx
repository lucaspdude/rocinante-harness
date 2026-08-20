"use client";

import { useState } from "react";
import { ModelPicker } from "../../../../lib/models/ModelPicker";
import { useSelectedModel } from "../../../../lib/models/useSelectedModel";

interface ComposerProps {
  busy: boolean;
  // PR-02: renamed 2nd param from `modelId` to `model` for consistency
  // with /api/v1/sessions/{id}/prompt and the rest of the picker
  // pipeline. The body field is `model` (api accepts both `model`
  // and `modelId`); useChatSession already dispatches the same
  // value into state.model so the composer can re-seed after
  // reloads via useSelectedModel below.
  onSend: (text: string, model?: string) => void;
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
  // Seed from localStorage first; fall back to the defaultModelId
  // prop (state.model from useChatSession) and finally to "".
  // useSelectedModel picks the right value on mount without
  // clobbering the user's stored pick when the prop changes.
  const { selectedModel, selectModel } = useSelectedModel(defaultModelId ?? "");
  function submit() {
    if (text.trim() === "") return;
    onSend(text, selectedModel || undefined);
    setText("");
  }
  return (
    <div className="flex flex-col gap-2">
      <ModelPicker value={selectedModel} onChange={selectModel} />
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={placeholder}
        onKeyDown={(e) => {
          if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
            e.preventDefault();
            submit();
          }
        }}
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
