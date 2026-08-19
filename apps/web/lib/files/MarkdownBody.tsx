"use client";

// MarkdownBody — drop-in replacement for the previous in-file
// escape-only renderer. Defers all sanitization to SafeMarkdown
// (PR-08). Keeping this thin wrapper preserves the import path
// callers (FileViewer) already use.

import { SafeMarkdown } from "../agent/SafeMarkdown";

export function MarkdownBody({ text }: { text: string }) {
  return (
    <div className="flex-1 overflow-auto p-2">
      <SafeMarkdown text={text} />
    </div>
  );
}
