"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { AreaHeader } from "@/components/AreaHeader";
import { HistoryChart } from "@/components/Charts";
import { Section } from "@/components/Form";
import { signed, pct } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import { partyVar } from "@/lib/parties";
import type { AreaSummary, HistoryView, Level } from "@/lib/types";

export function ComparePage() {
  const router = useRouter();
  const sp = useSearchParams();
  const level = (sp.get("level") ?? "national") as Level;
  const id = Number(sp.get("id") ?? 0);
  const { data: provinces } = useApi<AreaSummary[]>("/map/province");
  const { data: munis } = useApi<AreaSummary[]>("/map/municipality");
  const { data: h, isError } = useApi<HistoryView>(`/history/${level}/${id}`);
  const value = `${level}:${id}`;

  return (
    <div>
      <AreaHeader title="Compare with past elections" sub="2026 figures are partial until the count ends, so they will move" />
      <label className="block mb-4 max-w-md">
        <span className="label block mb-1">Area</span>
        <select className="input" value={value} onChange={(e) => {
          const [l, i] = e.target.value.split(":");
          router.replace(`/compare?level=${l}&id=${i}`, { scroll: false });
        }}>
          <option value="national:0">South Africa</option>
          <optgroup label="Provinces">{(provinces ?? []).map((p) => <option key={p.id} value={`province:${p.id}`}>{p.name}</option>)}</optgroup>
          <optgroup label="Municipalities">{(munis ?? []).map((m) => <option key={m.id} value={`municipality:${m.id}`}>{m.name}</option>)}</optgroup>
        </select>
      </label>
      {isError && <p className="label">No history is available for this area.</p>}
      {h && (
        <div className="grid gap-8 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]">
          <Section title={h.name} aside={h.note}><HistoryChart history={h} /></Section>
          <Section title="Change since 2021" aside="Percentage points of share">
            {h.comparable ? (
              <table className="ledger">
                <thead><tr><th>Party</th><th className="n">2021</th><th className="n">2026 so far</th><th className="n">Change</th></tr></thead>
                <tbody>
                  {h.parties.filter((p) => p.current !== null).slice(0, 12).map((p) => {
                    const base = p.years.find((y) => y.year === 2021)?.pct;
                    return (
                      <tr key={p.partyId}>
                        <td><span className="inline-flex items-center gap-1.5"><span className="swatch" style={{ background: partyVar(p.partyId) }} />{p.abbr}</span></td>
                        <td className="n">{base === undefined ? "–" : pct(base)}</td>
                        <td className="n">{pct(p.current!)}</td>
                        <td className="n font-semibold">{p.swing === null || p.partyId === -2 ? "–" : signed(p.swing)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            ) : <p className="label py-2">This area has no comparable 2021 result.</p>}
          </Section>
        </div>
      )}
    </div>
  );
}
