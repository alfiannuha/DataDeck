/**
 * Copy text to the clipboard. Returns false when the Clipboard API is
 * unavailable (e.g. insecure context or unsupported environment) instead of
 * throwing, so callers can decide on fallback UX.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (
      typeof navigator !== "undefined" &&
      typeof navigator.clipboard?.writeText === "function"
    ) {
      await navigator.clipboard.writeText(text);
      return true;
    }
    return false;
  } catch {
    return false;
  }
}
