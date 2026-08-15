"use client";

import { useT } from "../../lib/i18n";

export default function HomePage() {
  const t = useT();
  return (
    <main>
      <h1>{t("app.name")}</h1>
      <p>{t("footer.tagline")}</p>
    </main>
  );
}
