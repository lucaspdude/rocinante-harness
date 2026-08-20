"use client";

// PopoverMenu — PR-11. The single popover-menu primitive for the
// harness. Every action menu (workspace row, session row, model
// picker) renders through this so the keyboard contract, the
// shadow/radius recipe and the red-destructive rule are defined
// once.
//
// Implemented directly on a portal rather than a headless library:
// @base-ui/react is not a dependency of apps/web, and the menu
// contract here is small enough that the primitive is cheaper than
// the dependency. The keyboard + positioning rules live in
// ./menu-nav so they are unit-tested without a DOM.

import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import {
  menuKeyAction,
  nextIndex,
  positionMenu,
  type MenuAlign,
  type MenuItem,
  type MenuSide,
} from "./menu-nav";

export type { MenuItem, MenuAlign, MenuSide } from "./menu-nav";

export interface PopoverMenuProps {
  /**
   * The clickable element. Cloning is avoided: the trigger is
   * wrapped in a span with the button semantics applied to it, so
   * callers can pass any node (icon button, pill, text).
   */
  trigger: ReactNode;
  items: MenuItem[];
  /** Preferred side. Flips automatically when it would overflow. */
  side?: MenuSide;
  align?: MenuAlign;
  /** Accessible name for the trigger. */
  label?: string;
  /** Extra classes on the trigger button. */
  triggerClassName?: string;
  /** Min width of the popup in px. Reference uses 224 (min-w-56). */
  minWidth?: number;
  /** Rendered above the items — used by ModelPicker for its search box. */
  header?: ReactNode;
  /**
   * Set when `header` owns an input. The shell then leaves focus
   * alone on open so the caller's autoFocus survives, and arrow
   * keys typed in the header still drive the item list.
   */
  headerHasInput?: boolean;
  /** Rendered below the items — status lines, empty states. */
  footer?: ReactNode;
  /**
   * Replaces the default icon/label/shortcut row body. The shell
   * still owns the button, focus, keyboard and destructive color,
   * so a rich row (ModelPicker's multi-line model entry) keeps the
   * shared menu contract instead of forking its own popup.
   */
  renderItem?: (item: MenuItem, index: number) => ReactNode;
  onOpenChange?: (open: boolean) => void;
}

