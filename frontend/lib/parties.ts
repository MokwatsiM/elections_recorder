import type { PartyInfo } from "./types";

// Fixed per party so a colour never changes with rank. Eight is the most that stay
// distinguishable, so every other party is neutral and is always named in text.
const SLOTS: Record<number, string> = {
  7: "anc", 52: "da", 945: "eff", 3: "ifp", 957: "pa", 1511: "actionsa", 4: "vf", 1748: "mk",
};

export const partyVar = (id: number) => `var(--p-${SLOTS[id] ?? "other"})`;

/** Resolves a party's colour to a value MapLibre can use, for the current theme. */
export function partyColor(id: number) {
  return getComputedStyle(document.documentElement).getPropertyValue(`--p-${SLOTS[id] ?? "other"}`).trim();
}

export const hasOwnColour = (id: number) => id in SLOTS;

export function abbr(parties: PartyInfo[] | undefined, id: number, fallback?: string) {
  if (id === -2) return "Other";
  if (id === 0) return "Independents";
  const a = parties?.find((p) => p.id === id)?.abbr;
  return a || fallback || `Party ${id}`;
}
