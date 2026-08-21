"use client";

// Session-less composer wrapper (Phase 5 PR-04).
//
// Reuses the canonical ChatComposer UI from
// ./ChatComposer.tsx. On send, POSTs /api/v1/sessions with the selected
// project as cwd, then router.replace()s to /agent/{id}. When no
// project is selected the composer stays disabled and a toast surfaces
// "pick a project" on send (matching the previous behaviour — no modal
// gate).

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../i18n";
import { useToast } from "../toast";
import {
  api,
  ApiClientError,
  tokenProvider,
} from "../api/client";
import {
  ChatComposer as CanonicalComposer,
  type AccessMode,
} from "./ChatComposer";
import type { Project } from "../projects/useProjects";

interface Props {
  project: Project | null;
}

interface CreatedSession {
  id: string;
}

export function SessionlessComposer({ project }: Props) {
  const t = useT();
  const lp = useLocalizedPath();
  const router = useRouter();
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  const disabled = project === null;

  async function submit(
    text: string,
    modelId: string | undefined,
    _accessMode: AccessMode,
  ) {
    if (!project) {
      toast.error(t("projectSelector.needToPick"));
      return;
    }
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
      if (modelId) body.model = modelId;
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

  return (
    <CanonicalComposer
      busy={busy}
      onSend={submit}
      disabled={disabled}
      disabledHint={t("projectSelector.needToPick")}
    />
  );
}
