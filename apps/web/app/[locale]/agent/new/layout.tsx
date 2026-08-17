import type { ReactNode } from "react";
import { Sidebar } from "../Sidebar";
import { TopNav } from "../../../../lib/components/TopNav";

// /agent/new is a transient page: it creates a session and
// router.replace()s to /agent/<id>. The Sidebar is rendered
// without an active id so nothing is highlighted.
export default function NewSessionLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-screen flex-col">
      <TopNav />
      <div className="flex flex-1 min-h-0">
        <Sidebar activeId="" />
        <div className="flex-1 flex flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
