"use client";
import { useT } from "../../../../lib/i18n";
import type { ChatMessage } from "../../../../lib/agent/useChatSession";
import { SafeMarkdown } from "../../../../lib/agent/SafeMarkdown";

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  const t = useT();
  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-[var(--color-fg-subtle)]">
        {t("agent.empty")}
      </div>
    );
  }
  return (
    <ul
      role="log"
      aria-live="polite"
      className="flex flex-col gap-3 list-none p-0"
    >
      {messages.map((m) => {
        const isUser = m.role === "user";
        return (
          <li
            key={m.id}
            data-testid="message"
            data-role={m.role}
            className={`flex ${isUser ? "justify-end" : "justify-start"}`}
          >
            <div
              className={`max-w-[80%] rounded-lg px-4 py-2 ${
                isUser
                  ? "bg-[var(--color-primary)] text-[var(--color-primary-fg)]"
                  : "bg-[var(--color-bg-card)] border border-[var(--color-border)]"
              }`}
            >
              <div className="text-xs uppercase tracking-wide opacity-70 mb-1">
                {m.role}
              </div>
              <SafeMarkdown text={m.content} model={m.model} />
            </div>
          </li>
        );
      })}
    </ul>
  );
}
