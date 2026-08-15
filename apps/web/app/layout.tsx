import type { ReactNode } from "react";

export const metadata = {
  title: "rocinante-harness",
  description: "AI agent product",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html>
      <body>{children}</body>
    </html>
  );
}
