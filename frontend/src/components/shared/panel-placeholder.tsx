import type { ReactNode } from "react";

/**
 * Explicit placeholder for regions whose real feature arrives in a later
 * milestone. It intentionally renders no fake functionality.
 */
export function PanelPlaceholder({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex h-full min-h-0 flex-col items-center justify-center gap-1 overflow-auto p-6 text-center">
      <p className="text-sm font-medium text-foreground">{title}</p>
      <p className="max-w-sm text-xs text-muted-foreground">{description}</p>
      {children}
    </div>
  );
}
