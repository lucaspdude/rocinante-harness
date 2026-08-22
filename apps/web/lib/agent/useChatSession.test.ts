import { describe, it, expect } from "vitest";
import { chatReducer, MAX_FRAMES, type ChatState, type TrajectoryFrame } from "./useChatSession";

// PR-06: the chat reducer now captures every raw SSE frame into
// state.frames so the Trajectory tab can render the agent's full
// event timeline. The reducer is the public seam for this
// behavior — these tests cover the four observable contracts:
//
//   1. known frame types (delta, agent_end, error) still mutate
//      messages / status, AND frame is appended;
//   2. unknown frame types are still captured (e.g. tool_call,
//      tool_result, status) — they're not silently dropped;
//   3. RESET clears the frames window;
//   4. the rolling window cap prevents unbounded growth.

const initial: ChatState = {
	status: "idle",
	messages: [],
	pendingPrompt: null,
	error: null,
	frames: [],
	hydrated: true,
};

function getSeq(frame: TrajectoryFrame): number {
  const f = frame.frame;
  if (f && typeof f === "object" && "seq" in f && typeof f.seq === "number") {
    return f.seq;
  }
  throw new Error("frame missing numeric seq field");
}

describe("chatReducer — frame capture (PR-06 trajectory)", () => {
  it("captures a delta frame and appends to the existing assistant message", () => {
    const seeded: ChatState = {
      ...initial,
      frames: [{ at: "2026-08-20T00:00:00.000Z", frame: {} }],
    };
    const next = chatReducer(seeded, {
      type: "FRAME",
      frame: { type: "delta", text: "hello" },
    });
    expect(next.frames).toHaveLength(2);
    expect(next.frames[1]?.frame).toEqual({ type: "delta", text: "hello" });
    expect(next.frames[1]?.at).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    // delta also appends to messages
    expect(next.messages).toHaveLength(1);
    expect(next.messages[0]?.content).toBe("hello");
  });

  it("captures an agent_end frame and flips status to idle", () => {
    const seeded: ChatState = { ...initial, status: "streaming" };
    const next = chatReducer(seeded, {
      type: "FRAME",
      frame: { type: "agent_end" },
    });
    expect(next.frames).toHaveLength(1);
    expect(next.frames[0]?.frame).toEqual({ type: "agent_end" });
    expect(next.status).toBe("idle");
  });

  it("captures an error frame and surfaces the message", () => {
    const next = chatReducer(initial, {
      type: "FRAME",
      frame: { type: "error", message: "boom" },
    });
    expect(next.frames).toHaveLength(1);
    expect(next.error).toBe("boom");
    expect(next.status).toBe("idle");
  });

  it("captures unknown frame types (tool_call, tool_result, status) without dropping", () => {
    const r1 = chatReducer(initial, {
      type: "FRAME",
      frame: { type: "tool_call", name: "bash", args: { cmd: "ls" } },
    });
    expect(r1.frames).toHaveLength(1);
    expect(r1.frames[0]?.frame).toEqual({
      type: "tool_call",
      name: "bash",
      args: { cmd: "ls" },
    });
    expect(r1.messages).toHaveLength(0);

    const r2 = chatReducer(r1, {
      type: "FRAME",
      frame: { type: "tool_result", output: "ok" },
    });
    expect(r2.frames).toHaveLength(2);
    expect(r2.messages).toHaveLength(0);

    const r3 = chatReducer(r2, {
      type: "FRAME",
      frame: { type: "status", state: "running" },
    });
    expect(r3.frames).toHaveLength(3);
  });

  it("RESET clears the frames window", () => {
    const seeded: ChatState = {
      ...initial,
      frames: [
        { at: "2026-08-20T00:00:00.000Z", frame: { type: "delta", text: "x" } },
        { at: "2026-08-20T00:00:01.000Z", frame: { type: "agent_end" } },
      ],
    };
    const next = chatReducer(seeded, {
      type: "RESET",
      messages: [],
    });
    expect(next.frames).toHaveLength(0);
  });

  it("caps the trajectory window at MAX_FRAMES (drops oldest head)", () => {
    let state: ChatState = initial;
    // Push MAX_FRAMES + 50 frames; the final array should be
    // exactly MAX_FRAMES long, with the oldest 50 dropped.
    for (let i = 0; i < MAX_FRAMES + 50; i++) {
      state = chatReducer(state, {
        type: "FRAME",
        frame: { type: "status", seq: i },
      });
    }
    expect(state.frames).toHaveLength(MAX_FRAMES);
    // The first frame is the 51st appended (idx 50); the last is
    // the (MAX_FRAMES + 50)th appended.
    const first = state.frames[0];
    const last = state.frames[state.frames.length - 1];
    expect(first).toBeDefined();
    expect(last).toBeDefined();
    if (!first || !last) throw new Error("expected frames to be populated");
    expect(getSeq(first)).toBe(50);
    expect(getSeq(last)).toBe(MAX_FRAMES + 49);
  });

  it("preserves model when RESET clears frames", () => {
    const seeded: ChatState = {
      ...initial,
      model: "claude-sonnet-4",
      frames: [
        { at: "2026-08-20T00:00:00.000Z", frame: { type: "delta", text: "x" } },
      ],
    };
    const next = chatReducer(seeded, { type: "RESET", messages: [] });
    expect(next.model).toBe("claude-sonnet-4");
    expect(next.frames).toHaveLength(0);
  });
});

