"use client";

// FileEditor — codemirror@6 mount with Edit / Cancel / Save buttons
// and a Ctrl+S keymap. Read-only by default; clicking "Edit" enters
// edit mode; "Cancel" reverts the buffer to the original server state;
// "Save" or Ctrl+S reads the live EditorView doc and calls onSave.
//
// The parent owns the PATCH: onSave(content) is a Promise that throws
// on failure. FileEditor never touches the network itself.
//
// Theme: light/dark is derived from document.documentElement.dataset
// .theme (set by the existing settings page) and observed through a
// MutationObserver so live toggles re-theme the editor without a
// remount.

import { useEffect, useRef, useState } from "react";
import {
  type Extension,
  EditorState,
  Compartment,
  Prec,
} from "@codemirror/state";
import {
  EditorView,
  keymap,
  highlightActiveLine,
  lineNumbers,
  highlightActiveLineGutter,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { defaultHighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { useT } from "../i18n";
import { detectLang, fileExtension } from "./editor-lang";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { go } from "@codemirror/lang-go";
import { rust } from "@codemirror/lang-rust";
import { sql } from "@codemirror/lang-sql";
import { yaml } from "@codemirror/lang-yaml";
import { useFileContent, FILE_EDITOR_MAX_BYTES } from "./useFiles";

interface FileEditorProps {
  root: string;
  path: string;
  // onSave is the parent's save handler. It throws on failure; the
  // FileEditor lets the error bubble up so the parent can toast it
  // (the parent is the only one with the toast context bound to the
  // file lifecycle).
  onSave: (content: string) => Promise<void>;
}

interface SaveRefHandle {
  trigger: () => void;
}

function pickLangExtension(langId: ReturnType<typeof detectLang>): Extension {
  switch (langId) {
    case "javascript":
      return javascript({ jsx: true, typescript: true });
    case "typescript":
      // codemirror's lang-javascript covers TS via the typescript()
      // option; with jsx: true + typescript: true the grammar covers
      // .ts/.tsx/.mts/.cts without a separate package.
      return javascript({ jsx: true, typescript: true });
    case "python":
      return python();
    case "html":
      return html();
    case "css":
      return css();
    case "json":
      return json();
    case "markdown":
      return markdown();
    case "go":
      return go();
    case "rust":
      return rust();
    case "sql":
      return sql();
    case "yaml":
      return yaml();
    default:
      return [];
  }
}

const DARK_THEME = EditorView.theme(
  {
    "&": {
      backgroundColor: "#1c1c26",
      color: "#e4e4e7",
      height: "100%",
      fontSize: "12px",
    },
    ".cm-gutters": {
      backgroundColor: "#14141b",
      color: "#71717a",
      border: "none",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "#2a2a35",
      color: "#e4e4e7",
    },
    ".cm-activeLine": {
      backgroundColor: "#1f1f2c",
    },
    ".cm-cursor": {
      borderLeftColor: "#e4e4e7",
    },
    ".cm-selectionBackground, ::selection": {
      backgroundColor: "#3a3a48",
    },
    ".cm-content": {
      fontFamily: "ui-monospace, SF Mono, Monaco, monospace",
    },
  },
  { dark: true }
);

const LIGHT_THEME = EditorView.theme(
  {
    "&": {
      backgroundColor: "#ffffff",
      color: "#0a0a0f",
      height: "100%",
      fontSize: "12px",
    },
    ".cm-gutters": {
      backgroundColor: "#f5f5f5",
      color: "#71717a",
      border: "none",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "#e4e4e7",
      color: "#0a0a0f",
    },
    ".cm-activeLine": {
      backgroundColor: "#f8f8f8",
    },
    ".cm-cursor": {
      borderLeftColor: "#0a0a0f",
    },
    ".cm-selectionBackground, ::selection": {
      backgroundColor: "#c7d2fe",
    },
    ".cm-content": {
      fontFamily: "ui-monospace, SF Mono, Monaco, monospace",
    },
  },
  { dark: false }
);

export function FileEditor({ root, path, onSave }: FileEditorProps) {
  const t = useT();
  const { text, binary, loading, error } = useFileContent(root, path);

  // 0 = read-only (initial), 1 = editing. The Edit button toggles
  // to editing; Cancel or a successful Save returns to read-only.
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  // Buffer snapshot when entering edit mode. Used by Cancel to
  // re-mount the editor with the original content.
  const [original, setOriginal] = useState<string | null>(null);
  const saveRef = useRef<SaveRefHandle | null>(null);

  const tooLarge = !loading && !error && !binary && text !== null &&
    text.length > FILE_EDITOR_MAX_BYTES;

  function enterEdit() {
    if (text === null) return;
    setOriginal(text);
    setEditing(true);
  }

  function cancel() {
    setEditing(false);
    setOriginal(null);
  }

  if (loading && text === null && !binary) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("common.loading")}
        </p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="rh-error">{error}</p>
      </div>
    );
  }

  if (binary) {
    return (
      <div className="rh-card">
        <p className="text-sm">{t("files.binary")}</p>
      </div>
    );
  }

  if (tooLarge) {
    return (
      <div className="rh-card">
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("files.editor.tooLarge")}
        </p>
      </div>
    );
  }

  if (text === null) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("files.empty")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2 h-full">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          {!editing ? (
            <button
              type="button"
              onClick={enterEdit}
              className="rh-button-ghost text-sm"
            >
              {t("files.editor.edit")}
            </button>
          ) : (
            <>
              <button
                type="button"
                onClick={cancel}
                disabled={saving}
                className="rh-button-ghost text-sm"
              >
                {t("files.editor.cancel")}
              </button>
              <button
                type="button"
                onClick={() => saveRef.current?.trigger()}
                disabled={saving}
                className="rh-button-primary text-sm"
              >
                {saving ? t("common.loading") : t("files.editor.save")}
              </button>
            </>
          )}
        </div>
        {!editing && (
          <span className="text-xs text-[var(--color-fg-muted)]">
            {t("files.editor.readOnly")}
          </span>
        )}
      </div>
      <div className="flex-1 overflow-hidden rounded border border-[var(--color-border)]">
        {editing ? (
          <CodeMirrorEditor
            key={`edit-${root}-${path}-${original?.length ?? 0}`}
            initial={original ?? text}
            path={path}
            readOnly={false}
            onSave={async (content) => {
              setSaving(true);
              try {
                await onSave(content);
                setEditing(false);
                setOriginal(null);
              } catch {
                // Parent toasts the error. Drop edit mode and
                // remount with the original buffer.
                setEditing(false);
                setOriginal(null);
              } finally {
                setSaving(false);
              }
            }}
            saveRef={saveRef}
          />
        ) : (
          <CodeMirrorEditor
            key={`ro-${root}-${path}-${text.length}`}
            initial={text}
            path={path}
            readOnly={true}
            saveRef={saveRef}
          />
        )}
      </div>
    </div>
  );
}

