import type { ReactNode } from "react";
import { Sidebar } from "../Sidebar";

export default async function AgentLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className="flex h-screen">
      <Sidebar activeId={id} />
      <div className="flex-1 flex flex-col min-w-0">{children}</div>
    </div>
  );
}
