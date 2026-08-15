export const API = "/api/v1";

export function apiV1(...parts: string[]): string {
  const cleaned = parts.map((p) => p.replace(/^\/+|\/+$/g, ""));
  return [API, ...cleaned].filter(Boolean).join("/");
}
