import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, extname, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "..");
const i18nDir = resolve(repoRoot, "lib/i18n");

const en = JSON.parse(readFileSync(resolve(i18nDir, "en-US.json"), "utf8"));
const pt = JSON.parse(readFileSync(resolve(i18nDir, "pt-BR.json"), "utf8"));

const enKeys = new Set(Object.keys(en));
const ptKeys = new Set(Object.keys(pt));

// Walk apps/web/app/ and apps/web/lib/ for t("...") / t('...') references.
// Skip this test file itself. Accepts single or double quotes; template
// literals are NOT covered (the codebase uses double quotes consistently).
const SEARCH_DIRS = [resolve(repoRoot, "app"), resolve(repoRoot, "lib")];
const T_LITERAL = /\bt\(\s*(["'])([^"']+)\1/g;

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    let s;
    try {
      s = statSync(p);
    } catch {
      continue;
    }
    if (s.isDirectory()) {
      yield* walk(p);
    } else {
      yield p;
    }
  }
}

function* scanSourceFiles() {
  for (const root of SEARCH_DIRS) {
    try {
      for (const f of walk(root)) {
        const ext = extname(f);
        if (ext === ".ts" || ext === ".tsx" || ext === ".js" || ext === ".jsx") {
          if (f === __filename) continue;
          yield f;
        }
      }
    } catch {
      // SEARCH_DIRS may be missing; ignore.
    }
  }
}

describe("i18n parity", () => {
  it("en-US and pt-BR have the same keys", () => {
    const missingInPt = [...enKeys].filter((k) => !ptKeys.has(k));
    const missingInEn = [...ptKeys].filter((k) => !enKeys.has(k));
    expect(missingInPt, `keys missing in pt-BR`).toEqual([]);
    expect(missingInEn, `keys missing in en-US`).toEqual([]);
  });

  it("no empty values", () => {
    for (const [k, v] of Object.entries(en)) {
      expect(v, `en-US ${k}`).not.toBe("");
    }
    for (const [k, v] of Object.entries(pt)) {
      expect(v, `pt-BR ${k}`).not.toBe("");
    }
  });

  it("every t(...) call references a key that exists in BOTH locales", () => {
    const missing = new Map(); // key -> [files]
    for (const file of scanSourceFiles()) {
      const text = readFileSync(file, "utf8");
      for (const m of text.matchAll(T_LITERAL)) {
        const key = m[2];
        if (!enKeys.has(key)) {
          if (!missing.has(key)) missing.set(key, []);
          missing.get(key).push(file);
        }
      }
    }
    const report = [];
    for (const [key, files] of missing) {
      const rel = files.map((f) => f.replace(repoRoot + "/", ""));
      report.push(`en missing: ${key} in ${rel.join(", ")}`);
    }
    expect(report, "i18n key references without en-US entries").toEqual([]);
  });
});
