"use client";

export default function GlobalError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="en">
      <body>
        <div
          role="alert"
          className="flex min-h-screen flex-col items-center justify-center gap-3 p-8 text-center"
          style={{ background: "#09090b", color: "#fafafa" }}
        >
          <p className="text-sm font-medium">
            DataDeck failed to start.
          </p>
          <button
            type="button"
            onClick={reset}
            className="rounded-md border border-zinc-700 px-3 py-1.5 text-sm"
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  );
}
