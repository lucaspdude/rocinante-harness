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

export function Composer({ busy, onSend, onAbort, placeholder, sendLabel, stopLabel }: ComposerProps) {
  const [text, setText] = useState("");
  function submit() {
    if (text.trim() === "") return;
    onSend(text);
    setText("");
  }
  return (
    <div>
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
      />
      {busy ? (
        <button type="button" onClick={onAbort}>
          {stopLabel}
        </button>
      ) : (
        <button type="button" onClick={submit}>
          {sendLabel}
        </button>
      )}
    </div>
  );
}
