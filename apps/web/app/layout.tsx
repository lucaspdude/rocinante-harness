import type { ReactNode } from "react";
import "./globals.css";
import { KeyboardShortcutsRoot } from "../lib/keyboard/KeyboardShortcutsRoot";

export const metadata = {
  title: "rocinante-harness",
  description: "AI agent product",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <KeyboardShortcutsRoot />
        {children}
      </body>
    </html>
  );
}
