"use client";

// PR-06: Trajectory tab body.
//
// The Trajectory view is the raw event stream the api relays from
// the agent. The Chat reducer still parses the same frames into
// ChatMessage, but the user wants to see what the agent is doing
// under the hood (tool calls, status events, reasoning, etc.) so
// they can debug. Each frame is rendered as a small card with the
// frame type, wall-clock timestamp, and a compact JSON dump of the
// remaining fields. The view is intentionally lightweight — the
// reference shows a similar timeline; the full Gantt ribbon is
// deferred to a later PR per the phase-4 plan (§"Out of Phase 4").
//
// The view scrolls vertically; the message list and the trajectory
// view share the same scroll container so the page chrome doesn't
// jump when the user switches tabs.

import { useT } from "../i18n";
import type { TrajectoryFrame } from "./useChatSession";

export interface TrajectoryViewProps {
  frames: TrajectoryFrame[];
}

export function TrajectoryView({ frames }: TrajectoryViewProps) {
  const t = useT();
  if (frames.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center px-4 py-12 text-sm text-[var(--color-fg-subtle)]">
        {t("sessionHeader.tabTrajectoryEmpty")}
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-1 overflow-y-auto px-4 py-3">
      <div className="mb-2 text-xs text-[var(--color-fg-subtle)]">
        {t("sessionHeader.trajectoryFrameAtCount", { count: frames.length })}
      </div>
      <ol className="flex flex-col gap-2 list-none p-0">
        {frames.map((entry, idx) => (
          <FrameRow key={`${entry.at}-${idx}`} entry={entry} />
        ))}
      </ol>
    </div>
  );
}

function FrameRow({ entry }: { entry: TrajectoryFrame }) {
  const t = useT();
  const type = typeof entry.frame.type === "string" ? entry.frame.type : "frame";
  const time = formatTime(entry.at);
  return (
    <li
      data-testid="trajectory-frame"
      data-frame-type={type}
      className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2"
    >
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="font-mono uppercase tracking-wide text-[var(--color-fg-muted)]">
          {type}
        </span>
        <span className="text-[var(--color-fg-subtle)]">
          {t("sessionHeader.trajectoryFrameAt", { at: time })}
        </span>
      </div>
      <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap break-words text-xs text-[var(--color-fg)]">
        {JSON.stringify(entry.frame, null, 2)}
      </pre>
    </li>
  );
}

function formatTime(iso: string): string {
  // ISO 8601 → HH:MM:SS.mmm so the timeline reads naturally. The
  // browser's Date parsing is forgiving — falls back to the input
  // if the timestamp is malformed (e.g. during SSR).
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toISOString().slice(11, 23);
}
