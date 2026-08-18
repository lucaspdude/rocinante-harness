import type { ReactNode } from "react";
import { Sidebar } from "../../../../lib/sidebar/Sidebar";
import { TopNav } from "../../../../lib/components/TopNav";

export default async function AgentLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className="flex h-screen flex-col">
      <TopNav />
      <div className="flex flex-1 min-h-0">
        <Sidebar activeId={id} />
        <div className="flex-1 flex flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
