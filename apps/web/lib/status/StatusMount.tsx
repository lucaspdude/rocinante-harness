"use client";

// StatusMount is a client-only wrapper that calls useStatus and
// renders the pill. The layout (server component) imports this
// without pulling in the useStatus hook directly.

import { StatusPill } from "./StatusPill";
import { useStatus } from "./useStatus";

export function StatusMount() {
  const status = useStatus();
  return (
    <StatusPill
      kind={status.kind}
      apiVersion={status.apiVersion}
      ompVersion={status.ompVersion}
      lastOkAt={status.lastOkAt}
      lastFailAt={status.lastFailAt}
      lastError={status.lastError}
      recheck={status.recheck}
    />
  );
}
