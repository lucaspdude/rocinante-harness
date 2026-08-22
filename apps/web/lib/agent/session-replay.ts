// session-replay.ts — fetch the persisted JSONL log for a session
// and convert it into the shape useChatSession's reducer expects.
//
// PR-2 (phase 6) splits this out of useChatSession so the fetch +
// parse logic can be unit-tested without React. The reducer is
// still the single source of truth for messages / frames; this
// helper just translates the api response into the seed payload.

import { tokenProvider } from "../api/client";
import type { ChatMessage, TrajectoryFrame } from "./useChatSession";

export interface ReplayEntry {
  kind: "message" | "frame";
  seq?: number;
  message?: ChatMessage;
  frame?: Record<string, unknown>;
}

export interface ReplayResponse {
  id: string;
  entries: ReplayEntry[];
}

export interface ReplaySeed {
  messages: ChatMessage[];
  frames: TrajectoryFrame[];
}

// Replay fetches the persisted messages for sessionId and folds
// them into the seed shape the reducer takes via the REPLAY action.
// Errors are returned (not thrown) so the hook can decide whether
// to surface them; a missing session is a normal "fresh" state and
// resolves to an empty seed, NOT an error.
export async function fetchReplay(
  sessionId: string,
  opts: { signal?: AbortSignal } = {},
): Promise<{ seed: ReplaySeed; error?: string }> {
  const token = await tokenProvider.getAccess();
  const headers: HeadersInit = token
    ? { Authorization: `Bearer ${token}` }
    : {};
  let response: Response;
  try {
    response = await fetch(`/api/v1/sessions/${sessionId}/messages`, {
      headers,
      signal: opts.signal,
    });
  } catch (err) {
    if ((err as Error).name === "AbortError") {
      return { seed: { messages: [], frames: [] } };
    }
    return { seed: { messages: [], frames: [] }, error: String(err) };
  }
  if (!response.ok) {
    return {
      seed: { messages: [], frames: [] },
      error: `replay HTTP ${response.status}`,
    };
  }
  let body: ReplayResponse;
  try {
    body = (await response.json()) as ReplayResponse;
  } catch (err) {
    return { seed: { messages: [], frames: [] }, error: String(err) };
  }
  return { seed: entriesToSeed(body.entries) };
}

// entriesToSeed maps the api's JSONL entries to the reducer's seed
// shape. Messages go straight into ChatMessage[]; frames get an `at`
// timestamp (the api does not stamp frames, so we use the current
// wall clock — the reducer only reads `frame` from each entry).
export function entriesToSeed(entries: ReplayEntry[]): ReplaySeed {
  const messages: ChatMessage[] = [];
  const frames: TrajectoryFrame[] = [];
  const now = new Date().toISOString();
  for (const entry of entries) {
    if (entry.kind === "message" && entry.message) {
      messages.push(entry.message);
      continue;
    }
    if (entry.kind === "frame" && entry.frame) {
      frames.push({ at: now, frame: entry.frame });
    }
  }
  return { messages, frames };
}
