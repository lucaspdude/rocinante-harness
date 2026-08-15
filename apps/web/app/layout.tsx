import type { ReactNode } from "react";

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
