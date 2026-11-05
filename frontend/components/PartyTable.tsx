"use client";

import { useFlash, useParties } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import { num, pct, signed, titleCase } from "@/lib/format";
import type { HistoryView, Party } from "@/lib/types";

function Row({ p, max, showSeats, showCouncils, swing, candidate }: { p: Party; max: number; showSeats: boolean; showCouncils: boolean; swing?: number | null; candidate: boolean }) {
  const parties = useParties();
  const flash = useFlash(p.votes);
  return (
    <tr className={flash ? "mark-changed" : ""}>
      <td className="sm:w-full sm:max-w-0 min-w-24">
        <div className="flex items-baseline gap-1.5">
          <span className="swatch self-center" style={{ background: partyVar(p.id) }} aria-hidden />
          <span className="font-semibold truncate max-w-32 sm:max-w-48 shrink-0">{abbr(parties, p.id)}</span>
          <span className="label truncate hidden sm:inline">
            {candidate && p.candidate ? p.candidate : p.id > 0 ? titleCase(p.name) : ""}
          </span>
        </div>
        <div className="h-1.5 mt-1" aria-hidden>
          <div className="h-full" style={{ width: `${max ? (p.pct / max) * 100 : 0}%`, minWidth: 2, background: partyVar(p.id) }} />
        </div>
      </td>
      <td className="n font-semibold">{pct(p.pct)}</td>
      <td className="n hidden sm:table-cell">{num(p.votes)}</td>
      {swing !== undefined && <td className="n">{swing === null ? "–" : signed(swing)}</td>}
      {showSeats && <td className="n">{p.seats ?? 0}</td>}
      {showCouncils && <td className="n hidden sm:table-cell">{p.controlled ?? 0}</td>}
      {showCouncils && <td className="n hidden sm:table-cell">{p.largest ?? 0}</td>}
    </tr>
  );
}

export function PartyTable({ parties, seats = true, councils = false, history, candidates = false, limit }: {
  parties: Party[]; seats?: boolean; councils?: boolean; history?: HistoryView | null; candidates?: boolean; limit?: number;
}) {
  if (parties.length === 0) return <p className="cell border border-rule label !text-sm py-4">No results captured here yet.</p>;
  const shown = limit ? parties.slice(0, limit) : parties;
  const max = Math.max(...parties.map((p) => p.pct), 1);
  // "Other" is a different group of small parties each time, so it has no real swing.
  const swingOf = (id: number) => id === -2 ? null : history?.comparable ? (history.parties.find((s) => s.partyId === id)?.swing ?? null) : undefined;
  const showSwing = !!history?.comparable;
  return (
    <div className="overflow-x-auto">
      <table className="ledger">
        <thead>
          <tr>
            <th>Party</th>
            <th className="n">Share</th>
            <th className="n hidden sm:table-cell">Ballots</th>
            {showSwing && <th className="n" title="Change in share since the 2021 election, in percentage points">Since 2021</th>}
            {seats && <th className="n">Seats</th>}
            {councils && <th className="n hidden sm:table-cell" title="Councils where this party holds a majority of seats">Control</th>}
            {councils && <th className="n hidden sm:table-cell" title="Councils where this party has the most seats, with or without a majority">Largest</th>}
          </tr>
        </thead>
        <tbody>
          {shown.map((p) => <Row key={p.id} p={p} max={max} showSeats={seats} showCouncils={councils} swing={showSwing ? swingOf(p.id) : undefined} candidate={candidates} />)}
        </tbody>
      </table>
    </div>
  );
}
