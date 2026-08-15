"use client";

import type { ChatMessage } from "../../../../lib/agent/useChatSession";

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  return (
    <ul role="log" aria-live="polite" style={{ listStyle: "none", padding: 0 }}>
      {messages.map((m) => (
        <li key={m.id} data-testid="message" data-role={m.role}>
          <strong>{m.role}</strong>
          <pre style={{ whiteSpace: "pre-wrap" }}>{m.content}</pre>
        </li>
      ))}
    </ul>
  );
}
