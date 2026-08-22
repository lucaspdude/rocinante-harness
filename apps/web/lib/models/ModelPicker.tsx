"use client";

// ModelPicker — PR-02 dropdown that searches models.dev via
// /api/v1/models/catalog and lets the user pick a model id (sent
// to the api on session create).
//
// Review followup (10-review.md F5): renders the additional
// fields introduced in ModelsDevEntry (max_tokens, cache cost,
// reasoning, thinking_supported, auth_supported).

import { useState } from "react";
import { useLocale, useT } from "../i18n";
import { PopoverMenu, type MenuItem } from "../ui/PopoverMenu";
import { useModelCatalog, type ModelEntry } from "./useModelCatalog";

interface ModelPickerProps {
  value: string;
  onChange: (modelId: string) => void;
  /**
   * Optional priming value shown on first paint when `value` is
   * empty. The composer passes the user's last persisted model id
   * (PR-3) so the trigger label never flashes "Pick a model" before
   * the mount-effect restores state. Once the user opens the menu
   * the catalog-backed `value` controls the selected highlight.
   */
  defaultValue?: string;
}

// formatPrice returns a locale-aware string for a per-token USD
// price (input/output). When a converted amount + currency is
// supplied by the server (PR-11), we render that with the user's
// Intl.NumberFormat so $3.00 becomes R$ 15,30 for pt-BR. Falls
// back to the raw USD price with a (USD) suffix when no
// conversion was applied.
function formatPrice(opts: {
  locale: string;
  usd?: number;
  local?: number;
  currency?: string;
}): string {
  const { locale, usd, local, currency } = opts;
  if (local != null && currency) {
    try {
      return new Intl.NumberFormat(locale, {
        style: "currency",
        currency,
        maximumFractionDigits: 2,
      }).format(local);
    } catch {
      // Unknown currency code — fall through to USD formatting.
    }
  }
  if (usd == null) return "";
  return `${new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(usd)} (USD)`;
}

export function ModelPicker({ value, onChange, defaultValue = "" }: ModelPickerProps) {
  const t = useT();
  const locale = useLocale();
  const [query, setQuery] = useState("");
  const { models, loading, stale } = useModelCatalog(query, locale);

  // The catalog rows carry multi-line metadata, so they ride the
  // shared menu shell through renderItem: PopoverMenu keeps the
  // button, focus ring, keyboard contract and dismissal, while the
  // row body stays the rich model entry it already was.
  // items[i] is built from models[i], so renderItem resolves the
  // entry by index — no side lookup table to keep in sync.
  const items: MenuItem[] = models.map((m) => ({
    id: `${m.provider}:${m.id}`,
    label: m.id,
    selected: m.id === value,
    onSelect: () => {
      onChange(m.id);
      setQuery("");
    },
  }));

  return (
    <PopoverMenu
      label={t("composer.modelSearch")}
      side="top"
      align="start"
      minWidth={280}
      triggerClassName="rh-button-ghost text-xs w-full truncate text-left"
      onOpenChange={(open) => {
        if (!open) setQuery("");
      }}
      trigger={
        value || defaultValue ? (
          <span className="font-mono">{value || defaultValue}</span>
        ) : (
          <span className="text-[var(--color-fg-muted)]">
            {t("composer.modelPlaceholder")}
          </span>
        )
      }
      headerHasInput
      header={
        <>
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("composer.modelSearch")}
            aria-label={t("composer.modelSearch")}
            className="rh-input text-sm"
          />
          {stale && (
            <p className="text-xs text-[var(--color-fg-muted)] mt-1">
              {t("composer.modelStale")}
            </p>
          )}
        </>
      }
      footer={
        loading ? (
          <p className="text-xs text-[var(--color-fg-muted)]">
            {t("common.loading")}
          </p>
        ) : models.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-muted)]">
            {t("composer.modelEmpty")}
          </p>
        ) : null
      }
      items={items}
      renderItem={(_item, i) => {
        const m = models[i];
        if (!m) return null;
        return <ModelRow model={m} locale={locale} selected={m.id === value} />;
      }}
    />
  );
}

function ModelRow({
  model: m,
  locale,
  selected,
}: {
  model: ModelEntry;
  locale: string;
  selected: boolean;
}) {
  return (
    <span className="flex flex-col gap-0.5 w-full min-w-0" aria-selected={selected}>
      <span className="flex items-center justify-between gap-2">
        <span className="font-mono text-sm truncate">{m.id}</span>
        <span className="text-xs text-[var(--color-fg-muted)]">{m.provider}</span>
      </span>
      {m.name && m.name !== m.id && (
        <span className="text-xs text-[var(--color-fg-muted)] truncate">
          {m.name}
        </span>
      )}
      {m.context_length ? (
        <span className="text-xs text-[var(--color-fg-muted)]">
          {Math.round(m.context_length / 1000)}K ctx
        </span>
      ) : null}
      <span className="flex flex-wrap items-center gap-1 text-[10px] text-[var(--color-fg-muted)]">
        {m.cost_input != null ? (
          <span
            title={
              m.currency && m.currency !== "USD"
                ? `~${m.cost_input.toFixed(2)} USD / M tokens`
                : "USD / M tokens"
            }
            className="px-1 py-0.5 rounded bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
          >
            {formatPrice({
              locale,
              usd: m.cost_input,
              local: m.cost_input_local,
              currency: m.currency,
            })}
          </span>
        ) : null}
        {m.reasoning ? (
          <span
            title="Supports reasoning"
            className="px-1 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400"
          >
            reasoning
          </span>
        ) : null}
        {m.thinking_supported ? (
          <span
            title="Supports thinking"
            className="px-1 py-0.5 rounded bg-purple-500/10 text-purple-600 dark:text-purple-400"
          >
            thinking
          </span>
        ) : null}
        {m.cost_cache_read != null || m.cost_cache_write != null ? (
          <span
            title="Has cache pricing"
            className="px-1 py-0.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-300"
          >
            cache {(m.cost_cache_read ?? 0).toFixed(2)}/{(m.cost_cache_write ?? 0).toFixed(2)}
          </span>
        ) : null}
        {m.auth_supported ? (
          <span
            title="Supports /login auth"
            className="px-1 py-0.5 rounded bg-green-500/10 text-green-600 dark:text-green-400"
          >
            /login
          </span>
        ) : null}
      </span>
    </span>
  );
}
