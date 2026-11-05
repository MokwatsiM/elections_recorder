// South African number style: a space between thousands.
export const num = (n: number | undefined) =>
  (n ?? 0).toString().replace(/\B(?=(\d{3})+(?!\d))/g, " ");

export const pct = (n: number | undefined, digits = 1) => `${(n ?? 0).toFixed(digits)}%`;

export const signed = (n: number, digits = 1) => `${n > 0 ? "+" : n < 0 ? "−" : ""}${Math.abs(n).toFixed(digits)}`;

const time = new Intl.DateTimeFormat("en-ZA", { hour: "2-digit", minute: "2-digit", timeZone: "Africa/Johannesburg", hour12: false });
const dayTime = new Intl.DateTimeFormat("en-ZA", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: "Africa/Johannesburg", hour12: false });

export const clock = (iso: string) => time.format(new Date(iso));
export const stamp = (iso: string) => dayTime.format(new Date(iso));

const KEEP_UPPER = new Set(["ANC", "DA", "EFF", "IFP", "PA", "SACP", "VF", "MK", "UDM", "ACDP", "NFP", "COPE", "PAC", "GOOD", "AIC", "ATM", "IND"]);

/** The IEC writes party names in capitals; this is easier to read in tables. */
export function titleCase(name: string) {
  return name
    .split(" | ")[0]
    .toLowerCase()
    .replace(/(^|[\s/(-])(\p{L})/gu, (_, a, b) => a + b.toUpperCase())
    .split(" ")
    .map((w) => (KEEP_UPPER.has(w.toUpperCase()) ? w.toUpperCase() : w))
    .map((w) => (["Of", "The", "And", "For"].includes(w) ? w.toLowerCase() : w))
    .join(" ")
    .replace(/^./, (c) => c.toUpperCase());
}
