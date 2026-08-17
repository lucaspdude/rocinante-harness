"use client";

// Landing page for "New chat". POSTs to /api/v1/sessions, then
// navigates to /agent/<id>. If the create call fails, drops the
// user back to the home page (the api's failure modes are
// surfaced there as a toast in a later iteration).

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../../../../lib/i18n";
import { api } from "../../../../lib/api/client";

interface SessionRecord {
  id: string;
}

export default function NewSessionPage() {
  const t = useT();
  const router = useRouter();
  const lp = useLocalizedPath();

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const session = await api.post<SessionRecord>("/api/v1/sessions", {
          json: { omp_cwd: "/tmp" },
        });
        if (!cancelled && session?.id) {
          router.replace(lp(`/agent/${session.id}`));
        }
      } catch {
        if (!cancelled) router.replace(lp("/"));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router, lp]);

  return (
    <main className="max-w-2xl mx-auto px-4 py-16">
      <div className="rh-card">
        <p className="text-[var(--color-fg-muted)]">{t("agent.connecting")}</p>
      </div>
    </main>
  );
}
