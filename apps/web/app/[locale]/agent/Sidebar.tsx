"use client";

import { useEffect, useState } from "react";
import { useT, useLocalizedPath } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";

interface SessionGroup {
  omp_cwd: string;
  sessions: Array<{
    id: string;
    omp_cwd: string;
    title: string;
    state: string;
  }>;
}

export function Sidebar({ activeId }: { activeId: string }) {
  const t = useT();
  const lp = useLocalizedPath();
  const [groups, setGroups] = useState<SessionGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  function reload() {
    api
      .get<{ groups: SessionGroup[] }>("/api/v1/sessions")
      .then((data) => setGroups(data.groups ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    reload();
  }, []);

  async function newSession() {
    if (busy) return;
    setBusy(true);
    try {
      const res = await api.post<{ id: string }>("/api/v1/sessions", {
        json: { omp_cwd: "/tmp" },
      });
      if (res?.id) {
        window.location.href = lp(`/agent/${res.id}`);
      }
    } catch {
      // The chat page renders an error banner; the sidebar
      // doesn't need its own copy.
    } finally {
      setBusy(false);
    }
  }

  const totalSessions = groups.reduce(
    (n, g) => n + g.sessions.length,
    0
  );

  return (
    <aside className="w-60 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col h-full">
      <div className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between">
        <h2 className="text-sm font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">
          {t("sidebar.title")}
        </h2>
        <button
          type="button"
          onClick={newSession}
          disabled={busy}
          className="text-xs rh-button-ghost px-2 py-1 disabled:opacity-50"
          title={t("sidebar.newSession")}
          aria-label={t("sidebar.newSession")}
        >
          {busy ? "…" : "+"}
        </button>
      </div>
      <div className="flex-1 overflow-y-auto px-2 py-2">
        {loading ? (
          <p className="text-xs text-[var(--color-fg-subtle)] px-2">
            {t("sidebar.loading")}
          </p>
        ) : totalSessions === 0 ? (
          <div className="flex flex-col items-center gap-2 px-3 py-6 text-center">
            <p className="text-xs text-[var(--color-fg-subtle)]">
              {t("sidebar.empty")}
            </p>
            <button
              type="button"
              onClick={newSession}
              disabled={busy}
              className="text-xs rh-button-primary px-3 py-1 disabled:opacity-50"
            >
              {t("sidebar.newSession")}
            </button>
          </div>
        ) : (
          <ul className="flex flex-col gap-3">
            {groups.map((g) => (
              <li key={g.omp_cwd}>
                <strong className="block text-xs text-[var(--color-fg-subtle)] px-2 mb-1 truncate">
                  {g.omp_cwd}
                </strong>
                <ul className="flex flex-col gap-0.5">
                  {g.sessions.map((s) => {
                    const active = s.id === activeId;
                    return (
                      <li
                        key={s.id}
                        data-active={active}
                        className={`flex items-center justify-between rounded px-2 py-1 ${
                          active
                            ? "bg-[var(--color-bg-card)]"
                            : "hover:bg-[var(--color-bg-card)]"
                        }`}
                      >
                        <a
                          href={lp(`/agent/${s.id}`)}
                          className={`flex-1 truncate text-sm ${
                            active
                              ? "text-[var(--color-fg)]"
                              : "text-[var(--color-fg-muted)]"
                          }`}
                        >
                          {s.title || s.id}
                        </a>
                        <button
                          type="button"
                          onClick={async () => {
                            await api.delete(
                              `/api/v1/sessions/${s.id}`
                            );
                            setGroups((gs) =>
                              gs
                                .map((group) => ({
                                  ...group,
                                  sessions: group.sessions.filter(
                                    (x) => x.id !== s.id
                                  ),
                                }))
                                .filter(
                                  (group) => group.sessions.length > 0
                                )
                            );
                          }}
                          aria-label={t("sidebar.delete")}
                          className="ml-1 text-[var(--color-fg-subtle)] hover:text-[var(--color-danger)]"
                        >
                          ×
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}
