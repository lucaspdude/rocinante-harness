"use client";

// PR-06: client-side chat page.
//
// The header was a single <h1> with the page title; per the
// reference (docs/ui-ux-references/desktop.md §3) the session
// header is exactly 2 rows:
//
//   1. breadcrumb (workspace › session name) + agent pill
//   2. tab strip (Chat / Trajectory)
//
// The tab strip swaps the body between MessageList and
// TrajectoryView. The breadcrumb falls back to the session id
// when the api has no title cache yet so the header is always
// usable. The workspace segment links back to /agent (the
// session-less home with the ChatComposer).
//
// The chat session is consumed via useChatSession; the session
// info (project name + session title) is fetched once on mount
// from /api/v1/sessions and best-effort cached — if the request
// fails we still render the header with the id fallback.

import { useEffect, useRef, useState } from "react";
import { useT } from "../../../../lib/i18n";
import { useToast } from "../../../../lib/toast";
import { useChatSession } from "../../../../lib/agent/useChatSession";
import { SessionHeader, type SessionTab } from "../../../../lib/agent/SessionHeader";
import { TrajectoryView } from "../../../../lib/agent/TrajectoryView";
import { Composer } from "./Composer";
import { MessageList } from "./MessageList";
import { api } from "../../../../lib/api/client";

interface SessionSummary {
  id: string;
  title: string;
  omp_cwd: string;
}

interface SessionGroup {
  omp_cwd: string;
  sessions: SessionSummary[];
}

interface SessionsListResponse {
  groups: SessionGroup[];
}

/** Truncate a session id for the header fallback label. */
function shortSessionId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…${id.slice(-2)}`;
}

/** Last path segment of an absolute filesystem path. */
function basename(p: string): string {
  if (!p) return "";
  const cleaned = p.replace(/\/+$/, "");
  const parts = cleaned.split("/");
  return parts[parts.length - 1] || cleaned;
}

export function ClientAgent({ sessionId }: { sessionId: string }) {
  const t = useT();
  const { state, sendPrompt, abort } = useChatSession(sessionId);
  const toast = useToast();
  const lastErrorRef = useRef<string | null>(null);
  const [tab, setTab] = useState<SessionTab>("chat");
  const [workspaceName, setWorkspaceName] = useState<string | null>(null);
  const [sessionTitle, setSessionTitle] = useState<string>(shortSessionId(sessionId));

  // Surface stream errors as toasts (existing behavior).
  useEffect(() => {
    if (state.error && state.error !== lastErrorRef.current) {
      lastErrorRef.current = state.error;
      toast.error(state.error);
    }
  }, [state.error, toast]);

  // Fetch session info to render the breadcrumb. Best-effort: if
  // the request fails (no token, 404, etc.) we keep the id fallback
  // and let the breadcrumb workspace segment be the fallback label.
  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<SessionsListResponse>("/api/v1/sessions");
        if (cancelled) return;
        for (const g of res.groups ?? []) {
          const match = g.sessions.find((s) => s.id === sessionId);
          if (match) {
            setSessionTitle(match.title || shortSessionId(sessionId));
            setWorkspaceName(basename(match.omp_cwd));
            return;
          }
        }
      } catch {
        // ignore — the breadcrumb falls back to the id label
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  return (
    <div className="flex flex-col h-full min-h-0">
      <SessionHeader
        workspaceName={workspaceName}
        sessionTitle={sessionTitle}
        tab={tab}
        onTabChange={setTab}
      />
      <div
        className="flex-1 overflow-y-auto"
        data-testid="chat-tab-body"
        hidden={tab !== "chat"}
      >
        {tab === "chat" && <MessageList messages={state.messages} />}
      </div>
      <div
        className="flex-1 overflow-y-auto"
        data-testid="trajectory-tab-body"
        hidden={tab !== "trajectory"}
      >
        {tab === "trajectory" && <TrajectoryView frames={state.frames} />}
      </div>
      <footer className="border-t border-[var(--color-border)] px-4 py-3">
        <Composer
          busy={state.status === "streaming"}
          onSend={sendPrompt}
          onAbort={abort}
          placeholder={t("agent.placeholder")}
          sendLabel={t("agent.send")}
          stopLabel={t("agent.stop")}
        defaultModelId={state.model}
        />
      </footer>
    </div>
  );
}
