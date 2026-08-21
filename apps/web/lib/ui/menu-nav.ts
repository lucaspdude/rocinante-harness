// Pure keyboard-navigation + item-normalization helpers behind
// PopoverMenu. Kept out of the .tsx so they can be unit-tested
// without a DOM (vitest runs in node here — jsdom is only an
// optional peer of vitest and is not installed).

export interface MenuItem {
  /** Stable id. Used as the React key and for focus tracking. */
  id: string;
  label: string;
  /** Leading 16x16 icon. Rendered before the label. */
  icon?: React.ReactNode;
  /** Renders the row in --color-danger (Delete, Archive, ...). */
  destructive?: boolean;
  /** Trailing shortcut hint, e.g. "⌘R". */
  shortcut?: string;
  /** Renders a trailing checkmark. Selection lives in the menu. */
  selected?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}

/**
 * enabledIndexes returns the positions of items that can receive
 * focus. Disabled rows are skipped by every keyboard motion so
 * ArrowDown never parks focus on a dead row.
 */
export function enabledIndexes(items: readonly MenuItem[]): number[] {
  const out: number[] = [];
  for (let i = 0; i < items.length; i++) {
    if (!items[i]?.disabled) out.push(i);
  }
  return out;
}

/**
 * nextIndex moves focus by `delta` (+1 / -1) through the enabled
 * rows, wrapping at both ends. `current` of -1 means "nothing is
 * focused yet", in which case ArrowDown lands on the first enabled
 * row and ArrowUp on the last — the standard menu behavior.
 *
 * Returns -1 when there is nothing focusable at all.
 */
export function nextIndex(
  items: readonly MenuItem[],
  current: number,
  delta: 1 | -1,
): number {
  const enabled = enabledIndexes(items);
  if (enabled.length === 0) return -1;
  const pos = enabled.indexOf(current);
  if (pos === -1) {
    return delta === 1 ? (enabled[0] as number) : (enabled[enabled.length - 1] as number);
  }
  // (+ length) keeps the modulo positive when wrapping backwards.
  const nextPos = (pos + delta + enabled.length) % enabled.length;
  return enabled[nextPos] as number;
}

export type MenuAction =
  | { type: "close" }
  | { type: "focus"; index: number }
  | { type: "activate"; index: number }
  | { type: "none" };

/**
 * menuKeyAction maps a keydown to the menu's response. Splitting
 * this out of the component keeps the key contract (Esc closes,
 * Arrows move, Enter/Space activate, Home/End jump) testable and
 * identical for every consumer.
 *
 * `current` is the focused index, or -1 when focus is on the popup
 * itself. Returns {type:"none"} for keys the menu does not own so
 * the caller can let them through (typing, Tab, ...).
 */
export function menuKeyAction(
  key: string,
  items: readonly MenuItem[],
  current: number,
): MenuAction {
  switch (key) {
    case "Escape":
      return { type: "close" };
    case "ArrowDown": {
      const i = nextIndex(items, current, 1);
      return i === -1 ? { type: "none" } : { type: "focus", index: i };
    }
    case "ArrowUp": {
      const i = nextIndex(items, current, -1);
      return i === -1 ? { type: "none" } : { type: "focus", index: i };
    }
    case "Home": {
      // current=-1 makes nextIndex land on the first enabled row.
      const i = nextIndex(items, -1, 1);
      return i === -1 ? { type: "none" } : { type: "focus", index: i };
    }
    case "End": {
      const i = nextIndex(items, -1, -1);
      return i === -1 ? { type: "none" } : { type: "focus", index: i };
    }
    case "Enter":
    case " ": {
      const item = current >= 0 ? items[current] : undefined;
      if (!item || item.disabled) return { type: "none" };
      return { type: "activate", index: current };
    }
    default:
      return { type: "none" };
  }
}

export type MenuSide = "top" | "right" | "bottom" | "left";
export type MenuAlign = "start" | "center" | "end";

export interface Rect {
  top: number;
  left: number;
  width: number;
  height: number;
}

export interface Viewport {
  width: number;
  height: number;
}

/**
 * positionMenu computes fixed-position coordinates for the popup
 * given the trigger rect, the popup size, the requested side/align
 * and the viewport. It flips to the opposite side when the
 * preferred side would overflow, then clamps into the viewport so
 * a menu near an edge is never clipped.
 *
 * Pure so the flip/clamp rules are unit-tested rather than
 * eyeballed in a browser.
 */
export function positionMenu(opts: {
  trigger: Rect;
  menu: { width: number; height: number };
  side: MenuSide;
  align: MenuAlign;
  viewport: Viewport;
  offset?: number;
  padding?: number;
}): { top: number; left: number; side: MenuSide } {
  const { trigger, menu, align, viewport } = opts;
  const offset = opts.offset ?? 4;
  const padding = opts.padding ?? 8;

  let side = opts.side;

  // Flip when the preferred side lacks room and the opposite side
  // has more. Only vertical/horizontal pairs swap.
  const roomBelow = viewport.height - (trigger.top + trigger.height);
  const roomAbove = trigger.top;
  const roomRight = viewport.width - (trigger.left + trigger.width);
  const roomLeft = trigger.left;
  const needV = menu.height + offset + padding;
  const needH = menu.width + offset + padding;

  if (side === "bottom" && roomBelow < needV && roomAbove > roomBelow) side = "top";
  else if (side === "top" && roomAbove < needV && roomBelow > roomAbove) side = "bottom";
  else if (side === "right" && roomRight < needH && roomLeft > roomRight) side = "left";
  else if (side === "left" && roomLeft < needH && roomRight > roomLeft) side = "right";

  let top: number;
  let left: number;

  if (side === "bottom" || side === "top") {
    top = side === "bottom" ? trigger.top + trigger.height + offset : trigger.top - menu.height - offset;
    left =
      align === "start"
        ? trigger.left
        : align === "end"
          ? trigger.left + trigger.width - menu.width
          : trigger.left + trigger.width / 2 - menu.width / 2;
  } else {
    left = side === "right" ? trigger.left + trigger.width + offset : trigger.left - menu.width - offset;
    top =
      align === "start"
        ? trigger.top
        : align === "end"
          ? trigger.top + trigger.height - menu.height
          : trigger.top + trigger.height / 2 - menu.height / 2;
  }

  // Clamp into the viewport. max(padding, ...) wins over the upper
  // clamp so a menu taller than the viewport still starts on-screen.
  left = Math.max(padding, Math.min(left, viewport.width - menu.width - padding));
  top = Math.max(padding, Math.min(top, viewport.height - menu.height - padding));

  return { top, left, side };
}
