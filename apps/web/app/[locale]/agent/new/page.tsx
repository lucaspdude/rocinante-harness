"use client";

// PR-02: /agent/new is now a 5-line client redirect. The pre-PR
// version rendered the project picker as a modal gate; that was the
// entire problem the user reported ("the modal forced on users").
// /agent is the new chat-first home, so /agent/new just bounces
// there. Bookmark users still land somewhere; the URL is preserved
// in the browser history for back/forward navigation.

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useLocalizedPath } from "../../../../lib/i18n";

export default function NewSessionPage() {
  const router = useRouter();
  const lp = useLocalizedPath();
  useEffect(() => {
    router.replace(lp("/agent"));
  }, [router, lp]);
  return null;
}
