import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(__dirname, "..");
const en = JSON.parse(readFileSync(resolve(root, "lib/i18n/en-US.json"), "utf8"));
const pt = JSON.parse(readFileSync(resolve(root, "lib/i18n/pt-BR.json"), "utf8"));

describe("i18n parity", () => {
  it("en-US and pt-BR have the same keys", () => {
    const enKeys = Object.keys(en).sort();
    const ptKeys = Object.keys(pt).sort();
    expect(enKeys).toEqual(ptKeys);
  });
  it("no empty values", () => {
    for (const [k, v] of Object.entries(en)) {
      expect(v, `en-US ${k}`).not.toBe("");
    }
    for (const [k, v] of Object.entries(pt)) {
      expect(v, `pt-BR ${k}`).not.toBe("");
    }
  });
});