interface CodeMirrorEditorProps {
  initial: string;
  path: string;
  readOnly: boolean;
  onSave?: (content: string) => Promise<void>;
  saveRef: React.MutableRefObject<SaveRefHandle | null>;
}

function CodeMirrorEditor({
  initial,
  path,
  readOnly,
  onSave,
  saveRef,
}: CodeMirrorEditorProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  // Compartments let us swap theme/readonly/lang without rebuilding
  // the entire EditorState (which would discard the buffer).
  const themeCompartment = useRef(new Compartment()).current;
  const readOnlyCompartment = useRef(new Compartment()).current;
  const langCompartment = useRef(new Compartment()).current;
  const [theme, setTheme] = useState<"dark" | "light">("light");

  // Track theme changes via a MutationObserver on data-theme.
  useEffect(() => {
    setTheme(readTheme());
    if (typeof document === "undefined") return;
    const obs = new MutationObserver(() => setTheme(readTheme()));
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => obs.disconnect();
  }, []);

  // Mount the EditorView once. Subsequent prop changes reconfigure
  // via compartments.
  useEffect(() => {
    if (!hostRef.current) return;
    const langId = detectLang(path);
    const langExt = pickLangExtension(langId);

    // The save handler reads the live doc and delegates to the
    // parent's onSave. It claims the key event by returning true so
    // no other handler sees Ctrl+S.
    const saveKeymap = keymap.of([
      {
        key: "Mod-s",
        preventDefault: true,
        run: () => {
          if (onSave) {
            void onSave(viewRef.current?.state.doc.toString() ?? "");
          }
          return true;
        },
      },
    ]);

    const state = EditorState.create({
      doc: initial,
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        history(),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        keymap.of(defaultKeymap),
        keymap.of(historyKeymap),
        Prec.high(saveKeymap),
        langCompartment.of(langExt),
        themeCompartment.of(readTheme() === "dark" ? DARK_THEME : LIGHT_THEME),
        readOnlyCompartment.of(EditorState.readOnly.of(readOnly)),
      ],
    });
    const view = new EditorView({ state, parent: hostRef.current });
    viewRef.current = view;
    saveRef.current = {
      trigger: () => {
        if (onSave) {
          void onSave(view.state.doc.toString());
        }
      },
    };
    return () => {
      view.destroy();
      viewRef.current = null;
      saveRef.current = null;
    };
    // We mount once per (path, initial). Subsequent prop changes use
    // the compartments below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, initial]);

  // Reconfigure theme when the system toggles.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: themeCompartment.reconfigure(
        theme === "dark" ? DARK_THEME : LIGHT_THEME
      ),
    });
  }, [theme, themeCompartment]);

  // Reconfigure readOnly when the prop flips.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: readOnlyCompartment.reconfigure(EditorState.readOnly.of(readOnly)),
    });
  }, [readOnly, readOnlyCompartment]);

  // Reconfigure language if the file extension changes mid-session.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const langId = detectLang(path);
    view.dispatch({
      effects: langCompartment.reconfigure(pickLangExtension(langId)),
    });
  }, [path, langCompartment]);

  return <div ref={hostRef} className="h-full overflow-auto" />;
}

function readTheme(): "dark" | "light" {
  if (typeof document === "undefined") return "light";
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

// Re-export so callers (FileViewer) can show the placeholder without
// re-deriving the size limit.
export { fileExtension };