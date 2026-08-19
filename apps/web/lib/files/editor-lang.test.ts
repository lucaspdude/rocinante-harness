// Unit tests for editor-lang: pure helpers that map a file path to a
// codemirror language id. Centralized here so the codemirror package
// import stays out of the unit-test boundary.

import { describe, it, expect } from "vitest";
import { detectLang, fileExtension, type EditorLangId } from "./editor-lang";

describe("fileExtension", () => {
  it("returns the lowercased extension with the dot", () => {
    expect(fileExtension("src/main.go")).toBe(".go");
    expect(fileExtension("Foo.TS")).toBe(".ts");
    expect(fileExtension("a/b/c.PY")).toBe(".py");
  });
  it("returns an empty string when the path has no extension", () => {
    expect(fileExtension("Makefile")).toBe("");
    expect(fileExtension("src/Makefile")).toBe("");
    expect(fileExtension("Dockerfile")).toBe("");
  });
  it("ignores trailing dots and uses the last segment", () => {
    expect(fileExtension("foo/bar.js")).toBe(".js");
    expect(fileExtension("a/b/c.json")).toBe(".json");
  });
  it("handles backslashes (windows separators) gracefully", () => {
    expect(fileExtension("a\\b\\c.ts")).toBe(".ts");
  });
});

describe("detectLang", () => {
  const cases: Array<[string, EditorLangId]> = [
    ["src/index.js", "javascript"],
    ["src/index.mjs", "javascript"],
    ["src/index.cjs", "javascript"],
    ["src/App.jsx", "javascript"],
    ["src/index.ts", "typescript"],
    ["src/App.tsx", "typescript"],
    ["src/index.mts", "typescript"],
    ["src/index.cts", "typescript"],
    ["scripts/main.py", "python"],
    ["scripts/main.pyi", "python"],
    ["public/index.html", "html"],
    ["public/index.htm", "html"],
    ["styles/main.css", "css"],
    ["styles/main.scss", "css"],
    ["data/manifest.json", "json"],
    ["data/manifest.jsonc", "json"],
    ["README.md", "markdown"],
    ["README.markdown", "markdown"],
    ["cmd/server/main.go", "go"],
    ["src/lib.rs", "rust"],
    ["db/schema.sql", "sql"],
    [".github/workflows/build.yml", "yaml"],
    [".github/workflows/build.yaml", "yaml"],
  ];
  for (const [path, expected] of cases) {
    it(`maps ${path} to ${expected}`, () => {
      expect(detectLang(path)).toBe(expected);
    });
  }
  it("falls back to plaintext for unknown extensions", () => {
    expect(detectLang("foo.xyz")).toBe("plaintext");
    expect(detectLang("data.bin")).toBe("plaintext");
  });
  it("falls back to plaintext for extensionless files", () => {
    expect(detectLang("Makefile")).toBe("plaintext");
    expect(detectLang("Dockerfile")).toBe("plaintext");
  });
  it("is case-insensitive on the extension", () => {
    expect(detectLang("MAIN.GO")).toBe("go");
    expect(detectLang("FOO.JSON")).toBe("json");
  });
});