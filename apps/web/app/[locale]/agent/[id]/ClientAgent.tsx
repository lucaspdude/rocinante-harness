"use client";

import { useT } from "../../../../lib/i18n";
import { useChatSession } from "../../../../lib/agent/useChatSession";
import { Composer } from "./Composer";
import { MessageList } from "./MessageList";

export function ClientAgent({ sessionId }: { sessionId: string }) {
  const t = useT();
  const { state, sendPrompt, abort } = useChatSession(sessionId);
  return (
    <div className="flex flex-col h-screen">
      <header className="px-4 py-3 border-b border-[var(--color-border)]">
        <h1 className="text-lg font-semibold">{t("agent.title")}</h1>
      </header>
      <div className="flex-1 overflow-y-auto px-4 py-4">
        <MessageList messages={state.messages} />
      </div>
      {state.error && (
        <div className="px-4 py-2">
          <p role="alert" className="rh-error">
            {state.error}
          </p>
        </div>
      )}
      <footer className="border-t border-[var(--color-border)] px-4 py-3">
        <Composer
          busy={state.status === "streaming"}
          onSend={sendPrompt}
          onAbort={abort}
          placeholder={t("agent.placeholder")}
          sendLabel={t("agent.send")}
          stopLabel={t("agent.stop")}
          defaultModelId={state.modelId}
        />
      </footer>
    </div>
  );
}
