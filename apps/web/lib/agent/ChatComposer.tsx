"use client";

// Canonical composer UI — Phase 5 PR-04 consolidation.
//
// Single source of truth for the chat composer look-and-feel. The
// 16 px-radius card with auto-resize textarea, access mode pill,
// model picker, and 34 px send circle (PR-05 redesign) lives here
// and is reused by:
//   - apps/web/app/[locale]/agent/page.tsx          (session-less home)
//   - apps/web/app/[locale]/agent/[id]/Composer.tsx (in-session)
//
// Both callers pass `onSend` (a `(text, modelId, accessMode) => void`).
// The session-less wrapper POSTs /api/v1/sessions and then
// router.replace()s to /agent/{id}; the in-session wrapper relays to
// the existing useChatSession.sendPrompt.
//
// Cmd/Ctrl+Enter handling lives in apps/web/lib/keyboard/useShortcuts.ts
// (PR-09) which dispatches a `rh:composer-send` event the parent
// subscribes to.

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
} from "react";
import { ArrowUp, Pencil, Eye, Square } from "lucide-react";
import { useT } from "../i18n";
import { ModelPicker } from "../models/ModelPicker";
import {
  readSelectedModelId,
  writeSelectedModelId,
} from "./selectedModelStorage";

export type AccessMode = "write" | "read";

// Single-line height + vertical padding of the textarea (~22 px line
// height * rows + 0). Computed once at layout so the resize logic does
// not re-measure font metrics on every keystroke.
const LINE_HEIGHT_PX = 22;
// Cap the auto-resize at ~6 lines so a multi-paragraph paste does
// not push the card to the bottom of the viewport.
const MAX_HEIGHT_PX = LINE_HEIGHT_PX * 6 + 16; // 6 lines + padding
const MIN_HEIGHT_PX = LINE_HEIGHT_PX + 16; // 1 line + padding

export interface ChatComposerProps {
  busy: boolean;
  onSend: (text: string, modelId: string | undefined, accessMode: AccessMode) => void;
  onAbort?: () => void;
  placeholder?: string;
  sendLabel?: string;
  stopLabel?: string;
  defaultModelId?: string;
  defaultAccessMode?: AccessMode;
  disabled?: boolean;
  disabledHint?: string;
}

export function ChatComposer({
  busy,
  onSend,
  onAbort,
  placeholder,
  sendLabel = "Send",
  stopLabel = "Stop",
  defaultModelId = "",
  defaultAccessMode = "write",
  disabled = false,
  disabledHint,
}: ChatComposerProps) {
  const t = useT();
  const [text, setText] = useState("");
  const [modelId, setModelId] = useState(defaultModelId);
  const [accessMode, setAccessMode] = useState<AccessMode>(defaultAccessMode);
  // PR-3: prime the picker label on first paint with the stored model
  // id, so a refresh never flashes the "Pick a model" placeholder
  // before the mount-effect restores state. defaultModelId (session
  // metadata) wins when supplied.
  const [initialStoredModel] = useState(() =>
    defaultModelId ? "" : readSelectedModelId() ?? "",
  );
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  // PR-3: restore the user's last selected model from localStorage on
  // mount. When the session metadata supplies its own defaultModelId
  // (re-opening a session that ran with a specific model) the in-session
  // value wins and the stored value is ignored.
  useEffect(() => {
    if (defaultModelId) return;
    const stored = readSelectedModelId();
    if (stored) setModelId(stored);
  }, []);

  function pickModel(m: string) {
    setModelId(m);
    writeSelectedModelId(m);
  }

  const canSend = !disabled && text.trim().length > 0 && !busy;

  function showNeedToPick() {
    if (disabledHint) t(disabledHint);
  }

  // Auto-resize the textarea between MIN_ROWS and MAX_ROWS based on
  // the rendered scrollHeight. We avoid useLayoutEffect's SSR warning
  // by falling back to useEffect on the server.
  const useIsoLayoutEffect =
    typeof window !== "undefined" ? useLayoutEffect : useEffect;

  useIsoLayoutEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    // Cap the textarea height to MAX_HEIGHT_PX so a long paste does
    // not grow the card to the bottom of the viewport; let the
    // user scroll inside the textarea for longer content.
    el.style.height = "auto";
    const next = Math.min(Math.max(el.scrollHeight, MIN_HEIGHT_PX), MAX_HEIGHT_PX);
    el.style.height = `${next}px`;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  function submit() {
    if (busy) return;
    if (disabled) {
      showNeedToPick();
      return;
    }
    const trimmed = text.trim();
    if (!trimmed) return;
    onSend(trimmed, modelId.trim() || undefined, accessMode);
    setText("");
  }

  function onSendClick() {
    if (!canSend) {
      if (disabled) showNeedToPick();
      return;
    }
    submit();
  }

  function onTextareaChange(e: ChangeEvent<HTMLTextAreaElement>) {
    setText(e.target.value);
  }

  function onTextareaKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key !== "Enter") return;
    if (e.shiftKey) return; // newline
    e.preventDefault();
    if (disabled) {
      showNeedToPick();
      return;
    }
    submit();
  }

  function toggleAccessMode() {
    setAccessMode((m) => (m === "write" ? "read" : "write"));
  }

  const resolvedPlaceholder =
    placeholder ??
    (disabled && disabledHint ? disabledHint : t("composer.placeholder"));

  return (
    <div className="rh-composer-card w-full max-w-3xl mx-auto flex flex-col gap-3">
      <textarea
        ref={textareaRef}
        value={text}
        onChange={onTextareaChange}
        onKeyDown={onTextareaKeyDown}
        disabled={disabled}
        rows={1}
        placeholder={resolvedPlaceholder}
        aria-label={resolvedPlaceholder}
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
          <ModelPicker
            value={modelId}
            defaultValue={initialStoredModel}
            onChange={pickModel}
          />
          {busy && onAbort ? (
            <button
              type="button"
              onClick={onAbort}
              aria-label={stopLabel}
              title={stopLabel}
              className="rh-composer-send"
            >
              <Square size={14} fill="currentColor" aria-hidden />
            </button>
          ) : (
            <button
              type="button"
              onClick={onSendClick}
              disabled={!canSend}
              aria-label={sendLabel}
              title={t("composer.sendHint")}
              className="rh-composer-send"
            >
              <ArrowUp size={16} aria-hidden />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
