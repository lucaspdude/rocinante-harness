"use client";

import { useState } from "react";

interface ComposerProps {
  busy: boolean;
  onSend: (text: string) => void;
  onAbort: () => void;
  placeholder: string;
  sendLabel: string;
  stopLabel: string;
}

export function Composer({
  busy,
  onSend,
  onAbort,
  placeholder,
  sendLabel,
  stopLabel,
}: ComposerProps) {
  const [text, setText] = useState("");
  function submit() {
    if (text.trim() === "") return;
    onSend(text);
    setText("");
  }
  return (
    <div className="flex flex-col gap-2">
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
