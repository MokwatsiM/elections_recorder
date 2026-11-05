"use client";

import { useCallback } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { AreaTable } from "@/components/AreaTable";
import { Breadcrumb } from "@/components/Breadcrumb";
import { HistoryChart } from "@/components/Charts";
import { PartyMark, Section } from "@/components/Form";
import { NotFoundNote } from "@/components/NotFoundNote";
import { PartyTable } from "@/components/PartyTable";
import { ResultsMap } from "@/components/ResultsMap";
import { useApi } from "@/lib/hooks";
import type { AreaSummary, HistoryView, WardView } from "@/lib/types";

export function WardPage({ municId, wardId, initial }: { municId: number; wardId: number; initial: WardView | null }) {
  const { data: w, isError } = useApi<WardView>(`/municipalities/${municId}/wards/${wardId}`, initial);
  const { data: history } = useApi<HistoryView>(`/history/ward/${wardId}?municipality=${municId}`);
  const inWard = useCallback((r: AreaSummary) => r.parentId === wardId, [wardId]);
  const vdHref = useCallback((r: AreaSummary) => `/municipality/${municId}/ward/${wardId}/vd/${r.id}`, [municId, wardId]);

  if (!w) return isError ? <NotFoundNote what="this ward" /> : <div className="skeleton h-96" />;
  const reported = w.vds.filter((v) => v.reported).length;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:gap-x-8">
      <div className="lg:col-span-2">
        <Breadcrumb items={[{ href: "/", label: "South Africa" }, { href: `/province/${w.municipality.parentId}`, label: w.provinceName }, { href: `/municipality/${municId}`, label: w.municipality.name }, { label: `Ward ${w.no}` }]} />
        <AreaHeader title={`Ward ${w.no}`} sub={`${w.municipality.name}, ${reported} of ${w.vds.length} voting districts with results`} reportDate={w.reportDate} />
        <p className="text-sm">
          {w.winnerId !== undefined
            ? <span className="inline-flex items-center gap-1.5 flex-wrap">Ward seat allocated to <PartyMark id={w.winnerId} />{w.candidate && <span className="text-ink-2">({w.candidate})</span>}</span>
            : <span className="text-ink-2">The ward seat hasn&rsquo;t been allocated yet.</span>}
        </p>
      </div>
      <div className="min-w-0 order-2 lg:order-none">
        <Section title="Voting districts">
          <ResultsMap level="vd" parent={municId} scope={inWard} focus={{ kind: "ward", id: wardId }} hrefFor={vdHref} height="min(50vh, 460px)" />
        </Section>
      </div>
      <div className="min-w-0 order-1 lg:order-none">
        <Section title="Party results" aside="Ward and PR ballots counted so far">
          <PartyTable parties={w.parties} seats={false} candidates history={history} />
        </Section>
      </div>
      <div className="lg:col-span-2 order-3 lg:order-none">
        <Section title="All voting districts">
          <AreaTable rows={w.vds} hrefFor={vdHref} kind="voting districts" captured={false} />
        </Section>
      </div>
      {history && history.parties.length > 0 && (
        <div className="lg:col-span-2 order-4 lg:order-none">
          <Section title="Past elections" aside={history.note}><HistoryChart history={history} /></Section>
        </div>
      )}
    </div>
  );
}
