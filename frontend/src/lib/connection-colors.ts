/**
 * Connection color palette (PRF-03/A). Colors are presentation-only #RRGGBB
 * hints persisted with the profile. `DEFAULT_CONNECTION_COLOR` is the
 * backward-compatible fallback for records saved before colors existed.
 */

export interface ConnectionColorOption {
  name: string;
  value: string;
}

export const CONNECTION_COLORS: readonly ConnectionColorOption[] = [
  { name: "Blue", value: "#3b82f6" },
  { name: "Green", value: "#22c55e" },
  { name: "Purple", value: "#a855f7" },
  { name: "Orange", value: "#f97316" },
  { name: "Red", value: "#ef4444" },
  { name: "Teal", value: "#14b8a6" },
  { name: "Yellow", value: "#eab308" },
  { name: "Pink", value: "#ec4899" },
  { name: "Gray", value: "#6b7280" },
] as const;

export const DEFAULT_CONNECTION_COLOR = CONNECTION_COLORS[0].value;

/** Resolves a stored color to a valid hex value, falling back to the default. */
export function connectionColor(color?: string | null): string {
  return color && /^#[0-9a-fA-F]{6}$/.test(color)
    ? color.toLowerCase()
    : DEFAULT_CONNECTION_COLOR;
}
