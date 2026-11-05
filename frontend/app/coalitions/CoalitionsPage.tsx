"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { Section, StatusTag } from "@/components/Form";
import { useApi, useParties } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import type { CouncilRow, MunicipalityView } from "@/lib/types";

export function CoalitionsPage() {
  const router = useRouter();
  const sp = useSearchParams();
  const info = useParties();
  const { data: councils } = useApi<CouncilRow[]>("/councils");
  const fallback = useMemo(() => councils?.find((c) => c.council.status === "hung")?.id ?? councils?.[0]?.id, [councils]);
  const id = Number(sp.get("m")) || fallback;
  const { data: m } = useApi<MunicipalityView>(id ? `/municipalities/${id}` : null);
  const [picked, setPicked] = useState<{ muni?: number; ids: number[] }>({ ids: [] });
  const ids = picked.muni === id ? picked.ids : [];

  const seated = (m?.parties ?? []).filter((p) => (p.seats ?? 0) > 0).sort((a, b) => (b.seats ?? 0) - (a.seats ?? 0));
  const c = m?.council;
  const chosen = seated.filter((p) => ids.includes(p.id));
  const total = chosen.reduce((s, p) => s + (p.seats ?? 0), 0);
  const need = c?.majorityNeeded ?? 0;
  const toggle = (pid: number) => setPicked({ muni: id, ids: ids.includes(pid) ? ids.filter((x) => x !== pid) : [...ids, pid] });
  const incomplete = c && c.seatsAllocated < c.seatsTotal;

  return (
    <div className="max-w-4xl">
      <AreaHeader title="Coalition calculator" sub="A hypothetical tool for exploring seat arithmetic. It doesn't predict or report any agreement between parties." />
      <label className="block mb-4 max-w-md">
        <span className="label block mb-1">Council</span>
        <select className="input" value={id ?? ""} onChange={(e) => router.replace(`/coalitions?m=${e.target.value}`, { scroll: false })}>
          {(councils ?? []).map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
        </select>
      </label>
      {m && c && (
        <Section title={m.name} aside={<StatusTag status={c.status} />}>
          {incomplete && <p className="text-sm p-2 mb-3" style={{ background: "var(--warn-bg)", color: "var(--warn-ink)" }}>Only {c.seatsAllocated} of {c.seatsTotal} seats are allocated so far, so these totals will change.</p>}
          {seated.length === 0 ? <p className="label py-3">No seats have been allocated in this council yet.</p> : (
            <>
              <p className="label mb-2">Choose parties to combine:</p>
              <div className="flex flex-wrap gap-1.5 mb-5">
                {seated.map((p) => (
                  <button key={p.id} className="btn" aria-pressed={ids.includes(p.id)} onClick={() => toggle(p.id)}>
                    <span className="swatch" style={{ background: partyVar(p.id), outline: "1px solid var(--paper)" }} aria-hidden />
                    {abbr(info, p.id)} <span className="font-semibold">{p.seats}</span>
                  </button>
                ))}
              </div>
              <div className="flex items-baseline justify-between mb-1">
                <span className="display text-2xl" aria-live="polite">{total} of {c.seatsTotal} seats</span>
                <span className="text-sm font-semibold">{total >= need ? `Reaches the ${need} needed for a majority` : `${need - total} short of ${need}`}</span>
              </div>
              <div className="relative h-6 bg-[var(--no-result)]">
                <div className="absolute inset-0 flex gap-[2px]">
                  {chosen.map((p) => <div key={p.id} style={{ width: `${((p.seats ?? 0) / c.seatsTotal) * 100}%`, background: partyVar(p.id) }} />)}
                </div>
                <div className="absolute -top-1 -bottom-1 w-0.5 bg-ink" style={{ left: `${(need / c.seatsTotal) * 100}%` }} />
              </div>
            </>
          )}
        </Section>
      )}
    </div>
  );
}
