"use client";

// Horizontal "step indicator" used by the onboarding flow.
// Renders N dots, with the active step highlighted and previous
// steps marked as done. The component is purely visual; the
// parent owns the active step state.

interface StepDotsProps {
  steps: string[];
  active: number;
  // Optional i18n label for accessibility. If omitted, the
  // component derives "Step X of Y" from props.
  stepLabel?: (index: number, total: number, name: string) => string;
}

export function StepDots({ steps, active, stepLabel }: StepDotsProps) {
  return (
    <ol
      className="flex items-center justify-center gap-2 mb-6"
      aria-label="Progress"
    >
      {steps.map((name, i) => {
        const done = i < active;
        const current = i === active;
        const label =
          stepLabel?.(i, steps.length, name) ??
          `Step ${i + 1} of ${steps.length}: ${name}`;
        return (
          <li
            key={name}
            className="flex items-center gap-2"
            aria-current={current ? "step" : undefined}
          >
            <span
              aria-hidden="true"
              className={
                "w-2.5 h-2.5 rounded-full " +
                (done
                  ? "bg-green-500"
                  : current
                  ? "bg-[var(--color-primary)] ring-2 ring-[var(--color-primary)]/30"
                  : "bg-zinc-500/30")
              }
            />
            <span
              className={
                "text-xs " +
                (current
                  ? "text-[var(--color-fg)] font-medium"
                  : done
                  ? "text-[var(--color-fg-muted)]"
                  : "text-[var(--color-fg-subtle)]")
              }
            >
              {name}
            </span>
            {i < steps.length - 1 && (
              <span
                aria-hidden="true"
                className="w-6 h-px bg-[var(--color-border)]"
              />
            )}
            <span className="sr-only">{label}</span>
          </li>
        );
      })}
    </ol>
  );
}
