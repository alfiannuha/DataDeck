"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/** Default / bounds / persistence key for the resizable sidebar (v0.2.0 UX). */
export const SIDEBAR_DEFAULT_WIDTH = 280;
export const SIDEBAR_MIN_WIDTH = 220;
export const SIDEBAR_MAX_WIDTH = 520;
export const SIDEBAR_WIDTH_STORAGE_KEY = "datadeck.sidebarWidth";
/** Keep at least this much room for the workspace so resizing never overflows. */
const MIN_WORKSPACE_WIDTH = 320;

function viewportWidth(): number {
  return typeof window === "undefined"
    ? SIDEBAR_MAX_WIDTH + MIN_WORKSPACE_WIDTH
    : window.innerWidth;
}

/** Clamp a width to the allowed range, also respecting the viewport. */
export function clampSidebarWidth(
  width: number,
  viewport: number = viewportWidth(),
): number {
  const viewportMax = Math.max(SIDEBAR_MIN_WIDTH, viewport - MIN_WORKSPACE_WIDTH);
  const upper = Math.max(
    SIDEBAR_MIN_WIDTH,
    Math.min(SIDEBAR_MAX_WIDTH, viewportMax),
  );
  return Math.min(Math.max(Math.round(width), SIDEBAR_MIN_WIDTH), upper);
}

function readStoredWidth(): number {
  if (typeof window === "undefined") return SIDEBAR_DEFAULT_WIDTH;
  try {
    const raw = window.localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY);
    const value = raw === null ? Number.NaN : Number(raw);
    return Number.isFinite(value)
      ? clampSidebarWidth(value)
      : SIDEBAR_DEFAULT_WIDTH;
  } catch {
    return SIDEBAR_DEFAULT_WIDTH;
  }
}

function persistWidth(width: number) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, String(width));
  } catch {
    // Storage disabled/quota: width still works for this session.
  }
}

/**
 * Resizable sidebar state (drag to resize, double-click to reset, persist to
 * localStorage). Pointer handling lives in the component so it can use
 * `setPointerCapture`; this hook exposes plain coordinate-driven methods that are
 * easy to test and free of DOM-event coupling.
 */
export function useResizableSidebar() {
  const [width, setWidthState] = useState(SIDEBAR_DEFAULT_WIDTH);
  const [resizing, setResizing] = useState(false);
  const widthRef = useRef(SIDEBAR_DEFAULT_WIDTH);
  const drag = useRef<{ startX: number; startWidth: number } | null>(null);

  const apply = useCallback((next: number) => {
    const clamped = clampSidebarWidth(next);
    widthRef.current = clamped;
    setWidthState(clamped);
  }, []);

  // Load the persisted width after mount (client-only → no hydration mismatch).
  useEffect(() => {
    apply(readStoredWidth());
  }, [apply]);

  // Re-clamp when the viewport changes so the sidebar always fits.
  useEffect(() => {
    const onResize = () => apply(widthRef.current);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [apply]);

  // Never leave a drag-selection or stuck cursor if unmounted mid-drag.
  useEffect(
    () => () => {
      if (typeof document !== "undefined") document.body.style.userSelect = "";
    },
    [],
  );

  const beginResize = useCallback((clientX: number) => {
    drag.current = { startX: clientX, startWidth: widthRef.current };
    setResizing(true);
    if (typeof document !== "undefined") document.body.style.userSelect = "none";
  }, []);

  const moveResize = useCallback(
    (clientX: number) => {
      if (!drag.current) return;
      apply(drag.current.startWidth + (clientX - drag.current.startX));
    },
    [apply],
  );

  const endResize = useCallback(() => {
    if (!drag.current) return;
    drag.current = null;
    setResizing(false);
    if (typeof document !== "undefined") document.body.style.userSelect = "";
    persistWidth(widthRef.current);
  }, []);

  const resetWidth = useCallback(() => {
    apply(SIDEBAR_DEFAULT_WIDTH);
    persistWidth(SIDEBAR_DEFAULT_WIDTH);
  }, [apply]);

  return { width, resizing, beginResize, moveResize, endResize, resetWidth };
}
