import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { LOCALE_COOKIE, readLocaleFromCookie } from "../lib/locale/cookie";

// Bare-host entrypoint. Sends the user to the locale-aware home,
// honoring the cookie when present and falling back to the
// negotiated browser locale.
//
// The destination page renders the same content for signed-in and
// signed-out users (just a different primary CTA), so we don't try
// to branch here based on auth state — tokens live in localStorage
// and aren't visible to the server.
export default async function RootPage() {
  const jar = await cookies();
  const cookieHeader = jar.get(LOCALE_COOKIE)?.value;
  const locale = readLocaleFromCookie(cookieHeader);
  redirect(`/${locale}`);
}
