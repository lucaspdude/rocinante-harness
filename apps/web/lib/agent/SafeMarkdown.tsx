"use client";

// SafeMarkdown — allowlist markdown renderer that hardens
// react-markdown@9 against URL-based attacks.
//
// PR-08 (phase-3-polishing). Defense layers, in order:
//   1. urlTransform: only https:, http:, mailto:, root-relative
//      `/(?!/)`, parent `../` or `./`, `#fragment`, or
//      data:image/(png|jpeg|gif);base64 survive. Everything else
//      (javascript:, data:text/..., vbscript:, file:, arbitrary
//      schemes) is rejected.
//   2. disallowedElements: a belt-and-suspenders list of tag
//      names that must never render (script, style, iframe,
//      object, embed, form, input).
//   3. NO rehype-raw: raw HTML in markdown is escaped by default
//      in react-markdown@9. We intentionally omit rehype-raw so
//      a malicious agent output or poisoned README.md that
//      includes `<script>...</script>` renders as text.
//   4. <a> override: external http/https/mailto links get
//      rel="noopener noreferrer" target="_blank". Local links
//      (`/`, `./`, `../`, `#`) keep no target/rel.
//   5. <img> override: blocks `data:image/svg+xml` URLs.
//      <svg> can carry inline scripts; rejecting the data: URI
//      sidesteps that whole attack surface.
//   6. Code blocks: react-syntax-highlighter for common
//      languages; plain <pre><code> otherwise. Light/dark
//      theme tracks `document.documentElement.dataset.theme`
//      so the syntax palette matches the rest of the UI.