export function PopoverMenu({
  trigger,
  items,
  side = "bottom",
  align = "end",
  label,
  triggerClassName,
  minWidth = 224,
  header,
  headerHasInput = false,
  footer,
  renderItem,
  onOpenChange,
}: PopoverMenuProps) {
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(-1);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popupRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  const close = useCallback(
    (restoreFocus = true) => {
      setOpen(false);
      setFocused(-1);
      setPos(null);
      onOpenChange?.(false);
      if (restoreFocus) triggerRef.current?.focus();
    },
    [onOpenChange],
  );

  // Position after paint: the popup must be in the DOM before we
  // can measure it. useLayoutEffect avoids a visible jump from the
  // pre-measurement coordinates.
  useLayoutEffect(() => {
    if (!open) return;
    const t = triggerRef.current;
    const p = popupRef.current;
    if (!t || !p) return;
    const rect = t.getBoundingClientRect();
    const box = p.getBoundingClientRect();
    setPos(
      positionMenu({
        trigger: { top: rect.top, left: rect.left, width: rect.width, height: rect.height },
        menu: { width: Math.max(box.width, minWidth), height: box.height },
        side,
        align,
        viewport: { width: window.innerWidth, height: window.innerHeight },
      }),
    );
  }, [open, side, align, minWidth, items.length]);

  // Dismiss on outside pointer, scroll and resize. Scroll/resize
  // close rather than reposition: the menu is transient, and a
  // menu that chases a scrolling row reads as a bug.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      const target = e.target as Node;
      if (popupRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      close(false);
    }
    function onDismiss() {
      close(false);
    }
    document.addEventListener("mousedown", onPointerDown);
    window.addEventListener("resize", onDismiss);
    window.addEventListener("scroll", onDismiss, true);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("resize", onDismiss);
      window.removeEventListener("scroll", onDismiss, true);
    };
  }, [open, close]);

  // Move real DOM focus onto the focused row so screen readers
  // follow along and :focus-visible styling applies. When the menu
  // was opened by pointer (no row focused yet) focus lands on the
  // popup itself — otherwise focus would stay on the trigger and
  // the popup's keydown handler would never see Arrow/Esc/Enter.
  //
  // Gated on `pos`: until the popup has been measured it still
  // renders with visibility:hidden, and focus() on a hidden
  // subtree is silently dropped by the browser.
  useEffect(() => {
    if (!open || !pos) return;
    if (focused < 0) {
      // headerHasInput: hand initial focus to the header's field
      // (ModelPicker's search box) rather than the popup. React's
      // autoFocus does not fire reliably for portaled subtrees, so
      // the shell places focus explicitly.
      if (headerHasInput) {
        popupRef.current?.querySelector<HTMLElement>("input, textarea")?.focus();
      } else {
        popupRef.current?.focus();
      }
      return;
    }
    const el = popupRef.current?.querySelector<HTMLElement>(
      `[data-menu-index="${focused}"]`,
    );
    el?.focus();
  }, [open, focused, pos, headerHasInput]);

  function openWith(startIndex: number) {
    setOpen(true);
    setFocused(startIndex);
    onOpenChange?.(true);
  }

  function onTriggerKeyDown(e: React.KeyboardEvent) {
    if (open) return;
    if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openWith(nextIndex(items, -1, 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      openWith(nextIndex(items, -1, -1));
    }
  }

  function activate(index: number) {
    const item = items[index];
    if (!item || item.disabled) return;
    close();
    item.onSelect();
  }

  function onPopupKeyDown(e: React.KeyboardEvent) {
    const action = menuKeyAction(e.key, items, focused);
    if (action.type === "none") return;
    e.preventDefault();
    e.stopPropagation();
    if (action.type === "close") close();
    else if (action.type === "focus") setFocused(action.index);
    else activate(action.index);
  }

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        {...(open ? { "aria-controls": menuId } : {})}
        {...(label ? { "aria-label": label, title: label } : {})}
        onClick={() => (open ? close() : openWith(-1))}
        onKeyDown={onTriggerKeyDown}
        className={triggerClassName ?? "rh-button-ghost px-1.5 py-0.5 text-xs"}
      >
        {trigger}
      </button>
      {open && typeof document !== "undefined"
        ? createPortal(
            <div
              ref={popupRef}
              id={menuId}
              role="menu"
              tabIndex={-1}
              data-testid="popover-menu"
              aria-label={label ?? undefined}
              onKeyDown={onPopupKeyDown}
              style={{
                position: "fixed",
                top: pos?.top ?? 0,
                left: pos?.left ?? 0,
                minWidth,
                // Hidden until measured so the first paint never
                // flashes at the top-left corner.
                visibility: pos ? "visible" : "hidden",
                zIndex: 60,
                borderRadius: 12,
                boxShadow: "0 12px 32px 0 rgba(0,0,0,.28)",
              }}
              className="flex flex-col gap-0.5 p-1 bg-[var(--color-bg-card)] border border-[var(--color-border)]"
            >
              {header ? <div className="p-1">{header}</div> : null}
              {items.map((item, i) => (
                <button
                  key={item.id}
                  type="button"
                  role="menuitem"
                  data-menu-index={i}
                  data-destructive={item.destructive ? "true" : undefined}
                  disabled={item.disabled ?? false}
                  tabIndex={focused === i ? 0 : -1}
                  onMouseEnter={() => !item.disabled && setFocused(i)}
                  onClick={() => activate(i)}
                  className={`flex items-center gap-2 w-full text-left rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
                    item.destructive
                      ? "text-[var(--color-danger)] hover:bg-[var(--color-danger)]/10 focus:bg-[var(--color-danger)]/10"
                      : "text-[var(--color-fg)] hover:bg-[var(--color-bg-elevated)] focus:bg-[var(--color-bg-elevated)]"
                  } focus:outline-none`}
                >
                  {renderItem ? (
                    renderItem(item, i)
                  ) : (
                    <>
                      {item.icon ? (
                        <span className="shrink-0 inline-flex w-4 h-4 items-center justify-center">
                          {item.icon}
                        </span>
                      ) : null}
                      <span className="flex-1 truncate">{item.label}</span>
                      {item.shortcut ? (
                        <span className="shrink-0 text-[11px] text-[var(--color-fg-subtle)] font-mono">
                          {item.shortcut}
                        </span>
                      ) : null}
                      {item.selected ? (
                        <span className="shrink-0 text-[var(--color-fg-muted)]" aria-hidden="true">
                          <CheckIcon />
                        </span>
                      ) : null}
                    </>
                  )}
                </button>
              ))}
              {footer ? <div className="p-1">{footer}</div> : null}
            </div>,
            document.body,
          )
        : null}
    </>
  );
}

// Inline lucide-style stroked outlines at 16x16. lucide-react is
// not a dependency of apps/web; these are the handful of glyphs
// the menus actually use, copied to the same 24-viewBox / 2px
// stroke geometry so a later swap to lucide-react is visual parity.

function Svg({ children }: { children: ReactNode }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export function CheckIcon() {
  return (
    <Svg>
      <path d="M20 6 9 17l-5-5" />
    </Svg>
  );
}

export function PencilIcon() {
  return (
    <Svg>
      <path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z" />
    </Svg>
  );
}

export function TrashIcon() {
  return (
    <Svg>
      <path d="M3 6h18" />
      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" />
      <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
    </Svg>
  );
}

export function CopyIcon() {
  return (
    <Svg>
      <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
      <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
    </Svg>
  );
}

export function PlusIcon() {
  return (
    <Svg>
      <path d="M5 12h14" />
      <path d="M12 5v14" />
    </Svg>
  );
}

export function FolderIcon() {
  return (
    <Svg>
      <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
    </Svg>
  );
}

export function EyeOffIcon() {
  return (
    <Svg>
      <path d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.8 10.8 0 0 1-1.444 2.49" />
      <path d="M14.084 14.158a3 3 0 0 1-4.242-4.242" />
      <path d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143" />
      <path d="m2 2 20 20" />
    </Svg>
  );
}

export function EllipsisIcon() {
  return (
    <Svg>
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
      <circle cx="5" cy="12" r="1" />
    </Svg>
  );
}
