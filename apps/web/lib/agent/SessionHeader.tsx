"use client";

// PR-06: 2-row session header.
//
// The old header was a single <h1> with the title of the page
// only. The reference (DeepSeek harness — docs/ui-ux-references/
// desktop.md §3) collapses the chat page chrome into exactly two
// rows:
//
//   1. breadcrumb (workspace › session name) + agent-pill
//   2. tab strip (Chat / Trajectory)
//
// We keep the tab strip in the header so the user can swap views
// without scrolling; the body renders the active view (MessageList
// vs TrajectoryView). The header is sticky to the top of the chat
// column (the AgentShell grid already gives us border-b separation
// from the conversation body).
//
// Removed from the header (moved to other surfaces in PR-07/PR-11):
//   - share / export / rename buttons
//   - per-message model picker
//   - character / token counters
//   - "running" spinner

import { Bot, ChevronRight } from "lucide-react";
import { useT, useLocalizedPath } from "../i18n";

export type SessionTab = "chat" | "trajectory";

export interface SessionHeaderProps {
  /** Project (workspace) name. `null` when the session is orphan. */
  workspaceName: string | null;
  /** Session title. Falls back to the truncated id when missing. */
  sessionTitle: string;
  /** Active tab in the body. */
  tab: SessionTab;
  /** Switches the active tab. */
  onTabChange: (next: SessionTab) => void;
}

export function SessionHeader({
  workspaceName,
  sessionTitle,
  tab,
  onTabChange,
}: SessionHeaderProps) {
  const t = useT();
  const lp = useLocalizedPath();
  const workspaceLink = lp("/agent");
  const workspaceLabel = workspaceName ?? t("sessionHeader.breadcrumbWorkspaceFallback");

  return (
    <header
      className="border-b border-[var(--color-border)] bg-[var(--color-bg)]"
      data-testid="session-header"
    >
      {/* Row 1 — breadcrumb + agent pill */}
      <div className="flex items-center justify-between gap-2 px-4 py-2">
        <nav
          aria-label="Breadcrumb"
          className="flex items-center gap-1 min-w-0 text-sm"
        >
          <a
            href={workspaceLink}
            data-testid="session-header-workspace"
            className="flex items-center gap-1 text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] truncate"
          >
            <span className="truncate">
              {t("sessionHeader.breadcrumbWorkspace")}
            </span>
          </a>
          <ChevronRight
            aria-hidden
            className="h-3.5 w-3.5 shrink-0 text-[var(--color-fg-subtle)]"
          />
          <span
            data-testid="session-header-session"
            className="font-medium text-[var(--color-fg)] truncate"
            title={sessionTitle}
          >
            {sessionTitle}
          </span>
        </nav>
        <span
          className="inline-flex items-center gap-1.5 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-card)] px-2.5 py-1 text-xs text-[var(--color-fg-muted)]"
          data-testid="session-header-agent-pill"
          title={t("sessionHeader.agentPillTooltip")}
        >
          <Bot aria-hidden className="h-3.5 w-3.5" />
          <span>{t("sessionHeader.agentPill")}</span>
        </span>
      </div>
      {/* Row 2 — tab strip */}
      <div
        role="tablist"
        aria-label={t("sessionHeader.tabChat")}
        className="flex items-center gap-1 px-4"
      >
        <SessionTabButton
          active={tab === "chat"}
          onClick={() => onTabChange("chat")}
          label={t("sessionHeader.tabChat")}
          testId="session-header-tab-chat"
        />
        <SessionTabButton
          active={tab === "trajectory"}
          onClick={() => onTabChange("trajectory")}
          label={t("sessionHeader.tabTrajectory")}
          testId="session-header-tab-trajectory"
        />
        <span className="ml-2 text-xs text-[var(--color-fg-subtle)]">
          {workspaceLabel}
        </span>
      </div>
    </header>
  );
}

function SessionTabButton({
  active,
  onClick,
  label,
  testId,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  testId: string;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      data-active={active}
      data-testid={testId}
      onClick={onClick}
      className={`relative px-3 py-2 text-sm transition-colors ${
        active
          ? "text-[var(--color-fg)]"
          : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      }`}
    >
      <span>{label}</span>
      <span
        aria-hidden
        className={`pointer-events-none absolute inset-x-2 -bottom-px h-0.5 rounded-full transition-colors ${
          active ? "bg-[var(--color-primary)]" : "bg-transparent"
        }`}
      />
    </button>
  );
}
