"use client";

// PR-05: redesigned chat composer (DeepSeek reference).
//
// Single textarea + footer row inside a 16 px-radius shadowed card.
// Footer: left = access-mode pill; right = ModelPicker + 34 px send
// circle. Sits inside the chat-first home (apps/web/app/[locale]/agent/
// page.tsx) — when the user has not picked a project, the composer
// still renders but stays disabled and a toast surfaces "pick a project"
// on send (per PR-02 contract, not a modal gate).
//
// On send the composer POSTs /api/v1/sessions with the selected
// project as cwd and the current model id (PR-02 wiring), then
// router.replace()s to /agent/{id}. Plain Enter sends;
// Shift+Enter inserts a newline; Cmd/Ctrl+Enter routing is wired in
// PR-09 (TODO marker below).

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
} from "react";
import { useRouter } from "next/navigation";
import { ArrowUp, Pencil, Eye, Square } from "lucide-react";
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

type AccessMode = "write" | "read";

const MIN_ROWS = 1;
const MAX_ROWS = 6;
// Single-line height + vertical padding of the textarea (~22 px line
// height * rows + 0). Computed once at layout so the resize logic does
// not re-measure font metrics on every keystroke.
const LINE_HEIGHT_PX = 22;

export function ChatComposer({ project }: Props) {
  const t = useT();
  const lp = useLocalizedPath();
  const router = useRouter();
  const toast = useToast();
  const [text, setText] = useState("");
  const [modelId, setModelId] = useState("");
  const [accessMode, setAccessMode] = useState<AccessMode>("write");
  const [busy, setBusy] = useState(false);
  const [rows, setRows] = useState(MIN_ROWS);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  const disabled = project === null;
  const canSend = !disabled && text.trim().length > 0 && !busy;

  function showNeedToPick() {
    toast.error(t("projectSelector.needToPick"));
  }

  // Auto-resize the textarea between MIN_ROWS and MAX_ROWS based on
  // the rendered scrollHeight. We avoid useLayoutEffect's SSR warning
  // by falling back to useEffect on the server.
  const useIsoLayoutEffect =
    typeof window !== "undefined" ? useLayoutEffect : useEffect;

  useIsoLayoutEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    const lineCount = Math.ceil(el.scrollHeight / LINE_HEIGHT_PX);
    const clamped = Math.min(Math.max(lineCount, MIN_ROWS), MAX_ROWS);
    if (clamped !== rows) setRows(clamped);
    // We intentionally only depend on `text`; reading `rows` would
    // re-fire the effect right after every resize and lose the
    // measurement.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

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
        // TODO(PR-09): wire the global Cmd+Enter shortcut so the user
        // can send from anywhere on the page; for now the keydown
        // handler on the textarea below is the only send trigger.
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
    if (!canSend) {
      if (disabled) showNeedToPick();
      return;
    }
    void submit();
  }

  function onTextareaChange(e: ChangeEvent<HTMLTextAreaElement>) {
    setText(e.target.value);
  }

  function onTextareaKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key !== "Enter") return;
    if (e.shiftKey) return; // newline
    if (e.metaKey || e.ctrlKey) {
      // Cmd/Ctrl+Enter is the canonical send shortcut (PR-09 will
      // hoist this to a global handler). Behaviour matches Enter on
      // the textarea today so the user can send by either combo.
      e.preventDefault();
      if (disabled) {
        showNeedToPick();
        return;
      }
      void submit();
      return;
    }
    e.preventDefault();
    if (disabled) {
      showNeedToPick();
      return;
    }
    void submit();
  }

  function toggleAccessMode() {
    setAccessMode((m) => (m === "write" ? "read" : "write"));
  }

  return (
    <div className="rh-composer-card w-full max-w-3xl mx-auto flex flex-col gap-3">
      <textarea
        ref={textareaRef}
        value={text}
        onChange={onTextareaChange}
        onKeyDown={onTextareaKeyDown}
        disabled={disabled}
        rows={rows}
        placeholder={
          disabled
            ? t("projectSelector.needToPick")
            : t("composer.placeholder")
        }
        aria-label={t("composer.placeholder")}
        title={t("composer.sendHint")}
        className="rh-composer-textarea disabled:opacity-50"
      />
      <div className="flex flex-wrap items-center justify-between gap-2 mt-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          <button
            type="button"
            onClick={toggleAccessMode}
            disabled={busy}
            aria-pressed={accessMode === "write"}
            className="rh-composer-pill"
            title={
              accessMode === "write"
                ? t("composer.accessMode.write")
                : t("composer.accessMode.read")
            }
          >
            {accessMode === "write" ? (
              <Pencil size={12} aria-hidden />
            ) : (
              <Eye size={12} aria-hidden />
            )}
            <span>
              {t(
                accessMode === "write"
                  ? "composer.accessMode.write"
                  : "composer.accessMode.read",
              )}
            </span>
          </button>
        </div>
        <div className="flex items-center gap-1.5 ml-auto">
          <ModelPicker value={modelId} onChange={setModelId} />
          <button
            type="button"
            onClick={onSendClick}
            disabled={!canSend}
            aria-label={busy ? t("composer.stop") : t("composer.placeholder")}
            title={t("composer.sendHint")}
            className="rh-composer-send"
          >
            {busy ? (
              <Square size={14} fill="currentColor" aria-hidden />
            ) : (
              <ArrowUp size={16} aria-hidden />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
