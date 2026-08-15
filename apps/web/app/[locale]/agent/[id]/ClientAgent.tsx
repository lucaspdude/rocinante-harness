"use client";

import { useT } from "../../../../lib/i18n";
import { useChatSession } from "../../../../lib/agent/useChatSession";
import { Composer } from "./Composer";
import { MessageList } from "./MessageList";

export function ClientAgent({ sessionId }: { sessionId: string }) {
  const t = useT();
  const { state, sendPrompt, abort } = useChatSession(sessionId);
  return (
    <main>
      <h1>{t("agent.title")}</h1>
      <MessageList messages={state.messages} />
      {state.error && <p role="alert">{state.error}</p>}
      <Composer
        busy={state.status === "streaming"}
        onSend={sendPrompt}
        onAbort={abort}
        placeholder={t("agent.placeholder")}
        sendLabel={t("agent.send")}
        stopLabel={t("agent.stop")}
      />
    </main>
  );
}