// PR-2 (phase 6) dedup tests: when the api replays a frame's seq
// that arrives again over the live SSE stream, the reducer must
// drop the duplicate so the chat doesn't show the same text twice.
describe("chatReducer — PR-2 replay dedup", () => {
  it("skips a FRAME whose seq already exists in state.frames", () => {
    const seeded: ChatState = {
      ...initial,
      frames: [
        { at: "2026-08-20T00:00:00.000Z", frame: { type: "delta", seq: 7, text: "x" } },
      ],
    };
    const next = chatReducer(seeded, {
      type: "FRAME",
      frame: { type: "delta", seq: 7, text: "x" },
    });
    expect(next).toBe(seeded); // reducer returned same reference → dropped
    expect(next.frames).toHaveLength(1);
    expect(next.messages).toHaveLength(0);
  });

  it("keeps a FRAME with a new seq even if older seqs are present", () => {
    const seeded: ChatState = {
      ...initial,
      frames: [
        { at: "2026-08-20T00:00:00.000Z", frame: { type: "delta", seq: 7, text: "x" } },
      ],
    };
    const next = chatReducer(seeded, {
      type: "FRAME",
      frame: { type: "delta", seq: 8, text: "y" },
    });
    expect(next.frames).toHaveLength(2);
    expect(next.messages).toHaveLength(1);
    expect(next.messages[0]?.content).toBe("y");
  });

  it("passes through a FRAME without a seq (no dedup key)", () => {
    const seeded: ChatState = {
      ...initial,
      frames: [
        { at: "2026-08-20T00:00:00.000Z", frame: { type: "tool_call", name: "bash" } },
      ],
    };
    const next = chatReducer(seeded, {
      type: "FRAME",
      frame: { type: "tool_call", name: "bash" },
    });
    // No seq → no dedup → frame appended twice.
    expect(next.frames).toHaveLength(2);
  });

  it("REPLAY seeds messages and frames atomically and flips hydrated", () => {
    const next = chatReducer(initial, {
      type: "REPLAY",
      seed: {
        messages: [
          { id: "m1", role: "user", content: "hi", createdAt: "2026-08-22T10:00:00Z" },
        ],
        frames: [
          { at: "2026-08-20T00:00:00.000Z", frame: { type: "delta", seq: 1, text: "hello" } },
        ],
      },
    });
    expect(next.messages).toHaveLength(1);
    expect(next.messages[0]?.content).toBe("hi");
    expect(next.frames).toHaveLength(1);
    expect(next.hydrated).toBe(true);
  });
});
