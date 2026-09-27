"use client";

import type { ComponentPropsWithoutRef } from "react";

import { cn } from "@/lib/utils";

export function Select({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<"select">) {
  return (
    <select
      className={cn(
        "h-9 w-full rounded-md border border-border bg-background px-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)] disabled:opacity-50",
        className,
      )}
      {...props}
    >
      {children}
    </select>
  );
}
