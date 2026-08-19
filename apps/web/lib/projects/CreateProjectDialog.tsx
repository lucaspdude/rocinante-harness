"use client";

// CreateProjectDialog — modal with 3 tabs (Folder / Clone / Empty)
// backed by the registry endpoints. PR-05 spec; PR-02 wires the
// DirectoryPicker into the "Folder" and "Empty" tabs.

import { useEffect, useState } from "react";
import { useT } from "../i18n";
import { useMe } from "../me/useMe";
import {
  useProjects,
  type CloneStartResponse,
  type CreateProjectInput,
} from "./useProjects";
import { DirectoryPicker } from "./DirectoryPicker";

type TabId = "folder" | "clone" | "empty";

export interface CreateProjectDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (projectPath: string) => void;
  initialTab?: TabId;
}

function expandHome(p: string, home: string): string {
  if (p === "~") return home;
  if (p.startsWith("~/")) return home + p.slice(1);
  return p;
}

function isAbsolute(p: string, home: string): boolean {
  const expanded = expandHome(p, home);
  return expanded.startsWith("/") || /^[a-zA-Z]:[\\/]/.test(expanded);
}

function noTraversal(p: string): boolean {
  const parts = p.split(/[\\/]+/).filter(Boolean);
  for (const part of parts) {
    if (part === "..") return false;
  }
  return true;
}

const folderNameRegex = /^[A-Za-z0-9._-]+$/;

