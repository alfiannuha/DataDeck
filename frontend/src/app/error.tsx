"use client";

import { Button } from "@/components/ui/button";

export default function RouteError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div
      role="alert"
      className="flex min-h-screen flex-col items-center justify-center gap-3 p-8 text-center"
    >
      <p className="text-sm font-medium text-foreground">
        The workspace hit an unexpected error.
      </p>
      <p className="max-w-sm text-xs text-muted-foreground">
        {error.message || "An unknown error occurred."}
      </p>
      <Button variant="outline" size="sm" onClick={reset}>
        Reload workspace
      </Button>
    </div>
  );
}