import { useEffect, useState } from "react";
import ReactMarkdown, { type UrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import oneDark from "react-syntax-highlighter/dist/esm/styles/prism/one-dark";
import oneLight from "react-syntax-highlighter/dist/esm/styles/prism/one-light";

// Common languages the area doc explicitly enumerates (D6).
// Anything outside this set falls back to plain <pre><code>.
const KNOWN_LANGS: Record<string, true> = {
  js: true,
  jsx: true,
  ts: true,
  tsx: true,
  py: true,
  python: true,
  go: true,
  rs: true,
  rust: true,
  json: true,
  yaml: true,
  yml: true,
  md: true,
  markdown: true,
  sh: true,
  bash: true,
  zsh: true,
  shell: true,
  sql: true,
  html: true,
  css: true,
};

// Tags that must never render. The first three are the obvious
// script / network-egress vectors; the rest are belt-and-suspenders.
const DISALLOWED_TAGS = [
  "script",
  "style",
  "iframe",
  "object",
  "embed",
  "form",
  "input",
];

// Test-friendly: exported as a pure function so unit tests can
// validate the 6 attack vectors without spinning up a renderer.
// `null` means "reject this URL"; react-markdown@9 then drops the
// href and the link renders as plain text.
//
// The same transform is applied to <a href> and <img src>, so
// we also allowlist the inline-image mime types (png / jpeg /
// gif; data:image/svg+xml stays rejected — SVG can carry inline
// scripts).
export function safeUrlTransform(url: string): string | null {
  if (!url) return null;
  if (/^https?:\/\//i.test(url)) return url;
  if (/^mailto:/i.test(url)) return url;
  // Root-relative path: must not start with `//` (protocol-relative
  // attack) or `/\\` (Windows UNC). Anything else starts with a
  // single slash and then a non-slash character.
  if (/^\/[^\/\\]/.test(url)) return url;
  // Parent or self relative.
  if (/^\.\.?\//.test(url)) return url;
  // Hash fragment.
  if (/^#/.test(url)) return url;
  // Inline image allowlist (img src only in practice; data:image
  // href is harmless — it just renders a no-op link).
  if (/^data:image\/(png|jpeg|gif);base64,/i.test(url)) return url;
  return null;
}

function useResolvedTheme(): "dark" | "light" {
  const [theme, setTheme] = useState<"dark" | "light">("light");
  useEffect(() => {
    if (typeof document === "undefined") return;
    const read = () =>
      document.documentElement.dataset.theme === "dark" ? "dark" : "light";
    setTheme(read());
    const obs = new MutationObserver(() => setTheme(read()));
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => obs.disconnect();
  }, []);
  return theme;
}

interface SafeMarkdownProps {
  text: string;
  className?: string;
  // PR-10: model id used to produce this assistant message.
  // When provided, renders a small unobtrusive pill at the
  // top-right of the markdown body so users can tell which model
  // generated each response (especially when switching models
  // mid-session). User/system/tool messages leave this undefined
  // and render no pill.
  model?: string;
}

export function SafeMarkdown({ text, className, model }: SafeMarkdownProps) {
  const theme = useResolvedTheme();
  const codeTheme = theme === "dark" ? oneDark : oneLight;

  return (
    <div className={`relative ${className ?? "markdown-body text-sm"}`}>
      {model ? (
        <span
          data-testid="model-pill"
          className="absolute top-1 right-1 text-[10px] leading-none px-1.5 py-0.5 rounded bg-[var(--color-bg-card)] text-[var(--color-fg-subtle)] border border-[var(--color-border)]"
          title={model}
        >
          {model}
        </span>
      ) : null}
      <ReactMarkdown
        urlTransform={safeUrlTransform as UrlTransform}
        remarkPlugins={[remarkGfm]}

        rehypePlugins={[]}
        disallowedElements={DISALLOWED_TAGS}
        unwrapDisallowed={false}
        components={{
          a({ href, children, ...rest }) {
            // If safeUrlTransform rejected the href, react-markdown@9
            // strips the `href` attribute before reaching this
            // component. We branch on the post-transform value.
            const external =
              typeof href === "string" &&
              (/^https?:\/\//i.test(href) || /^mailto:/i.test(href));
            if (external) {
              return (
                <a
                  href={href}
                  target="_blank"
                  rel="noopener noreferrer"
                  {...rest}
                >
                  {children}
                </a>
              );
            }
            // Local link — no target, no rel. Defensive copy in case
            // some downstream caller appends rel="..." by accident.
            const { target: _t, rel: _r, ...safe } = rest;
            return (
              <a href={href} {...safe}>
                {children}
              </a>
            );
          },
          img({ src, alt, ...rest }) {
            // urlTransform already enforces the image allowlist
            // (https: or data:image/(png|jpeg|gif);base64,). If a
            // value slipped through, render alt as plain text and
            // never emit a broken <img src="...">.
            const raw = typeof src === "string" ? src : "";
            const ok =
              /^https?:\/\//i.test(raw) ||
              /^data:image\/(png|jpeg|gif);base64,/i.test(raw);
            if (!ok) {
              return (
                <span className="text-[var(--color-fg-muted)] italic">
                  [image blocked: {alt ?? ""}]
                </span>
              );
            }
            return <img src={raw} alt={alt ?? ""} {...rest} />;
          },
          code({ className: cls, children, ...rest }) {
            // Inline code (no className containing "language-*")
            // renders as the default <code>. Block code reaches us
            // with `className="language-xxx"`.
            const match = /language-(\w+)/.exec(cls ?? "");
            const lang = match?.[1];
            const isBlock =
              match !== null ||
              (typeof children === "string" && children.includes("\n"));
            if (!isBlock || !lang || !KNOWN_LANGS[lang]) {
              return (
                <code className={cls} {...rest}>
                  {children}
                </code>
              );
            }
            const value = String(children ?? "").replace(/\n$/, "");
            return (
              <SyntaxHighlighter
                language={lang}
                style={codeTheme}
                customStyle={{
                  margin: "0.5rem 0",
                  borderRadius: "0.375rem",
                  fontSize: "0.8125rem",
                  padding: "0.75rem",
                }}
              >
                {value}
              </SyntaxHighlighter>
            );
          },
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
}
