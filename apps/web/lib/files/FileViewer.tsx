"use client";

// FileViewer — shows raw file contents. Markdown via small render;
// binary markers show "binary, N bytes, open in editor" copy.

import { useEffect } from "react";
import { useT } from "../i18n";
import { useFileContent } from "./useFiles";

interface FileViewerProps {
  root: string;
  path: string;
  onClose: () => void;
}

export function FileViewer({ root, path, onClose }: FileViewerProps) {
  const t = useT();
  const { text, binary, loading, error } = useFileContent(root, path);

  useEffect(() => {
    // Reset the markdown path memo when path changes.
  }, [path]);

  return (
    <div className="flex flex-col gap-2 h-full">
      <header className="flex items-center justify-between">
        <span className="text-xs font-mono truncate" title={path}>
          {path}
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] text-sm"
        >
          ×
        </button>
      </header>
      {loading && !text && !binary ? (
        <p className="text-xs text-[var(--color-fg-muted)]">{t("common.loading")}</p>
      ) : error ? (
        <p role="alert" className="rh-error">
          {error}
        </p>
      ) : binary ? (
        <div className="rh-card">
          <p className="text-sm">{t("files.binary")}</p>
        </div>
      ) : text === null ? (
        <p className="text-xs text-[var(--color-fg-muted)]">{t("files.empty")}</p>
      ) : isMarkdown(path) ? (
        <MarkdownBody text={text} />
      ) : (
        <pre className="flex-1 overflow-auto text-xs font-mono whitespace-pre p-2 bg-[var(--color-bg-card)] rounded">
          {text}
        </pre>
      )}
    </div>
  );
}

// MarkdownBody — minimal renderer using HTML escaping only.
function MarkdownBody({ text }: { text: string }) {
  const parts: { kind: "p" | "code"; body: string; lang: string }[] = [];
  const fence = /```(\w+)?\n([\s\S]*?)```/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = fence.exec(text)) !== null) {
    if (m.index > last) {
      const between = text.slice(last, m.index);
      if (between.trim()) parts.push({ kind: "p", body: between, lang: "" });
    }
    const body = m[2] ?? "";
    const lang = m[1] ?? "";
    parts.push({ kind: "code", body, lang });
    last = m.index + m[0].length;
  }
  if (last < text.length) {
    const tail = text.slice(last);
    if (tail.trim()) parts.push({ kind: "p", body: tail, lang: "" });
  }
  if (parts.length === 0) parts.push({ kind: "p", body: text, lang: "" });
  return (
    <div className="flex-1 overflow-auto p-2 text-sm">
      {parts.map((p, i) =>
        p.kind === "code" ? (
          <pre
            key={i}
            className="bg-[var(--color-bg-card)] rounded p-2 font-mono text-xs whitespace-pre overflow-x-auto"
          >
            {p.body}
          </pre>
        ) : (
          <p
            key={i}
            className="my-2 whitespace-pre-wrap"
            dangerouslySetInnerHTML={{ __html: renderSafeParagraph(p.body) }}
          />
        )
      )}
    </div>
  );
}

function renderSafeParagraph(input: string): string {
  const escaped = input
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  return escaped.replace(/\n/g, "<br/>");
}

function isMarkdown(path: string): boolean {
  return /\.(md|markdown)$/i.test(path);
}
