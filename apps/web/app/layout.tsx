import type { ReactNode } from "react";
import "./globals.css";

export const metadata = {
  title: "rocinante-harness",
  description: "AI agent product",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
