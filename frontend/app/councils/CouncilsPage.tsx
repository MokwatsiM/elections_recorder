"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { Field, PartyMark, StatusTag } from "@/components/Form";
import { SeatBar } from "@/components/SeatBar";
import { WatchButton } from "@/components/Watch";
import { pct } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { CouncilRow, CouncilStatus, NationalView } from "@/lib/types";

type Filter = "all" | "metro" | CouncilStatus;
const FILTERS: { key: Filter; label: string }[] = [
  { key: "all", label: "All" }, { key: "metro", label: "Metros" }, { key: "majority", label: "Majority" },
  { key: "hung", label: "Hung" }, { key: "drawn", label: "Drawn" }, { key: "pending", label: "Not yet calculated" },
];

export function CouncilsPage({ initial, initialNational }: { initial: CouncilRow[] | null; initialNational: NationalView | null }) {
  const { data: rows } = useApi<CouncilRow[]>("/councils", initial);
  const { data: n } = useApi<NationalView>("/national", initialNational);
  const [filter, setFilter] = useState<Filter>("all");
  const [text, setText] = useState("");

  const shown = useMemo(() => {
    const t = text.trim().toLowerCase();
    return (rows ?? []).filter((r) =>
      (filter === "all" || (filter === "metro" ? r.isMetro : r.council.status === filter)) && (!t || r.name.toLowerCase().includes(t)),
    );
  }, [rows, filter, text]);

  return (
    <div>
      <AreaHeader title="Councils" sub="Who holds a majority in each of the 213 councils. Status comes from the IEC, not from us." />
      {n?.tally && (
        <div className="cells grid-cols-2 lg:grid-cols-4 mb-5">
          <Field label="Controlled by one party" value={n.tally.majority} hint="More than half the seats" />
          <Field label="Hung" value={n.tally.hung} hint="No party has more than half" />
          <Field label="Drawn" value={n.tally.drawn} hint="A party has exactly half" />
          <Field label="Not yet calculated" value={n.tally.pending} hint="Waiting on seat allocation" />
        </div>
      )}
      <div className="flex flex-wrap gap-2 items-center mb-3">
        <div className="flex flex-wrap" role="group" aria-label="Filter councils">
          {FILTERS.map((f) => <button key={f.key} className="btn -ml-px first:ml-0" aria-pressed={filter === f.key} onClick={() => setFilter(f.key)}>{f.label}</button>)}
        </div>
        <input className="input text-sm sm:max-w-56" type="search" placeholder="Filter by name" aria-label="Filter councils by name" value={text} onChange={(e) => setText(e.target.value)} />
        <span className="label ml-auto">{shown.length} shown</span>
      </div>
      <ul className="grid sm:grid-cols-2 xl:grid-cols-3 gap-px bg-[var(--rule)] border border-rule">
        {shown.map((r) => (
          <li key={r.id} className="cell">
            <div className="flex justify-between gap-2 items-start">
              <Link href={`/municipality/${r.id}`} className="font-semibold hover:underline min-w-0">{r.name}</Link>
              <StatusTag status={r.council.status} />
            </div>
            <div className="label mt-0.5 mb-1.5 flex flex-wrap gap-x-1.5 items-center">
              {pct(r.vdPct, 0)} captured
              {r.council.status === "majority" && r.council.controlParty !== undefined && <> &middot; <PartyMark id={r.council.controlParty} /> controls</>}
              {r.council.status === "hung" && r.council.largestParty !== undefined && <> &middot; <PartyMark id={r.council.largestParty} /> largest</>}
            </div>
            <SeatBar council={r.council} parties={r.parties} />
            {r.council.status === "hung" && <p className="label mt-1"><Link className="underline" href={`/coalitions?m=${r.id}`}>Try coalition combinations</Link></p>}
          </li>
        ))}
      </ul>
      {shown.length === 0 && <p className="label py-6 text-center">No councils match this filter yet.</p>}
    </div>
  );
}
