import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { LOCALE_COOKIE, readLocaleFromCookie } from "../lib/locale/cookie";

// Bare-host entrypoint. Sends the user to the locale-aware
// home, honoring the cookie when present and falling back to
// the negotiated browser locale. The destination page
// (app/[locale]/page.tsx) checks the api's onboarding status
// and redirects to /<locale>/onboarding if the install is
// fresh.
export default async function RootPage() {
  const jar = await cookies();
  const cookieHeader = jar.get(LOCALE_COOKIE)?.value;
  const locale = readLocaleFromCookie(cookieHeader);
  redirect(`/${locale}`);
}
