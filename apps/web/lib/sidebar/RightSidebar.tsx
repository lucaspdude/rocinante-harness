"use client";

// RightSidebar — PR-09: shell + state machine for the Files/Changes
// panel beside the chat column. Width states per spec:
//   collapsed  → 56 px rail only
//   default    → 360 px content
//   wide       → 540 px content
// Active tab persists in localStorage as rh:right-sidebar-tab.

import { useEffect, useState } from "react";
import { useT } from "../i18n";
import { FileExplorer } from "../files/FileExplorer";
import { GitChangesPanel } from "../files/GitChangesPanel";
import { FileViewer } from "../files/FileViewer";
import { SearchPanel } from "../files/SearchPanel";
import { TabBar, loadTabs, type Tab } from "../files/TabBar";

export type WidthState = "collapsed" | "default" | "wide";
export type RightTab = "files" | "changes" | "search";

const WIDTH_KEY = "rh:right-sidebar-state";
const TAB_KEY = "rh:right-sidebar-tab";

interface RightSidebarProps {
  cwd: string | null;
}
export function RightSidebar({ cwd }: RightSidebarProps) {
// F10 (review followup): the initial useState value below
// ("default") matches PR-09 §D1's "expanded por default (state =
// `\"default\"`, 360 px) na primeira visita". localStorage hydration
// runs in the useEffect below; until that completes the rendered
// width is the spec default. Persistent collapse after a manual
// user action is intentional — the spec allows the user to
// keep their last view.
  const t = useT();
  const [widthState, setWidthState] = useState<WidthState>("default");
  const [activeTab, setActiveTab] = useState<RightTab>("files");
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [openPath, setOpenPath] = useState<string | null>(null);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const w = window.localStorage.getItem(WIDTH_KEY);
    if (w === "collapsed" || w === "wide") setWidthState(w);
    const tab = window.localStorage.getItem(TAB_KEY);
    if (tab === "files" || tab === "changes" || tab === "search") setActiveTab(tab);
    if (cwd) {
      const stored = loadTabs(cwd);
      if (stored.length > 0) {
        const last = stored[stored.length - 1]?.path ?? null;
        setTabs(stored);

        setOpenPath(last);
      }
    }
  }, [cwd]);

  // Persist state.
  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(WIDTH_KEY, widthState);
    }
  }, [widthState]);
  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(TAB_KEY, activeTab);
    }
  }, [activeTab]);

  function onRailClick(tab: RightTab) {
    if (widthState === "collapsed") {
      setWidthState("default");
      setActiveTab(tab);
      return;
    }
    if (activeTab === tab) {
      cycleWidth();
      return;
    }
    setActiveTab(tab);
  }

  function cycleWidth() {
    setWidthState((cur) => {
      if (cur === "default") return "wide";
      if (cur === "wide") return "collapsed";
      return "default";
    });
  }

  function toggleFromHeader() {
    setWidthState((cur) => (cur === "collapsed" ? "default" : "collapsed"));
  }

  if (widthState === "collapsed") {
    return (
      <aside className="w-14 border-l border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col items-center py-2 gap-2">
        <button
          type="button"
          onClick={() => onRailClick("files")}
          aria-label={t("rightSidebar.rail.files")}
          title={t("rightSidebar.rail.files")}
          className={`rh-button-ghost px-2 py-1 text-base ${activeTab === "files" && widthState !== "collapsed" ? "bg-blue-500/15" : ""}`}
        >
          📁
        </button>
        <button
          type="button"
          onClick={() => onRailClick("changes")}
          aria-label={t("rightSidebar.rail.changes")}
          title={t("rightSidebar.rail.changes")}
          className="rh-button-ghost px-2 py-1 text-base"
        >
          🪪
        </button>
        <button
          type="button"
          onClick={() => onRailClick("search")}
          aria-label={t("rightSidebar.rail.search")}
          title={t("rightSidebar.rail.search")}
          className="rh-button-ghost px-2 py-1 text-base"
        >
          🔎
        </button>
        <button
          type="button"
          onClick={() => setWidthState("default")}
          className="rh-button-ghost px-2 py-1 text-xs"
          aria-label={t("rightSidebar.expand")}
        >
          «
        </button>
      </aside>
    );
  }

  const widthClass = widthState === "wide" ? "w-[540px]" : "w-[360px]";

  return (
    <aside
      className={`${widthClass} border-l border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col h-full transition-all`}
    >
      <header className="flex items-center justify-between px-2 py-1 border-b">
        <div role="tablist" className="flex">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === "files"}
            data-active={activeTab === "files"}
            onClick={() => onRailClick("files")}
            className={`px-2 py-1 text-sm border-b-2 ${
              activeTab === "files"
                ? "border-blue-500 font-medium"
                : "border-transparent text-[var(--color-fg-muted)]"
            }`}
          >
            {t("rightSidebar.rail.files")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === "changes"}
            data-active={activeTab === "changes"}
            onClick={() => onRailClick("changes")}
            className={`px-2 py-1 text-sm border-b-2 ${
              activeTab === "changes"
                ? "border-blue-500 font-medium"
                : "border-transparent text-[var(--color-fg-muted)]"
            }`}
          >
            {t("rightSidebar.rail.changes")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === "search"}
            data-active={activeTab === "search"}
            onClick={() => onRailClick("search")}
            className={`px-2 py-1 text-sm border-b-2 ${
              activeTab === "search"
                ? "border-blue-500 font-medium"
                : "border-transparent text-[var(--color-fg-muted)]"
            }`}
          >
            {t("rightSidebar.rail.search")}
          </button>
        </div>
        <button
          type="button"
          onClick={toggleFromHeader}
          className="rh-button-ghost px-2 py-1 text-xs"
          aria-label={t("rightSidebar.collapse")}
        >
          »
        </button>
      </header>
      <div className="flex-1 min-h-0 flex flex-col">
        {activeTab === "files" && (
          <FilesBody
            cwd={cwd}
            tabs={tabs}
            setTabs={setTabs}
            openPath={openPath}
            setOpenPath={setOpenPath}
          />
        )}
        {activeTab === "changes" && <GitChangesPanel cwd={cwd} />}
        {activeTab === "search" && (
          <SearchPanel
            root={cwd}
            onOpenFile={(path, name) => {
              if (!tabs.some((tb) => tb.path === path)) {
                setTabs([...tabs, { path, name }]);
              }
              setActiveTab("files");
              setOpenPath(path);
            }}
          />
        )}
      </div>
    </aside>
  );
}

