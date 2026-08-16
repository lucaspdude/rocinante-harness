"use client";

import { useEffect, useState } from "react";
import { useT } from "../../../lib/i18n";
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
  const [groups, setGroups] = useState<SessionGroup[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    api
      .get<{ groups: SessionGroup[] }>("/api/v1/sessions")
      .then((data) => {
        if (!cancelled) setGroups(data.groups ?? []);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function newSession() {
    const res = await api.post<{ id: string }>("/api/v1/sessions", {});
    if (res?.id) window.location.href = `../${res.id}`;
  }

  return (
    <aside className="w-60 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col h-full">
      <div className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between">
        <h2 className="text-sm font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">
          {t("sidebar.title")}
        </h2>
        <button
          type="button"
          onClick={newSession}
          className="text-xs rh-button-ghost px-2 py-1"
          title={t("sidebar.newSession")}
        >
          +
        </button>
      </div>
      <div className="flex-1 overflow-y-auto px-2 py-2">
        {loading ? (
          <p className="text-xs text-[var(--color-fg-subtle)] px-2">
            {t("sidebar.loading")}
          </p>
        ) : groups.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-subtle)] px-2">
            {t("sidebar.empty")}
          </p>
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
                          href={`../${s.id}`}
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
                            await api.delete(`/api/v1/sessions/${s.id}`);
                            setGroups((gs) =>
                              gs
                                .map((group) => ({
                                  ...group,
                                  sessions: group.sessions.filter(
                                    (x) => x.id !== s.id
                                  ),
                                }))
                                .filter((group) => group.sessions.length > 0)
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
