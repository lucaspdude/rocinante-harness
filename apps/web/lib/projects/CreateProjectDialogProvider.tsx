"use client";

// PR-02: shared dialog opener. The top bar (ProjectSelectorBar) and the
// Sidebar's "+ New project" buttons both need to open the same dialog
// instance. They can't each keep their own state and rely on
// `CreateProjectDialog` props because the dialog lives once in the tree
// at the layout level. This provider lifts `open` to a single context so
// any consumer (top bar, sidebar, future menu item) calls `open()` and
// the same dialog renders.

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import { CreateProjectDialog } from "./CreateProjectDialog";

interface CreateProjectDialogCtx {
  open: () => void;
  close: () => void;
  isOpen: boolean;
}

// null when used outside the provider (e.g. legacy tests that render
// <Sidebar /> without the agent shell). Consumers should null-check.
const CtxImpl = createContext<CreateProjectDialogCtx | null>(null);

export function CreateProjectDialogProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const handleClose = useCallback(() => setOpen(false), []);
  const handleOpen = useCallback(() => setOpen(true), []);
  // After a project is created the dialog fires onCreated(path). We
  // forward that as a window event so the chat-home page can auto-
  // select the freshly-created project (the registry poll will pick
  // it up too, but auto-selecting makes the UX feel instant).
  const handleCreated = useCallback((path: string) => {
    if (typeof window !== "undefined") {
      window.dispatchEvent(
        new CustomEvent("rh:project:created", { detail: path }),
      );
    }
  }, []);
  const value = useMemo<CreateProjectDialogCtx>(
    () => ({ open: handleOpen, close: handleClose, isOpen: open }),
    [handleOpen, handleClose, open],
  );
  return (
    <CtxImpl.Provider value={value}>
      {children}
      <CreateProjectDialog
        open={open}
        onClose={handleClose}
        onCreated={handleCreated}
      />
    </CtxImpl.Provider>
  );
}

export function useCreateProjectDialog(): CreateProjectDialogCtx | null {
  return useContext(CtxImpl);
}
