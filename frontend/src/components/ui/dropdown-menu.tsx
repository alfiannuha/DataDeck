"use client";

import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import type { ComponentPropsWithoutRef } from "react";

import { cn } from "@/lib/utils";

/**
 * Row/table context-menu primitive (shadcn-style wrapper around Radix
 * DropdownMenu; ADR-010 §10 menu family). Radix supplies WAI-ARIA menu roles,
 * roving focus, typeahead, Escape and outside-click handling.
 */
export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;

export function DropdownMenuContent({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        sideOffset={4}
        className={cn(
          "z-50 min-w-40 overflow-hidden rounded-md border border-border bg-panel-raised p-1 shadow-xl",
          className,
        )}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  );
}

/**
 * A menu item. Unavailable actions stay focusable with `aria-disabled` so
 * screen-reader users can hear the item and its inline explanation; selecting
 * a disabled item is a no-op (never a hidden mutation).
 */
export function DropdownMenuItem({
  className,
  disabled,
  onSelect,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Item> & {
  disabled?: boolean;
}) {
  return (
    <DropdownMenuPrimitive.Item
      aria-disabled={disabled || undefined}
      onSelect={(event) => {
        if (disabled) {
          event.preventDefault();
          return;
        }
        onSelect?.(event);
      }}
      className={cn(
        "relative flex cursor-pointer select-none flex-col gap-0.5 rounded-sm px-2 py-1.5 text-xs text-foreground outline-none focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none",
        disabled && "opacity-60 focus:bg-transparent focus:text-foreground",
        className,
      )}
      {...props}
    />
  );
}

export function DropdownMenuSeparator({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Separator>) {
  return (
    <DropdownMenuPrimitive.Separator
      className={cn("-mx-1 my-1 h-px bg-border", className)}
      {...props}
    />
  );
}

/** Muted explanation shown inside a disabled menu item. */
export function DropdownMenuHint({
  className,
  ...props
}: ComponentPropsWithoutRef<"span">) {
  return (
    <span
      className={cn("text-[10px] font-normal text-muted-foreground", className)}
      {...props}
    />
  );
}
