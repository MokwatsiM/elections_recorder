"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useFlash } from "@/lib/hooks";
import { pct } from "@/lib/format";
import type { AreaSummary } from "@/lib/types";
import { PartyMark, StatusTag } from "./Form";

type SortKey = "name" | "leader" | "captured";

function Row({ row, href, status, captured, winner }: { row: AreaSummary; href: string; status: boolean; captured: boolean; winner: boolean }) {
  const flash = useFlash(row.valid);
  return (
    <tr className={flash ? "mark-changed" : ""}>
      <td><Link href={href} className="font-medium underline-offset-2 hover:underline no-underline">{row.name}</Link></td>
      <td>
        {row.reported && row.leaderId !== undefined
          ? <span className="flex items-center gap-2 justify-between"><PartyMark id={row.leaderId} /><span>{pct(row.leaderPct)}</span></span>
          : <span className="label">No result yet</span>}
      </td>
      {captured && <td className="n">{row.vdPct !== undefined ? pct(row.vdPct, 0) : "–"}</td>}
      {winner && <td>{row.winnerId !== undefined ? <PartyMark id={row.winnerId} /> : <span className="label">Not allocated</span>}</td>}
      {status && <td>{row.status && <StatusTag status={row.status} />}</td>}
    </tr>
  );
}

export function AreaTable({ rows, hrefFor, kind, status = false, captured = true, winner = false, filterable = true }: {
  rows: AreaSummary[]; hrefFor: (r: AreaSummary) => string; kind: string; status?: boolean; captured?: boolean; winner?: boolean; filterable?: boolean;
}) {
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState<{ key: SortKey; dir: 1 | -1 }>({ key: "name", dir: 1 });
  const shown = useMemo(() => {
    const f = filter.trim().toLowerCase();
    const out = f ? rows.filter((r) => r.name.toLowerCase().includes(f)) : [...rows];
    const val = (r: AreaSummary) => (sort.key === "leader" ? (r.leaderPct ?? -1) : sort.key === "captured" ? (r.vdPct ?? -1) : 0);
    out.sort((a, b) => (sort.key === "name" ? a.name.localeCompare(b.name, undefined, { numeric: true }) : val(a) - val(b)) * sort.dir);
    return out;
  }, [rows, filter, sort]);

  const head = (key: SortKey, label: string, n = false) => (
    <th className={n ? "n" : ""} aria-sort={sort.key === key ? (sort.dir === 1 ? "ascending" : "descending") : undefined}>
      <button className="sort" onClick={() => setSort((s) => ({ key, dir: s.key === key ? (s.dir === 1 ? -1 : 1) : key === "name" ? 1 : -1 }))}>{label}</button>
    </th>
  );

  return (
    <div>
      {filterable && rows.length > 8 && (
        <input className="input text-sm mb-2 sm:max-w-xs" type="search" placeholder={`Filter ${rows.length} ${kind}`} aria-label={`Filter ${kind}`} value={filter} onChange={(e) => setFilter(e.target.value)} />
      )}
      <div className="overflow-x-auto">
        <table className="ledger">
          <thead>
            <tr>
              {head("name", "Area")}
              {head("leader", "Leading on votes")}
              {captured && head("captured", "Captured", true)}
              {winner && <th>Ward seat</th>}
              {status && <th>Council</th>}
            </tr>
          </thead>
          <tbody>
            {shown.map((r) => <Row key={r.id} row={r} href={hrefFor(r)} status={status} captured={captured} winner={winner} />)}
            {shown.length === 0 && <tr><td colSpan={5} className="label">No {kind} match &ldquo;{filter}&rdquo;.</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}
