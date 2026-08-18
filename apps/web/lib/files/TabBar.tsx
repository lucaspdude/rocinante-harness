"use client";

// TabBar — list of open file tabs, scoped to a project.

import { useEffect } from "react";
import { useT } from "../i18n";

export interface Tab {
  path: string;
  name: string;
}

const MAX_TABS = 10;
const STORAGE_KEY = (root: string) =>
  `rh:tabs:${encodeURIComponent(root)}`;

export function loadTabs(root: string): Tab[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY(root));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (t): t is Tab =>
        typeof t === "object" && t !== null &&
        typeof (t as Tab).path === "string" &&
        typeof (t as Tab).name === "string"
    );
  } catch {
    return [];
  }
}

export function saveTabs(root: string, tabs: Tab[]): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY(root), JSON.stringify(tabs));
}

interface TabBarProps {
  root: string;
  tabs: Tab[];
  setTabs: (next: Tab[]) => void;
  activePath: string | null;
  onSelect: (path: string) => void;
  onClose: (path: string) => void;
}

export function TabBar({ root, tabs, setTabs, activePath, onSelect, onClose }: TabBarProps) {
  const t = useT();
  useEffect(() => {
    saveTabs(root, tabs);
  }, [root, tabs]);
  function addTab(path: string, name: string) {
    if (tabs.some((t) => t.path === path)) {
      onSelect(path);
      return;
    }
    const next = [...tabs, { path, name }];
    const trimmed = next.length > MAX_TABS ? next.slice(next.length - MAX_TABS) : next;
    setTabs(trimmed);
    onSelect(path);
  }
  if (tabs.length === 0) return null;
  return (
    <div role="tablist" className="flex items-center border-b overflow-x-auto text-xs">
      {tabs.map((tab) => (
        <div
          key={tab.path}
          role="tab"
          aria-selected={activePath === tab.path}
          data-active={activePath === tab.path}
          className={`flex items-center gap-1 px-2 py-1 border-r cursor-pointer ${
            activePath === tab.path
              ? "bg-[var(--color-bg-card)] font-medium"
              : "hover:bg-[var(--color-bg-card)]"
          }`}
          onClick={() => onSelect(tab.path)}
        >
          <span
            className="truncate max-w-[160px] font-mono"
            title={tab.path}
          >
            {tab.name}
          </span>
          <button
            type="button"
            aria-label={t("common.close")}
            onClick={(e) => {
              e.stopPropagation();
              onClose(tab.path);
            }}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}

export { STORAGE_KEY, MAX_TABS };
export const __TEST__ = { addTabMarker: 0 };
