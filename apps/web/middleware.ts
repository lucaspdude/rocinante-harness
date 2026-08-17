// Locale-prefix middleware.
//
// Without this, paths like /login, /settings, or /agent/new match
// the [locale] catch-all (Next treats the first segment as the
// locale) and render the home page with the wrong "locale"
// prefix baked into every link. That's the bug the user reported:
//
//   /login     -> home with href="/login/login"
//   /settings  -> home with href="/settings/settings"
//   /agent/new -> home with href="/agent/new/agent/new"
//
// This middleware ensures every URL starts with a real locale.
// If the first segment isn't one of SUPPORTED_LOCALES, we 308 to
// /<defaultLocale>/<rest-of-path>. We use 308 (permanent) because
// the bad URL will never become valid — /login is not a real
// path; only /<locale>/login is.
//
// Asset paths (/_next/*) and the api rewrite are skipped: those
// are infrastructure, not user-facing pages.

import { NextResponse, type NextRequest } from "next/server";

const SUPPORTED_LOCALES: Record<string, true> = {
  "en-US": true,
  "pt-BR": true,
};
const DEFAULT_LOCALE = "en-US";

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;

  // Strip leading slash, then take the first segment.
  const first: string = pathname.replace(/^\//, "").split("/", 1)[0] ?? "";

  // Empty path ("/") -> let the root page handler do its cookie
  // negotiation + redirect.
  if (first === "") return NextResponse.next();

  // Already a real locale -> let it through.
  if (SUPPORTED_LOCALES[first]) return NextResponse.next();

  // Anything else is a non-locale path. 308-redirect to the same
  // path under the default locale so the [locale] segment is
  // valid. The destination page renders normally.
  const url = req.nextUrl.clone();
  url.pathname = `/${DEFAULT_LOCALE}${pathname.startsWith("/") ? pathname : "/" + pathname}`;
  return NextResponse.redirect(url, 308);
}

// Run on every page request, but skip Next's own chunks and
// anything that goes through the api rewrite (those are proxied
// in next.config.ts and should never see this middleware).
export const config = {
  matcher: ["/((?!api/v1|_next/static|_next/image|favicon.ico).*)"],
};
