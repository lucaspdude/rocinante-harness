"use client";

import { useEffect, useState } from "react";
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
  const [groups, setGroups] = useState<SessionGroup[]>([]);
  useEffect(() => {
    let cancelled = false;
    api.get<{ groups: SessionGroup[] }>("/api/v1/sessions")
      .then((data) => {
        if (!cancelled) setGroups(data.groups ?? []);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);
  return (
    <nav>
      <ul>
        {groups.map((g) => (
          <li key={g.omp_cwd}>
            <strong>{g.omp_cwd}</strong>
            <ul>
              {g.sessions.map((s) => (
                <li key={s.id} data-active={s.id === activeId}>
                  <a href={`../${s.id}`}>{s.title || s.id}</a>
                  <button
                    type="button"
                    onClick={async () => {
                      await api.delete(`/api/v1/sessions/${s.id}`);
                      setGroups((gs) => gs.filter((group) => !group.sessions.find((x) => x.id === s.id)));
                    }}
                    aria-label="delete"
                  >
                    ×
                  </button>
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    </nav>
  );
}
