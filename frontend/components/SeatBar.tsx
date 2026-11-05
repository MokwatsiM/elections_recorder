"use client";

import { useParties } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import type { Council, Party } from "@/lib/types";

/** Seats won, drawn against the full chamber with the majority line marked. */
export function SeatBar({ council, parties, showLabels = true }: { council: Council; parties: Party[]; showLabels?: boolean }) {
  const info = useParties();
  const total = council.seatsTotal || 1;
  const withSeats = parties.filter((p) => (p.seats ?? 0) > 0).sort((a, b) => (b.seats ?? 0) - (a.seats ?? 0));
  const line = (council.majorityNeeded / total) * 100;
  return (
    <div>
      <div className="relative h-3.5 bg-[var(--no-result)]" role="img"
        aria-label={`${council.seatsAllocated} of ${council.seatsTotal} seats allocated: ${withSeats.map((p) => `${abbr(info, p.id)} ${p.seats}`).join(", ") || "none yet"}. ${council.majorityNeeded} needed for a majority.`}>
        <div className="absolute inset-0 flex gap-[2px]">
          {withSeats.map((p) => <div key={p.id} style={{ width: `${((p.seats ?? 0) / total) * 100}%`, background: partyVar(p.id) }} />)}
        </div>
        <div className="absolute -top-1 -bottom-1 w-0.5 bg-ink" style={{ left: `${line}%` }} />
      </div>
      {showLabels && (
        <div className="flex flex-wrap gap-x-2.5 gap-y-0.5 mt-1 label">
          {withSeats.slice(0, 5).map((p) => (
            <span key={p.id} className="inline-flex items-center gap-1"><span className="swatch" style={{ background: partyVar(p.id) }} />{abbr(info, p.id)} {p.seats}</span>
          ))}
          <span className="ml-auto">{council.majorityNeeded} for majority</span>
        </div>
      )}
    </div>
  );
}
