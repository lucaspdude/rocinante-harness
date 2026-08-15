import { ClientAgent } from "./ClientAgent";

export default function AgentPage({ params }: { params: Promise<{ id: string }> }) {
  return <AgentPageBody params={params} />;
}

async function AgentPageBody({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ClientAgent sessionId={id} />;
}
