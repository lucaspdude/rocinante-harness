// SSE client that parses W3C-compliant text/event-stream payloads
// from a fetch ReadableStream. Handles chunked delivery (frames
// split across ReadableStream chunks), multibyte UTF-8 that may
// span chunks, and the standard 'data:' / 'id:' / 'event:' fields.

export interface SseMessage {
  id?: string;
  event?: string;
  data: string;
}

export interface SseHandlers {
  onMessage: (msg: SseMessage) => void;
  onError?: (err: unknown) => void;
  onClose?: () => void;
}

export async function consumeSse(response: Response, handlers: SseHandlers): Promise<void> {
  if (!response.ok) {
    handlers.onError?.(new Error(`SSE start: ${response.status}`));
    return;
  }
  if (!response.body) {
    handlers.onError?.(new Error("no response body"));
    return;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";
  let pending: { event?: string; id?: string; data: string[] } | null = null;

  function dispatch() {
    if (!pending) return;
    if (pending.data.length === 0) {
      pending = null;
      return;
    }
    handlers.onMessage({
      id: pending.id,
      event: pending.event,
      data: pending.data.join("\n"),
    });
    pending = null;
  }

  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buffer.indexOf("\n")) !== -1) {
        let line = buffer.substring(0, idx);
        if (line.endsWith("\r")) line = line.substring(0, line.length - 1);
        buffer = buffer.substring(idx + 1);
        if (line === "") {
          dispatch();
          continue;
        }
        if (line.startsWith(":")) continue;
        const colon = line.indexOf(":");
        let field = line;
        let value = "";
        if (colon !== -1) {
          field = line.substring(0, colon);
          value = colon + 1 < line.length ? line.substring(colon + 1) : "";
          if (value.startsWith(" ")) value = value.substring(1);
        }
        if (field === "data") {
          if (!pending) pending = { data: [] };
          pending.data.push(value);
        } else if (field === "id") {
          if (!pending) pending = { data: [] };
          pending.id = value;
        } else if (field === "event") {
          if (!pending) pending = { data: [] };
          pending.event = value;
        }
      }
    }
  } catch (err) {
    handlers.onError?.(err);
  }
  handlers.onClose?.();
}
