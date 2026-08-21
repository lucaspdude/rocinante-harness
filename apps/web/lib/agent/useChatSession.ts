"use client";

import { useCallback, useEffect, useReducer, useRef } from "react";
import { api, tokenProvider } from "../api/client";
import { consumeSse } from "../sse/client";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system" | "tool";
  content: string;
  createdAt: string;
  // PR-10: model used to produce this message. Assistant messages
  // carry the model id that was active at the time the SSE
  // stream opened; user / system / tool messages leave it
  // undefined. The pill in MessageList reads this.
  model?: string;
}

// TrajectoryFrame is the raw SSE envelope the api relays. The chat
// reducer still parses the same shapes into ChatMessage, but the
// Trajectory tab (PR-06) renders the original event so the user can
// inspect the agent's tool calls, reasoning, and any other fields
// that may arrive in the future. `at` is the wall-clock time the
// frame arrived client-side; the api does not stamp frames.
export interface TrajectoryFrame {
  at: string;
  frame: Record<string, unknown>;
}

export interface ChatState {
  status: "idle" | "streaming" | "awaiting";
  messages: ChatMessage[];
  pendingPrompt: string | null;
  error: string | null;
  model?: string;
  // frames is the rolling window of raw SSE frames. Capped at
  // MAX_FRAMES so a long run doesn't leak memory; older frames are
  // dropped from the head when the cap is hit.
  frames: TrajectoryFrame[];
}

// Exported for tests; not part of the public API.
export const MAX_FRAMES = 500;

type Action =
  | { type: "RESET"; messages: ChatMessage[] }
  | { type: "USER_MESSAGE"; content: string }
  | { type: "FRAME"; frame: Record<string, unknown> }
  | { type: "AGENT_END" }
  | { type: "ABORT_OK" }
  | { type: "SET_ERROR"; error: string }
  | { type: "SET_MODEL"; model: string };

// Exported for tests; not part of the public API.
export function chatReducer(state: ChatState, action: Action): ChatState {
  switch (action.type) {
    case "RESET":
      return {
        status: "idle",
        messages: action.messages,
        pendingPrompt: null,
        error: null,
        model: state.model,
        frames: [],
      };
    case "USER_MESSAGE": {
      const msg: ChatMessage = {
        id: crypto.randomUUID(),
        role: "user",
        content: action.content,
        createdAt: new Date().toISOString(),
      };
      return { ...state, status: "streaming", messages: [...state.messages, msg] };
    }
    case "FRAME": {
      const entry: TrajectoryFrame = {
        at: new Date().toISOString(),
        frame: action.frame,
      };
      const nextFrames =
        state.frames.length >= MAX_FRAMES
          ? [...state.frames.slice(state.frames.length - MAX_FRAMES + 1), entry]
          : [...state.frames, entry];
      const type = action.frame.type as string | undefined;
      if (type === "delta") {
        const text = (action.frame.text as string) ?? "";
        const last = state.messages[state.messages.length - 1];
        if (last && last.role === "assistant") {
          const updated = [...state.messages];
          updated[updated.length - 1] = { ...last, content: last.content + text };
          return { ...state, messages: updated, frames: nextFrames };
        }
        const fresh: ChatMessage = {
          id: crypto.randomUUID(),
          role: "assistant",
          content: text,
          createdAt: new Date().toISOString(),
          model: state.model,
        };
        return { ...state, messages: [...state.messages, fresh], frames: nextFrames };
      }
      if (type === "agent_end") {
        return { ...state, status: "idle", frames: nextFrames };
      }
      if (type === "error") {
        return {
          ...state,
          status: "idle",
          error: String(action.frame.message ?? "error"),
          frames: nextFrames,
        };
      }
      return { ...state, frames: nextFrames };
    }
    case "AGENT_END":
      return { ...state, status: "idle" };
    case "ABORT_OK":
      return { ...state, status: "idle" };
    case "SET_ERROR":
      return { ...state, status: "idle", error: action.error };
    case "SET_MODEL":
      return { ...state, model: action.model };
    default:
      return state;
  }
}

const initialState: ChatState = {
  status: "idle",
  messages: [],
  pendingPrompt: null,
  error: null,
  frames: [],
};

export function useChatSession(sessionId: string | null) {
  const [state, dispatch] = useReducer(chatReducer, initialState);
  const ctrlRef = useRef<AbortController | null>(null);

  const startStream = useCallback(async () => {
    if (!sessionId) return;
    const ctrl = new AbortController();
    ctrlRef.current?.abort();
    ctrlRef.current = ctrl;
    const token = await tokenProvider.getAccess();
    const headers: HeadersInit = token
      ? { Authorization: `Bearer ${token}` }
      : {};
    let response: Response;
    try {
      response = await fetch(`/api/v1/sessions/${sessionId}/events`, {
        headers,
        signal: ctrl.signal,
      });
    } catch (err) {
      if ((err as Error).name === "AbortError") return;
      dispatch({ type: "SET_ERROR", error: String(err) });
      return;
    }
    if (!response.ok) {
      dispatch({ type: "SET_ERROR", error: `SSE start: ${response.status}` });
      return;
    }
    if (!response.body) {
      dispatch({ type: "SET_ERROR", error: "no body" });
      return;
    }
    await consumeSse(response, {
      onMessage: (msg) => {
        try {
          const frame = JSON.parse(msg.data);
          dispatch({ type: "FRAME", frame });
        } catch {
          // ignore malformed frames
        }
      },
      onClose: () => dispatch({ type: "AGENT_END" }),
      onError: (err) => {
        if ((err as Error).name !== "AbortError") {
          dispatch({ type: "SET_ERROR", error: String(err) });
        }
      },
    });
  }, [sessionId]);

  useEffect(() => {
    startStream();
    return () => ctrlRef.current?.abort();
  }, [startStream]);

  const sendPrompt = useCallback(
    async (text: string, model?: string) => {
      if (!sessionId) return;
      const idem = crypto.randomUUID();
      dispatch({ type: "USER_MESSAGE", content: text });
      if (model) {
        dispatch({ type: "SET_MODEL", model });
      }
      try {
        await api.post(`/api/v1/sessions/${sessionId}/prompt`, {
          json: { text, ...(model ? { model } : {}) },
          headers: { "Idempotency-Key": idem },
        });
      } catch (err) {
        dispatch({ type: "SET_ERROR", error: String(err) });
      }
    },
    [sessionId]
  );

  const abort = useCallback(async () => {
    if (!sessionId) return;
    try {
      await api.post(`/api/v1/sessions/${sessionId}/abort`, {});
      dispatch({ type: "ABORT_OK" });
    } catch (err) {
      dispatch({ type: "SET_ERROR", error: String(err) });
    }
  }, [sessionId]);

  return {
    state,
    sendPrompt,
    abort,
    reset: (msgs: ChatMessage[]) => dispatch({ type: "RESET", messages: msgs }),
  };
}
