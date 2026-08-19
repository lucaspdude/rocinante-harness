"use client";

// PR-07: floating action bar shown at the bottom of the sidebar
// when at least one project is selected. Lets the user archive or
// delete multiple projects in one shot. Delete opens a small
// confirmation dialog that asks the user to type the project path
// to confirm the destructive op (the api's BulkHandler enforces the
// same check on the server side — see apps/api/internal/api/
// projects_handler.go).

import { useEffect, useState } from "react";
import { useT } from "../i18n";
import { useToast, extractError } from "../toast";
import {
  useProjects,
  type BulkProjectResult,
} from "../projects/useProjects";

interface BulkActionBarProps {
  selected: string[];
  allPaths: string[];
  onClear: () => void;
  onToggleSelectAll: () => void;
}

interface DeleteDialogProps {
  open: boolean;
  paths: string[];
  onCancel: () => void;
  onConfirm: (confirmPath: string) => Promise<void>;
}

function DeleteDialog({ open, paths, onCancel, onConfirm }: DeleteDialogProps) {
  const t = useT();
  const [input, setInput] = useState("");
  // For the prompt: show the first path (single delete) or a count
  // summary (multi delete — api still requires the literal path).
  const sample = paths[0] ?? "";
  const isMulti = paths.length > 1;

  useEffect(() => {
    if (open) setInput("");
  }, [open]);

  if (!open) return null;
  const matches = input === sample;

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div className="bg-[var(--color-bg-elevated)] border border-[var(--color-border)] rounded-md shadow-lg w-full max-w-md p-4 flex flex-col gap-3">
        <h3 className="text-sm font-medium">
          {t("projects.bulk.deleteTitle")}
        </h3>
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("projects.bulk.deleteBody")}
        </p>
        {isMulti ? (
          <p className="text-xs text-[var(--color-fg-subtle)]">
            {paths.length} paths: {sample}, ...
          </p>
        ) : null}
        <label className="text-xs flex flex-col gap-1">
          <span className="text-[var(--color-fg-muted)]">
            {t("projects.bulk.confirmPrompt", { path: sample })}
          </span>
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t("projects.bulk.deleteInputPlaceholder")}
            className="rh-input font-mono text-xs"
            autoFocus
          />
        </label>
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="rh-button-ghost px-3 py-1 text-xs"
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            disabled={!matches}
            onClick={() => {
              if (!matches) return;
              void onConfirm(sample);
            }}
            className="rh-button-primary px-3 py-1 text-xs disabled:opacity-50 bg-[var(--color-danger,#c0392b)]"
          >
            {t("common.confirm")}
          </button>
        </div>
      </div>
    </div>
  );
}

export function BulkActionBar({
  selected,
  allPaths,
  onClear,
  onToggleSelectAll,
}: BulkActionBarProps) {
  const t = useT();
  const toast = useToast();
  const { bulkArchive, bulkDelete } = useProjects(0, false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  if (selected.length === 0) return null;

  const allSelected =
    allPaths.length > 0 && selected.length === allPaths.length;

  const report = (res: BulkProjectResult, kind: "archive" | "delete") => {
    const ok = kind === "archive" ? res.archived ?? 0 : res.deleted ?? 0;
    const fail = res.errors?.length ?? 0;
    if (fail === 0) {
      toast.success(
        kind === "archive"
          ? t("projects.bulk.archived", { count: ok })
          : t("projects.bulk.deleted", { count: ok }),
      );
    } else {
      toast.warning(
        t("projects.bulk.partialFailTitle"),
        t("projects.bulk.partialFail", { done: ok, fail }),
      );
    }
  };

  const handleArchive = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const res = await bulkArchive(selected);
      report(res, "archive");
      onClear();
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (confirmPath: string) => {
    setBusy(true);
    try {
      const res = await bulkDelete(selected, confirmPath);
      report(res, "delete");
      setDialogOpen(false);
      onClear();
    } catch (e) {
      const { code } = extractError(e);
      if (code === "confirmation_required") {
        toast.info(t("projects.bulk.confirmMismatch"));
      } else {
        toast.error(e);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <div
        role="toolbar"
        aria-label={t("projects.bulk.archive", { count: selected.length })}
        className="fixed bottom-4 left-1/2 -translate-x-1/2 z-40 flex items-center gap-2 px-3 py-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] shadow-lg"
      >
        <span className="text-xs text-[var(--color-fg-muted)]">
          {selected.length}
        </span>
        <button
          type="button"
          onClick={onToggleSelectAll}
          className="rh-button-ghost text-xs px-2 py-1"
        >
          {allSelected
            ? t("projects.bulk.clear")
            : t("projects.bulk.selectAll")}
        </button>
        <button
          type="button"
          onClick={handleArchive}
          disabled={busy}
          className="rh-button-ghost text-xs px-2 py-1 disabled:opacity-50"
        >
          {t("projects.bulk.archive", { count: selected.length })}
        </button>
        <button
          type="button"
          onClick={() => setDialogOpen(true)}
          disabled={busy}
          className="rh-button-primary text-xs px-2 py-1 disabled:opacity-50 bg-[var(--color-danger,#c0392b)]"
        >
          {t("projects.bulk.delete", { count: selected.length })}
        </button>
      </div>
      <DeleteDialog
        open={dialogOpen}
        paths={selected}
        onCancel={() => {
          if (busy) return;
          setDialogOpen(false);
        }}
        onConfirm={handleDelete}
      />
    </>
  );
}