export function CreateProjectDialog({
  open,
  onClose,
  onCreated,
  initialTab,
}: CreateProjectDialogProps) {
  const t = useT();
  const { me } = useMe();
  const home = me?.home ?? "/root";
  const { register, startClone, projects } = useProjects(open ? 5000 : 0);
  const [tab, setTab] = useState<TabId>(initialTab ?? "folder");
  const [folderPath, setFolderPath] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);
  const [emptyPath, setEmptyPath] = useState("");
  const [cloneUrl, setCloneUrl] = useState("");
  const [cloneParent, setCloneParent] = useState("");
  const [cloneFolder, setCloneFolder] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [cloneJob, setCloneJob] = useState<CloneStartResponse | null>(null);
  const [progress, setProgress] = useState<number>(0);
  const [phase, setPhase] = useState<string>("idle");

  useEffect(() => {
    if (open && initialTab) setTab(initialTab);
    if (!open) {
      setError(null);
      setBusy(false);
      setCloneJob(null);
      setProgress(0);
      setPhase("idle");
    }
  }, [open, initialTab]);

  // Subscribe to the clone SSE stream once a job is created.
  useEffect(() => {
    if (!cloneJob) return;
    const es = new EventSource(cloneJob.stream_url);
    es.addEventListener("progress", (ev) => {
      const m = ev as MessageEvent;
      try {
        const d = JSON.parse(m.data);
        if (typeof d.pct === "number") setProgress(d.pct);
      } catch {
        // ignore
      }
    });
    es.addEventListener("phase", (ev) => {
      const m = ev as MessageEvent;
      try {
        const d = JSON.parse(m.data);
        if (typeof d.phase === "string") setPhase(d.phase);
      } catch {
        // ignore
      }
    });
    es.addEventListener("registered", (ev) => {
      const m = ev as MessageEvent;
      try {
        const d = JSON.parse(m.data);
        const proj = d.project;
        if (proj && typeof proj.path === "string") {
          onCreated?.(proj.path);
        }
      } catch {
        // ignore
      }
    });
    es.addEventListener("complete", () => {
      setPhase("complete");
      setProgress(100);
      es.close();
      onCreated?.(cloneJob.target);
      setTimeout(() => {
        onClose();
      }, 600);
    });
    es.addEventListener("fail", (ev) => {
      const m = ev as MessageEvent;
      try {
        const d = JSON.parse(m.data);
        setError(d.error ?? t("projects.clone.failed"));
      } catch {
        setError(t("projects.clone.failed"));
      }
      es.close();
    });
    es.onerror = () => {
      es.close();
    };
    return () => es.close();
  }, [cloneJob, onCreated, onClose, t]);

  if (!open) return null;

  async function doRegister(input: CreateProjectInput) {
    setBusy(true);
    setError(null);
    try {
      const p = await register(input);
      onCreated?.(p.path);
      onClose();
    } catch (e: unknown) {
      const err = e as { body?: { message?: string; code?: string }; message?: string };
      setError(err.body?.message ?? err.message ?? "failed");
    } finally {
      setBusy(false);
    }
  }

  async function doClone() {
    if (!cloneUrl || !cloneParent) {
      setError(t("projects.clone.missingFields"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const input: { url: string; parent_path: string; folder_name?: string } = {
        url: cloneUrl,
        parent_path: cloneParent,
      };
      if (cloneFolder) input.folder_name = cloneFolder;
      const res = await startClone(input);
      setCloneJob(res);
      setPhase("starting");
      setProgress(0);
    } catch (e: unknown) {
      const err = e as { body?: { message?: string; code?: string }; message?: string };
      setError(err.body?.message ?? err.message ?? "failed");
    } finally {
      setBusy(false);
    }
  }

  function submitFolder() {
    if (!isAbsolute(folderPath, home) || !noTraversal(folderPath)) {
      setError(t("projects.errors.invalidPath"));
      return;
    }
    const expanded = expandHome(folderPath, home);
    void doRegister({ path: expanded, name: basename(expanded) });
  }
  function submitEmpty() {
    if (!isAbsolute(emptyPath, home) || !noTraversal(emptyPath)) {
      setError(t("projects.errors.invalidPath"));
      return;
    }
    const expanded = expandHome(emptyPath, home);
    if (!folderNameRegex.test(basename(expanded))) {
      setError(t("projects.errors.invalidFolderName"));
      return;
    }
    void doRegister({ path: expanded, name: basename(expanded) });
  }
  function submitClone() {
    if (cloneFolder && !folderNameRegex.test(cloneFolder)) {
      setError(t("projects.errors.invalidFolderName"));
      return;
    }
    void doClone();
  }

  const tabBtn = (id: TabId, label: string) => (
    <button
      key={id}
      type="button"
      onClick={() => setTab(id)}
      disabled={busy || phase === "starting"}
      data-active={tab === id}
      className={`px-3 py-1.5 text-sm border-b-2 ${
        tab === id
          ? "border-blue-500 font-medium"
          : "border-transparent text-[var(--color-fg-muted)]"
      }`}
    >
      {label}
    </button>
  );

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-project-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    >
      <div className="rh-card w-full max-w-lg max-h-[90vh] flex flex-col overflow-hidden">
        <header className="flex items-center justify-between mb-3">
          <h2 id="create-project-title" className="text-base font-medium">
            {t("projects.title")}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            ×
          </button>
        </header>

        <div role="tablist" className="flex gap-1 border-b mb-3">
          {tabBtn("folder", t("projects.tab.folder"))}
          {tabBtn("clone", t("projects.tab.clone"))}
          {tabBtn("empty", t("projects.tab.empty"))}
        </div>

        {error && (
          <p role="alert" className="rh-error mb-3">
            {error}
          </p>
        )}

        {tab === "folder" && (
          <div className="flex flex-col gap-3">
            <label className="rh-label" htmlFor="folder-path">
              {t("projects.field.folderPath")}
            </label>
            <div className="flex gap-2">
              <input
                id="folder-path"
                type="text"
                value={folderPath}
                onChange={(e) => setFolderPath(e.target.value)}
                placeholder="~/projects/my-app"
                className="rh-input font-mono text-sm flex-1"
                disabled={busy}
              />
              <button
                type="button"
                onClick={() => setPickerOpen(true)}
                disabled={busy}
                className="rh-button-ghost text-sm"
                aria-haspopup="dialog"
              >
                {t("projects.folderPicker.pickButton")}
              </button>
            </div>
            <button
              type="button"
              onClick={submitFolder}
              disabled={busy || !folderPath}
              className="rh-button-primary text-sm self-end"
            >
              {busy ? t("common.loading") : t("projects.action.register")}
            </button>
          </div>
        )}

        {tab === "clone" && !cloneJob && (
          <div className="flex flex-col gap-3">
            <label className="rh-label" htmlFor="clone-url">
              {t("projects.field.cloneUrl")}
            </label>
            <input
              id="clone-url"
              type="text"
              value={cloneUrl}
              onChange={(e) => setCloneUrl(e.target.value)}
              placeholder="https://github.com/owner/repo"
              className="rh-input font-mono text-sm"
              disabled={busy}
            />
            <label className="rh-label" htmlFor="clone-parent">
              {t("projects.field.parentPath")}
            </label>
            <div className="flex gap-2">
              <input
                id="clone-parent"
                type="text"
                value={cloneParent}
                onChange={(e) => setCloneParent(e.target.value)}
                placeholder="~/projects"
                className="rh-input font-mono text-sm flex-1"
                disabled={busy}
              />
              <button
                type="button"
                onClick={() => setPickerOpen(true)}
                disabled={busy}
                className="rh-button-ghost text-sm"
                aria-haspopup="dialog"
              >
                {t("projects.folderPicker.pickButton")}
              </button>
            </div>
            <label className="rh-label" htmlFor="clone-folder">
              {t("projects.field.folderName")}
            </label>
            <input
              id="clone-folder"
              type="text"
              value={cloneFolder}
              onChange={(e) => setCloneFolder(e.target.value)}
              placeholder="repo (auto from url)"
              className="rh-input font-mono text-sm"
              disabled={busy}
            />
            <button
              type="button"
              onClick={submitClone}
              disabled={busy || !cloneUrl || !cloneParent}
              className="rh-button-primary text-sm self-end"
            >
              {busy ? t("common.loading") : t("projects.action.clone")}
            </button>
          </div>
        )}

        {tab === "clone" && cloneJob && (
          <div className="flex flex-col gap-3">
            <p className="text-sm">
              {t("projects.clone.phase")}: <span className="font-mono">{phase}</span>
            </p>
            <div className="h-2 bg-zinc-200 rounded overflow-hidden">
              <div
                className="h-full bg-blue-500 transition-all"
                style={{ width: `${progress}%` }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={progress}
              />
            </div>
            <p className="text-xs text-[var(--color-fg-muted)] font-mono break-all">
              {cloneJob.target}
            </p>
          </div>
        )}

        {tab === "empty" && (
          <div className="flex flex-col gap-3">
            <label className="rh-label" htmlFor="empty-path">
              {t("projects.field.emptyPath")}
            </label>
            <div className="flex gap-2">
              <input
                id="empty-path"
                type="text"
                value={emptyPath}
                onChange={(e) => setEmptyPath(e.target.value)}
                placeholder="~/projects/new-app"
                className="rh-input font-mono text-sm flex-1"
                disabled={busy}
              />
              <button
                type="button"
                onClick={() => setPickerOpen(true)}
                disabled={busy}
                className="rh-button-ghost text-sm"
                aria-haspopup="dialog"
              >
                {t("projects.folderPicker.pickButton")}
              </button>
            </div>
            <button
              type="button"
              onClick={submitEmpty}
              disabled={busy || !emptyPath}
              className="rh-button-primary text-sm self-end"
            >
              {busy ? t("common.loading") : t("projects.action.create")}
            </button>
          </div>
        )}

        <DirectoryPicker
          open={pickerOpen}
          onCancel={() => setPickerOpen(false)}
          onSelect={(path) => {
            // Set the path on whichever input corresponds to
            // the active tab so the user can immediately click
            // Register/Create. The picker already returns an
            // absolute path; we don't try to keep the "~" form.
            if (tab === "folder") setFolderPath(path);
            else if (tab === "empty") setEmptyPath(path);
            else if (tab === "clone") setCloneParent(path);
            setPickerOpen(false);
          }}
          registeredPaths={projects.map((p) => p.path)}
          busy={busy}
        />
        <footer className="mt-auto pt-3 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rh-button-ghost text-sm"
            disabled={busy && phase === "starting"}
          >
            {t("common.close")}
          </button>
        </footer>
      </div>
    </div>
  );
}

function basename(p: string): string {
  if (!p) return "";
  const parts = p.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] ?? "";
}