function FilesBody({
  cwd,
  tabs,
  setTabs,
  openPath,
  setOpenPath,
}: {
  cwd: string | null;
  tabs: Tab[];
  setTabs: (next: Tab[]) => void;
  openPath: string | null;
  setOpenPath: (path: string | null) => void;
}) {
  if (!cwd) {
    return (
      <div className="flex flex-col h-full">
        <EmptyState />
      </div>
    );
  }
  return (
    <div className="flex flex-col h-full">
      <TabBar
        root={cwd}
        tabs={tabs}
        setTabs={setTabs}
        activePath={openPath}
        onSelect={setOpenPath}
        onClose={(p) => {
          const next = tabs.filter((t) => t.path !== p);
          setTabs(next);
          if (openPath === p) {
            setOpenPath(next.length > 0 ? next[next.length - 1]?.path ?? null : null);
          }
        }}
      />
      {openPath ? (
        <FileViewer
          root={cwd}
          path={openPath}
          onClose={() => {
            const remaining = tabs.filter((t) => t.path !== openPath);
            setTabs(remaining);
            setOpenPath(remaining.length > 0 ? remaining[remaining.length - 1]?.path ?? null : null);
          }}
        />
      ) : (
        <FileExplorer
          root={cwd}
          onOpenFile={(path, name) => {
            if (!tabs.some((t) => t.path === path)) {
              setTabs([...tabs, { path, name }]);
            }
            setOpenPath(path);
          }}
        />
      )}
    </div>
  );
}

function EmptyState() {
  const t = useT();
  return (
    <p className="text-sm text-[var(--color-fg-muted)] p-3">
      {t("rightSidebar.noProject")}
    </p>
  );
}
