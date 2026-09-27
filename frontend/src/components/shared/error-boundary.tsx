"use client";

import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button } from "@/components/ui/button";

interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

function DefaultFallback({ onReset }: { onReset: () => void }) {
  return (
    <div
      role="alert"
      className="flex h-full min-h-48 flex-col items-center justify-center gap-3 p-8 text-center"
    >
      <p className="text-sm font-medium text-foreground">Something went wrong.</p>
      <p className="max-w-sm text-xs text-muted-foreground">
        This part of the workspace failed to render. Your other tabs remain
        available.
      </p>
      <Button variant="outline" size="sm" onClick={onReset}>
        Try again
      </Button>
    </div>
  );
}

/**
 * Reusable error boundary so a single failing component does not destroy the
 * whole workspace. Route-level failures are additionally handled by the App
 * Router `error.tsx` / `global-error.tsx` boundaries.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Keep diagnostics local; never log credentials or query payloads.
    console.error("Unhandled UI error:", error.message, info.componentStack);
  }

  reset = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return this.props.fallback ?? <DefaultFallback onReset={this.reset} />;
    }
    return this.props.children;
  }
}
