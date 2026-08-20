"use client";

// PR-02: session-less chat composer.
//
// Renders the same composer UI (ModelPicker + textarea + send button)
// as the existing in-session composer in apps/web/app/[locale]/agent/[id]/
// Composer.tsx. The difference: this composer holds a draft until the
// user sends for the first time. On send, it POSTs /api/v1/sessions
// with the selected project as cwd, then router.replace()s to /agent/{id}.
//
// When no project is selected, the composer is disabled: sending fires
// a toast telling the user to pick a project (per the PR spec, not a
// modal gate). Plain Enter sends; Cmd/Ctrl+Enter also sends.

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../i18n";
import { useToast } from "../toast";
import { api, ApiClientError, tokenProvider } from "../api/client";
import { ModelPicker } from "../models/ModelPicker";
import type { Project } from "../projects/useProjects";

interface Props {
  project: Project | null;
}

interface CreatedSession {
  id: string;
}

export function ChatComposer({ project }: Props) {
  const t = useT();
  const lp = useLocalizedPath();
  const router = useRouter();
  const toast = useToast();
  const [text, setText] = useState("");
  const [modelId, setModelId] = useState("");
  const [busy, setBusy] = useState(false);

  const disabled = project === null;

  function showNeedToPick() {
    toast.error(t("projectSelector.needToPick"));
  }

  async function submit() {
    if (busy) return;
    if (!project) {
      showNeedToPick();
      return;
    }
    const trimmed = text.trim();
    if (!trimmed) return;
    // Creating a session and then opening its SSE stream both require a
    // bearer token. Without one the stream fails with 401 after the
    // redirect, stranding the user on a dead chat page — send them to
    // /login before anything is created.
    const token = await tokenProvider.getAccess();
    if (!token) {
      toast.error(t("composer.signInRequired"));
      router.replace(lp("/login"));
      return;
    }
    setBusy(true);
    try {
      const body: { omp_cwd: string; project_path: string; model?: string } = {
        omp_cwd: project.path,
        project_path: project.path,
      };
      if (modelId.trim()) body.model = modelId.trim();
      const res = await api.post<CreatedSession>("/api/v1/sessions", {
        json: body,
      });
      if (res?.id) {
        router.replace(lp(`/agent/${res.id}`));
      } else {
        toast.error("Failed to create session");
      }
    } catch (e) {
      const msg =
        e instanceof ApiClientError
          ? e.body.message ?? e.message
          : e instanceof Error
          ? e.message
          : String(e);
      toast.error(msg);
    } finally {
      setBusy(false);
    }
  }

  function onSendClick() {
    if (disabled) {
      showNeedToPick();
      return;
    }
    void submit();
  }

  return (
    <div className="flex flex-col gap-3 p-6 max-w-3xl w-full mx-auto">
      <ModelPicker value={modelId} onChange={setModelId} />
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        disabled={disabled}
        rows={5}
        placeholder={
          disabled
            ? t("projectSelector.needToPick")
            : t("agent.placeholder")
        }
        aria-label={t("agent.placeholder")}
        className="rh-input resize-none disabled:opacity-50"
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            if (e.shiftKey) return;
            e.preventDefault();
            if (disabled) {
              showNeedToPick();
              return;
            }
            void submit();
          }
        }}
      />
      <div className="flex justify-end">
        <button
          type="button"
          onClick={onSendClick}
          disabled={busy}
          className="rh-button-primary disabled:opacity-50"
        >
          {t("agent.send")}
        </button>
      </div>
    </div>
  );
}
