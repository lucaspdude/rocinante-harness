import { describe, it, expect } from "vitest";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { SafeMarkdown, safeUrlTransform } from "./SafeMarkdown";

// The PR-08 spec (reviewer checklist) calls out six attack
// vectors. The pure-function tests over safeUrlTransform cover
// the URL allowlist; the renderer tests below exercise the
// actual anchor + image behavior end-to-end via
// renderToStaticMarkup (no extra deps). We use createElement
// (not JSX) so the test file does not require vitest's esbuild
// JSX configuration.

describe("safeUrlTransform — 6 attack vectors", () => {
  it("1. rejects javascript: scheme", () => {
    expect(safeUrlTransform("javascript:alert(1)")).toBeNull();
  });

  it("2. rejects raw <script> URL (treated as no-protocol path; falls through)", () => {
    // <script>alert(1)</script> in markdown is escaped to literal
    // text by react-markdown@9 (no rehype-raw). If somehow the URL
    // `script` reaches the transform, it is rejected because it
    // matches none of the allowed patterns.
    expect(safeUrlTransform("script:alert(1)")).toBeNull();
    // And a bare word like `script` (no scheme) is also rejected —
    // it isn't a #fragment, doesn't start with / or ./, etc.
    expect(safeUrlTransform("script")).toBeNull();
  });

  it("3. accepts https://example.com", () => {
    expect(safeUrlTransform("https://example.com")).toBe(
      "https://example.com",
    );
  });

  it("4. accepts /local/path", () => {
    expect(safeUrlTransform("/local/path")).toBe("/local/path");
  });

  it("5. rejects data:image/svg+xml", () => {
    // The data:image/svg+xml scheme is intentionally rejected
    // because SVG can carry inline scripts. Even when wrapped as
    // an href, it must be blocked.
    expect(
      safeUrlTransform("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4="),
    ).toBeNull();
    // And other data: forms are rejected too (only the inline
    // image allowlist covers data:image/(png|jpeg|gif);base64,
    // and only for <img>, not for href).
    expect(safeUrlTransform("data:text/html,<script>alert(1)</script>")).toBeNull();
  });

  it("6. accepts mailto: links", () => {
    expect(safeUrlTransform("mailto:hello")).toBe("mailto:hello");
  });

  it("rejects other dangerous schemes", () => {
    expect(safeUrlTransform("vbscript:msgbox(1)")).toBeNull();
    expect(safeUrlTransform("file:///etc/passwd")).toBeNull();
    expect(safeUrlTransform("ftp://example.com")).toBeNull();
    // Protocol-relative URL is rejected (would inherit the page's
    // protocol and could pivot to javascript:).
    expect(safeUrlTransform("//evil.example/x")).toBeNull();
  });

  it("accepts ./ and ../ relative paths", () => {
    expect(safeUrlTransform("./relative")).toBe("./relative");
    expect(safeUrlTransform("../up")).toBe("../up");
  });

  it("accepts #fragments", () => {
    expect(safeUrlTransform("#heading-1")).toBe("#heading-1");
  });

  it("accepts data:image/(png|jpeg|gif);base64 inline images", () => {
    expect(safeUrlTransform("data:image/png;base64,iVBORw0KGgo=")).toBe(
      "data:image/png;base64,iVBORw0KGgo=",
    );
    expect(safeUrlTransform("data:image/jpeg;base64,/9j/4AAQ")).toBe(
      "data:image/jpeg;base64,/9j/4AAQ",
    );
    expect(safeUrlTransform("data:image/gif;base64,R0lGODlh")).toBe(
      "data:image/gif;base64,R0lGODlh",
    );
  });

  it("rejects Windows-style paths", () => {
    // Backslash should not be treated as a path separator for our
    // purposes; the only accepted local shapes are `/(...)`,
    // `./`, `../`, `#...`.
    expect(safeUrlTransform("\\evil")).toBeNull();
  });
});

describe("SafeMarkdown renderer — anchors", () => {
  it("external https link gets rel=\"noopener noreferrer\" target=\"_blank\"", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[example](https://example.com)" }),
    );
    expect(html).toContain('href="https://example.com"');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it("external mailto link gets rel=\"noopener noreferrer\" target=\"_blank\"", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[mail](mailto:hello)" }),
    );
    expect(html).toContain('href="mailto:hello"');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it("root-relative link has no target or rel", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[local](/local/path)" }),
    );
    expect(html).toContain('href="/local/path"');
    expect(html).not.toContain('target="_blank"');
    expect(html).not.toContain("noopener");
  });

  it("./ and ../ relative links have no target or rel", () => {
    const dotHtml = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[dot](./relative)" }),
    );
    expect(dotHtml).toContain('href="./relative"');
    expect(dotHtml).not.toContain('target="_blank"');

    const parentHtml = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[parent](../up)" }),
    );
    expect(parentHtml).toContain('href="../up"');
    expect(parentHtml).not.toContain('target="_blank"');
  });

  it("#fragment link has no target or rel", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[heading](#heading-1)" }),
    );
    expect(html).toContain('href="#heading-1"');
    expect(html).not.toContain('target="_blank"');
    expect(html).not.toContain("noopener");
  });

  it("javascript: link renders as plain text (href dropped)", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, { text: "[click](javascript:alert(1))" }),
    );
    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("href=");
    expect(html).toContain("click");
  });

  it("data: image href is rejected (renders as text)", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, {
        text: "[svg](data:image/svg+xml,<svg/>)",
      }),
    );
    expect(html).not.toContain("data:image/svg+xml");
    expect(html).not.toContain("<img");
  });
});

describe("SafeMarkdown renderer — images", () => {
  it("https image renders with src", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, {
        text: "![alt](https://example.com/x.png)",
      }),
    );
    expect(html).toContain("<img");
    expect(html).toContain('src="https://example.com/x.png"');
    expect(html).toContain('alt="alt"');
  });

  it("data:image/png;base64 renders with src", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, {
        text: "![alt](data:image/png;base64,iVBORw0KGgo=)",
      }),
    );
    expect(html).toContain("<img");
    expect(html).toContain("data:image/png;base64");
  });

  it("data:image/svg+xml is blocked; placeholder text is shown", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, {
        text: "![alt](data:image/svg+xml,<svg>alert(1)</svg>)",
      }),
    );
    expect(html).not.toContain("<svg");
    expect(html).not.toContain("data:image/svg+xml");
    expect(html).toContain("image blocked");
    expect(html).toContain("alt");
  });
});

describe("SafeMarkdown renderer — raw HTML passthrough", () => {
  it("<script> in markdown is escaped to text, not executed", () => {
    const html = renderToStaticMarkup(
      createElement(SafeMarkdown, {
        text: "<script>alert(1)</script>",
      }),
    );
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
});
