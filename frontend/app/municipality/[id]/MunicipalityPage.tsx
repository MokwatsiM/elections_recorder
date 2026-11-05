"use client";

import Link from "next/link";
import { useCallback, useState } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { AreaTable } from "@/components/AreaTable";
import { Breadcrumb } from "@/components/Breadcrumb";
import { HistoryChart, RaceChart } from "@/components/Charts";
import { Captured, Field, PartyMark, Section, StatusTag } from "@/components/Form";
import { NotFoundNote } from "@/components/NotFoundNote";
import { PartyTable } from "@/components/PartyTable";
import { ResultsMap } from "@/components/ResultsMap";
import { SeatBar } from "@/components/SeatBar";
import { WatchButton } from "@/components/Watch";
import { num } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { AreaSummary, HistoryView, MunicipalityView, TimelinePoint } from "@/lib/types";

export function MunicipalityPage({ id, initial }: { id: number; initial: MunicipalityView | null }) {
  const { data: m, isError } = useApi<MunicipalityView>(`/municipalities/${id}`, initial);
  const { data: history } = useApi<HistoryView>(`/history/municipality/${id}`);
  const { data: timeline } = useApi<TimelinePoint[]>(`/timeline/municipality/${id}`);
  const [level, setLevel] = useState<"ward" | "vd">("ward");

  const wardHref = useCallback((r: AreaSummary) => `/municipality/${id}/ward/${r.id}`, [id]);
  const mapHref = useCallback(
    (r: AreaSummary) => (level === "ward" ? wardHref(r) : `/municipality/${id}/ward/${r.parentId}/vd/${r.id}`),
    [id, level, wardHref],
  );

  if (!m) return isError ? <NotFoundNote what="this municipality" /> : <div className="skeleton h-96" />;
  const c = m.council;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:gap-x-8">
      <div className="lg:col-span-2">
        <Breadcrumb items={[{ href: "/", label: "South Africa" }, { href: `/province/${m.parentId}`, label: m.provinceName }, { label: m.name }]} />
        <AreaHeader title={m.name} sub={m.isMetro ? "Metropolitan municipality" : "Local municipality"} reportDate={m.reportDate} action={<WatchButton id={id} />} />
        <div className="cells sm:grid-cols-2 lg:grid-cols-[2fr_1fr_1fr_1fr]">
          <Captured progress={m.progress} />
          <Field label="Ballots counted" value={num(m.votes.cast)} hint={`${num(m.votes.spoilt)} spoilt`} />
          <Field label="Wards with a seat allocated" value={num(m.progress.wards)} hint={`of ${num(m.progress.wardsExpected)}`} />
          {c && (
            <div className="cell">
              <div className="label mb-1">Council</div>
              <StatusTag status={c.status} />
              {c.status === "majority" && c.controlParty !== undefined && <div className="mt-1 text-sm flex items-center gap-1"><PartyMark id={c.controlParty} /> holds a majority</div>}
              {c.status === "hung" && (
                <div className="mt-1 text-sm">
                  {c.largestParty !== undefined && !c.largestTied ? <span className="inline-flex items-center gap-1"><PartyMark id={c.largestParty} /> is the largest, no majority</span> : "Largest parties are tied, no majority"}
                </div>
              )}
            </div>
          )}
        </div>
        {c && (
          <div className="mt-3">
            <div className="label mb-1">{num(c.seatsAllocated)} of {num(c.seatsTotal)} seats allocated</div>
            <SeatBar council={c} parties={m.parties} />
            {c.status !== "pending" && <p className="label mt-1"><Link className="underline" href={`/coalitions?m=${id}`}>Try coalition combinations</Link></p>}
          </div>
        )}
      </div>

      <div className="min-w-0 order-2 lg:order-none">
        <Section title="Results by area">
          <ResultsMap level={level} parent={id} focus={{ kind: "municipality", id }} hrefFor={mapHref}
            levels={[{ level: "ward", label: "Wards" }, { level: "vd", label: "Voting districts" }]} onLevel={(l) => setLevel(l as "ward" | "vd")} />
        </Section>
      </div>

      <div className="min-w-0 order-1 lg:order-none">
        <Section title="Party results" aside="Share of valid ballots counted so far">
          <PartyTable parties={m.parties} history={history} />
        </Section>
      </div>

      <div className="lg:col-span-2 order-3 lg:order-none">
        <Section title={`${m.wards.length} wards`}>
          <AreaTable rows={m.wards} hrefFor={wardHref} kind="wards" winner />
        </Section>
      </div>

      <div className="min-w-0 order-4 lg:order-none">
        <Section title="The race so far" aside="Early figures shift as different areas report">
          <RaceChart points={timeline ?? []} />
        </Section>
      </div>

      {history && history.parties.length > 0 && (
        <div className="min-w-0 order-5 lg:order-none">
          <Section title="Past elections" aside={history.note}>
            <HistoryChart history={history} />
          </Section>
        </div>
      )}
    </div>
  );
}
