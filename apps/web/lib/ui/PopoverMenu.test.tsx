// Unit tests for the PopoverMenu keyboard + positioning contract.
// The component itself is a thin portal shell over these helpers;
// vitest runs in node here (jsdom is only an optional peer of
// vitest and is not installed), so the behavior users can observe
// — arrow-key motion, wrapping, disabled skipping, Esc, Enter,
// side flipping and viewport clamping — is asserted directly.
//
// Two focus rules live in the component effect rather than here
// (they need a real browser and were verified against one):
//   1. Focus is applied only after the popup has been measured.
//      Before that it renders visibility:hidden, and focus() on a
//      hidden subtree is silently dropped — which left the whole
//      keyboard contract dead after a pointer-open.
//   2. With headerHasInput the shell focuses the header field
//      instead of the popup, because React autoFocus does not
//      fire reliably for portaled subtrees.

import { describe, it, expect } from "vitest";
import {
  enabledIndexes,
  menuKeyAction,
  nextIndex,
  positionMenu,
  type MenuItem,
} from "./menu-nav";

function mk(
  id: string,
  extra: Partial<MenuItem> = {},
): MenuItem {
  return { id, label: id, onSelect: () => {}, ...extra };
}

const three = [mk("rename"), mk("copy"), mk("delete", { destructive: true })];

describe("enabledIndexes", () => {
  it("lists every index when nothing is disabled", () => {
    expect(enabledIndexes(three)).toEqual([0, 1, 2]);
  });

  it("omits disabled rows so focus never lands on them", () => {
    const items = [mk("a"), mk("b", { disabled: true }), mk("c")];
    expect(enabledIndexes(items)).toEqual([0, 2]);
  });

  it("is empty when every row is disabled", () => {
    expect(enabledIndexes([mk("a", { disabled: true })])).toEqual([]);
  });
});

describe("nextIndex", () => {
  it("lands on the first row when opening downward with no focus", () => {
    expect(nextIndex(three, -1, 1)).toBe(0);
  });

  it("lands on the last row when opening upward with no focus", () => {
    expect(nextIndex(three, -1, -1)).toBe(2);
  });

  it("advances one row at a time", () => {
    expect(nextIndex(three, 0, 1)).toBe(1);
    expect(nextIndex(three, 1, 1)).toBe(2);
  });

  it("wraps past the end back to the first row", () => {
    expect(nextIndex(three, 2, 1)).toBe(0);
  });

  it("wraps before the start to the last row", () => {
    expect(nextIndex(three, 0, -1)).toBe(2);
  });

  it("skips disabled rows in both directions", () => {
    const items = [mk("a"), mk("b", { disabled: true }), mk("c")];
    expect(nextIndex(items, 0, 1)).toBe(2);
    expect(nextIndex(items, 2, -1)).toBe(0);
  });

  it("returns -1 when there is nothing focusable", () => {
    expect(nextIndex([mk("a", { disabled: true })], -1, 1)).toBe(-1);
  });
});

describe("menuKeyAction", () => {
  it("closes on Escape regardless of focus", () => {
    expect(menuKeyAction("Escape", three, -1)).toEqual({ type: "close" });
    expect(menuKeyAction("Escape", three, 1)).toEqual({ type: "close" });
  });

  it("moves focus with ArrowDown and ArrowUp", () => {
    expect(menuKeyAction("ArrowDown", three, 0)).toEqual({ type: "focus", index: 1 });
    expect(menuKeyAction("ArrowUp", three, 1)).toEqual({ type: "focus", index: 0 });
  });

  it("jumps to the first row on Home and the last on End", () => {
    expect(menuKeyAction("Home", three, 2)).toEqual({ type: "focus", index: 0 });
    expect(menuKeyAction("End", three, 0)).toEqual({ type: "focus", index: 2 });
  });

  it("activates the focused row on Enter and Space", () => {
    expect(menuKeyAction("Enter", three, 2)).toEqual({ type: "activate", index: 2 });
    expect(menuKeyAction(" ", three, 0)).toEqual({ type: "activate", index: 0 });
  });

  it("does not activate when nothing is focused", () => {
    expect(menuKeyAction("Enter", three, -1)).toEqual({ type: "none" });
  });

  it("does not activate a disabled row", () => {
    const items = [mk("a", { disabled: true })];
    expect(menuKeyAction("Enter", items, 0)).toEqual({ type: "none" });
  });

  it("ignores keys the menu does not own so typing passes through", () => {
    expect(menuKeyAction("a", three, 0)).toEqual({ type: "none" });
    expect(menuKeyAction("Tab", three, 0)).toEqual({ type: "none" });
  });
});

describe("positionMenu", () => {
  const viewport = { width: 1000, height: 800 };
  const menu = { width: 224, height: 120 };

  it("places a bottom/end menu under the trigger, right-aligned", () => {
    const r = positionMenu({
      trigger: { top: 100, left: 500, width: 24, height: 24 },
      menu,
      side: "bottom",
      align: "end",
      viewport,
    });
    expect(r.side).toBe("bottom");
    expect(r.top).toBe(128); // 100 + 24 + 4 offset
    expect(r.left).toBe(300); // 500 + 24 - 224
  });

  it("left-aligns with align:start", () => {
    const r = positionMenu({
      trigger: { top: 100, left: 500, width: 24, height: 24 },
      menu,
      side: "bottom",
      align: "start",
      viewport,
    });
    expect(r.left).toBe(500);
  });

  it("flips to the top when there is no room below", () => {
    const r = positionMenu({
      trigger: { top: 740, left: 500, width: 24, height: 24 },
      menu,
      side: "bottom",
      align: "end",
      viewport,
    });
    expect(r.side).toBe("top");
    expect(r.top).toBe(616); // 740 - 120 - 4
  });

  it("flips to the bottom when there is no room above", () => {
    const r = positionMenu({
      trigger: { top: 10, left: 500, width: 24, height: 24 },
      menu,
      side: "top",
      align: "end",
      viewport,
    });
    expect(r.side).toBe("bottom");
  });

  it("flips right to left when the right edge is tight", () => {
    const r = positionMenu({
      trigger: { top: 100, left: 950, width: 24, height: 24 },
      menu,
      side: "right",
      align: "start",
      viewport,
    });
    expect(r.side).toBe("left");
  });

  it("clamps a menu that would overflow the left edge", () => {
    const r = positionMenu({
      trigger: { top: 100, left: 4, width: 24, height: 24 },
      menu,
      side: "bottom",
      align: "end",
      viewport,
    });
    expect(r.left).toBe(8); // padding, not -196
  });

  it("clamps a menu that would overflow the right edge", () => {
    const r = positionMenu({
      trigger: { top: 100, left: 980, width: 24, height: 24 },
      menu,
      side: "bottom",
      align: "start",
      viewport,
    });
    expect(r.left).toBe(768); // 1000 - 224 - 8
  });

  it("keeps a menu taller than the viewport on-screen", () => {
    const r = positionMenu({
      trigger: { top: 400, left: 500, width: 24, height: 24 },
      menu: { width: 224, height: 900 },
      side: "bottom",
      align: "end",
      viewport,
    });
    expect(r.top).toBe(8);
  });
});
