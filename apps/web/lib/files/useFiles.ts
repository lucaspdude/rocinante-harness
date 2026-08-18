"use client";

// useFiles hook — polls /api/v1/files for directory listing +
// /api/v1/git/status for changed files. PR-08.
//
// Review followup (F6): useFilesActive adds conditional polling
// (active parameter) so the right sidebar only refetches while the
// chat session is actively streaming or awaiting.

import { useEffect, useState } from "react";
import { api } from "../api/client";

export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mode: string;
  mod_time: string;
}

interface FilesListResponse {
  root: string;
  path: string;
  entries: FileEntry[];
}

interface GitStatusResponse {
  cwd: string;
  repo?: string;
  files: { path: string; status: string }[];
  clean: boolean;
}

export interface FilesList {
  entries: FileEntry[];
  loading: boolean;
  error: string | null;
}

export function useFiles(root: string, intervalMs = 5000): FilesList {
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!root) return;
    let cancelled = false;
    const url = `/api/v1/files?root=${encodeURIComponent(root)}&path=.`;
    async function load() {
      try {
        const res = await api.get<FilesListResponse>(url);
        if (cancelled) return;
        setEntries(res.entries ?? []);
        setError(null);
      } catch (e: unknown) {
        const err = e as { body?: { message?: string }; message?: string };
        if (!cancelled) setError(err.body?.message ?? err.message ?? "failed");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    setLoading(true);
    void load();
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(() => void load(), intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [root, intervalMs]);

  return { entries, loading, error };
}

// useFilesActive is the F6 conditional-polling version. When
// `active` is false, no setInterval is started; the hook only
// emits a single initial load (so the UI has data on first
// render) and stops.
export function useFilesActive(
  root: string,
  active: boolean,
  intervalMs = 5000
): FilesList {
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!root) return;
    let cancelled = false;
    let intervalId: ReturnType<typeof setInterval> | undefined;
    const url = `/api/v1/files?root=${encodeURIComponent(root)}&path=.`;
    async function load() {
      try {
        const res = await api.get<FilesListResponse>(url);
        if (cancelled) return;
        setEntries(res.entries ?? []);
        setError(null);
      } catch (e: unknown) {
        const err = e as { body?: { message?: string }; message?: string };
        if (!cancelled) setError(err.body?.message ?? err.message ?? "failed");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    setLoading(true);
    if (active && intervalMs > 0) {
      intervalId = setInterval(() => void load(), intervalMs);
    }
    return () => {
      cancelled = true;
      if (intervalId !== undefined) clearInterval(intervalId);
    };
  }, [root, intervalMs, active]);

  return { entries, loading, error };
}

export interface FileContent {
  text: string | null;
  binary: boolean;
  loading: boolean;
  error: string | null;
}

export function useFileContent(root: string, path: string): FileContent {
  const [text, setText] = useState<string | null>(null);
  const [binary, setBinary] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!root || !path) return;
    let cancelled = false;
    setText(null);
    setError(null);
    setLoading(true);
    const url = `/api/v1/files/content?root=${encodeURIComponent(root)}&path=${encodeURIComponent(path)}`;
    fetch(url, {
      headers: { Accept: "text/plain" },
      credentials: "include",
    })
      .then(async (res) => {
        if (cancelled) return;
        const ct = res.headers.get("Content-Type") ?? "";
        if (ct.includes("application/json")) {
          const body = await res.json();
          if (body?.kind === "binary" || body?.kind === "too_large") {
            setBinary(true);
            setLoading(false);
            return;
          }
        }
        if (!res.ok) {
          throw new Error(`http ${res.status}`);
        }
        const t = await res.text();
        if (cancelled) return;
        setText(t);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        const err = e as { message?: string };
        setError(err.message ?? "failed");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [root, path]);

  return { text, binary, loading, error };
}

export interface GitStatus {
  files: { path: string; status: string }[];
  clean: boolean;
  loading: boolean;
  error: string | null;
}

export function useGitStatus(
  cwd: string | null,
  intervalMs = 5000
): GitStatus {
  const [files, setFiles] = useState<{ path: string; status: string }[]>([]);
  const [clean, setClean] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!cwd) return;
    let cancelled = false;
    const url = `/api/v1/git/status?cwd=${encodeURIComponent(cwd)}`;
    async function load() {
      try {
        const res = await api.get<GitStatusResponse>(url);
        if (cancelled) return;
        setFiles(res.files ?? []);
        setClean(!!res.clean);
        setError(null);
      } catch (e: unknown) {
        const err = e as { body?: { message?: string }; message?: string };
        if (!cancelled) setError(err.body?.message ?? err.message ?? "failed");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    setLoading(true);
    void load();
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(() => void load(), intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [cwd, intervalMs]);

  return { files, clean, loading, error };
}
