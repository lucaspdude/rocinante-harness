"use client";

import { useCallback, useEffect, useReducer, useRef } from "react";
import { api } from "../api/client";
import { consumeSse } from "../sse/client";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system" | "tool";
  content: string;
  createdAt: string;
}

export interface ChatState {
  status: "idle" | "streaming" | "awaiting";
  messages: ChatMessage[];
  pendingPrompt: string | null;
  error: string | null;
}

type Action =
  | { type: "RESET"; messages: ChatMessage[] }
  | { type: "USER_MESSAGE"; content: string }
  | { type: "FRAME"; frame: Record<string, unknown> }
  | { type: "AGENT_END" }
  | { type: "ABORT_OK" }
  | { type: "SET_ERROR"; error: string };

function reducer(state: ChatState, action: Action): ChatState {
  switch (action.type) {
    case "RESET":
      return { status: "idle", messages: action.messages, pendingPrompt: null, error: null };
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
      const type = action.frame.type as string | undefined;
      if (type === "delta") {
        const text = (action.frame.text as string) ?? "";
        const last = state.messages[state.messages.length - 1];
        if (last && last.role === "assistant") {
          const updated = [...state.messages];
          updated[updated.length - 1] = { ...last, content: last.content + text };
          return { ...state, messages: updated };
        }
        const fresh: ChatMessage = {
          id: crypto.randomUUID(),
          role: "assistant",
          content: text,
          createdAt: new Date().toISOString(),
        };
        return { ...state, messages: [...state.messages, fresh] };
      }
      if (type === "agent_end") {
        return { ...state, status: "idle" };
      }
      if (type === "error") {
        return { ...state, status: "idle", error: String(action.frame.message ?? "error") };
      }
      return state;
    }
    case "AGENT_END":
      return { ...state, status: "idle" };
    case "ABORT_OK":
      return { ...state, status: "idle" };
    case "SET_ERROR":
      return { ...state, status: "idle", error: action.error };
    default:
      return state;
  }
}

const initialState: ChatState = {
  status: "idle",
  messages: [],
  pendingPrompt: null,
  error: null,
};

export function useChatSession(sessionId: string | null) {
  const [state, dispatch] = useReducer(reducer, initialState);
  const ctrlRef = useRef<AbortController | null>(null);

  const startStream = useCallback(async () => {
    if (!sessionId) return;
    ctrlRef.current = new AbortController();
    const url = `/api/v1/sessions/${sessionId}/events`;
    const headers = new Headers();
    try {
      const res = await fetch(url, {
        method: "GET",
        headers,
        signal: ctrlRef.current.signal,
      });
      if (!res.ok || !res.body) {
        dispatch({ type: "SET_ERROR", error: `stream ${res.status}` });
        return;
      }
      await consumeSse(res, {
        onMessage: (msg) => {
          if (!msg.data) return;
          try {
            const frame = JSON.parse(msg.data) as Record<string, unknown>;
            dispatch({ type: "FRAME", frame });
            if (frame.type === "agent_end") {
              dispatch({ type: "AGENT_END" });
            }
          } catch {
            // ignore non-JSON
          }
        },
      });
    } catch (err) {
      dispatch({ type: "SET_ERROR", error: String(err) });
    }
  }, [sessionId]);

  useEffect(() => {
    startStream();
    return () => ctrlRef.current?.abort();
  }, [startStream]);

  const sendPrompt = useCallback(
    async (text: string) => {
      if (!sessionId) return;
      const idem = crypto.randomUUID();
      dispatch({ type: "USER_MESSAGE", content: text });
      try {
        await api.post(`/api/v1/sessions/${sessionId}/prompt`, {
          json: { text },
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

  return { state, sendPrompt, abort, reset: (msgs: ChatMessage[]) => dispatch({ type: "RESET", messages: msgs }) };
}
