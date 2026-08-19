"use client";

// FileViewer — shows raw file contents. Markdown via small render;
// binary markers show "binary, N bytes, open in editor" copy.
// PR-05: in-browser editor toggle via <FileEditor onSave=... />.

import { useEffect, useRef, useState } from "react";
import { useT } from "../i18n";
import { useToast, extractError } from "../toast";
import { useFileContent } from "./useFiles";
import { FileEditor } from "./FileEditor";
import { MarkdownBody } from "./MarkdownBody";

interface FileViewerProps {
  root: string;
  path: string;
  onClose: () => void;
}

export function FileViewer({ root, path, onClose }: FileViewerProps) {
  const t = useT();
  const toast = useToast();
  const { text, binary, loading, error, save } = useFileContent(root, path);
  const lastErrorRef = useRef<string | null>(null);
  const [editing, setEditing] = useState(false);

  useEffect(() => {
    if (error && error !== lastErrorRef.current) {
      lastErrorRef.current = error;
      toast.error(error);
    }
  }, [error, toast]);

  // Drop the edit toggle when the file changes underneath us.
  useEffect(() => {
    setEditing(false);
  }, [root, path]);


  return (
    <div className="flex flex-col gap-2 h-full">
      <header className="flex items-center justify-between">
        <span className="text-xs font-mono truncate" title={path}>
          {path}
        </span>
        <div className="flex items-center gap-2">
          {!editing && !loading && !binary && text !== null && text.length > 0 && (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="rh-button-ghost text-sm"
            >
              {t("files.editor.edit")}
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] text-sm"
          >
            ×
          </button>
        </div>
      </header>
      {editing ? (
        <FileEditor
          root={root}
          path={path}
          onSave={async (content) => {
            try {
              await save(content);
              setEditing(false);
            } catch (e: unknown) {
              // Drop edit mode so the user isn't stuck retrying in a
              // stale buffer; the toast surfaces the api error.
              setEditing(false);
              const { message } = extractError(e);
              toast.error(
                message
                  ? t("files.editor.saveFailed", { message })
                  : t("files.editor.saveFailed", { message: "unknown" })
              );
            }
          }}
        />
      ) : loading && !text && !binary ? (
        <p className="text-xs text-[var(--color-fg-muted)]">{t("common.loading")}</p>
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

function isMarkdown(path: string): boolean {
  return /\.(md|markdown)$/i.test(path);
}
