// editor-lang — pure helpers for the codemirror-based FileEditor.
// The map translates a file extension (lowercase, with dot) into a
// canonical "language id" used by the FileEditor to pick a syntax
// extension. Centralized here so it can be unit-tested without
// pulling in codemirror or React.

export type EditorLangId =
  | "javascript"
  | "typescript"
  | "python"
  | "html"
  | "css"
  | "json"
  | "markdown"
  | "go"
  | "rust"
  | "sql"
  | "yaml"
  | "plaintext";

const EXT_TO_LANG: Record<string, EditorLangId> = {
  ".js": "javascript",
  ".mjs": "javascript",
  ".cjs": "javascript",
  ".jsx": "javascript",
  ".ts": "typescript",
  ".tsx": "typescript",
  ".mts": "typescript",
  ".cts": "typescript",
  ".py": "python",
  ".pyi": "python",
  ".html": "html",
  ".htm": "html",
  ".css": "css",
  ".scss": "css",
  ".json": "json",
  ".jsonc": "json",
  ".md": "markdown",
  ".markdown": "markdown",
  ".go": "go",
  ".rs": "rust",
  ".sql": "sql",
  ".yaml": "yaml",
  ".yml": "yaml",
};

// Extract the last extension (case-insensitive). Returns "" when
// the path has no extension (e.g. "Makefile", "Dockerfile").
export function fileExtension(path: string): string {
  const slash = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  const base = slash >= 0 ? path.slice(slash + 1) : path;
  const dot = base.lastIndexOf(".");
  if (dot <= 0) return ""; // dotfiles ("/.env") don't count
  return base.slice(dot).toLowerCase();
}

// Map a file path to a language id. Falls back to "plaintext" when
// the extension is unknown (codemirror will highlight nothing).
export function detectLang(path: string): EditorLangId {
  const ext = fileExtension(path);
  if (!ext) return "plaintext";
  return EXT_TO_LANG[ext] ?? "plaintext";
}