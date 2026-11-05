"use client";

import { AreaHeader } from "@/components/AreaHeader";
import { AreaTable } from "@/components/AreaTable";
import { Breadcrumb } from "@/components/Breadcrumb";
import { RaceChart } from "@/components/Charts";
import { Captured, Field, Section } from "@/components/Form";
import { NotFoundNote } from "@/components/NotFoundNote";
import { PartyTable } from "@/components/PartyTable";
import { ResultsMap } from "@/components/ResultsMap";
import { num } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { AreaSummary, HistoryView, ProvinceView, TimelinePoint } from "@/lib/types";

const hrefFor = (r: AreaSummary) => `/municipality/${r.id}`;

export function ProvincePage({ id, initial }: { id: number; initial: ProvinceView | null }) {
  const { data: p, isError } = useApi<ProvinceView>(`/provinces/${id}`, initial);
  const { data: history } = useApi<HistoryView>(`/history/province/${id}`);
  const { data: timeline } = useApi<TimelinePoint[]>(`/timeline/province/${id}`);
  if (!p) return isError ? <NotFoundNote what="this province" /> : <div className="skeleton h-96" />;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:gap-x-8">
      <div className="lg:col-span-2">
        <Breadcrumb items={[{ href: "/", label: "South Africa" }, { label: p.name }]} />
        <AreaHeader title={p.name} reportDate={p.reportDate} />
        <div className="cells sm:grid-cols-2 lg:grid-cols-[2fr_1fr_1fr_1fr]">
          <Captured progress={p.progress} />
          <Field label="Ballots counted" value={num(p.votes.cast)} hint={`${num(p.votes.spoilt)} spoilt`} />
          <Field label="Seats allocated" value={num(p.progress.seats)} hint={`of ${num(p.progress.seatsExpected)}`} />
          {p.tally && <Field label="Councils decided" value={`${p.tally.majority + p.tally.hung + p.tally.drawn} of ${p.progress.councilsExpected}`} hint={`${p.tally.majority} majority, ${p.tally.hung} hung, ${p.tally.drawn} drawn`} />}
        </div>
      </div>
      <div className="min-w-0 order-2 lg:order-none">
        <Section title="Municipalities">
          <ResultsMap level="municipality" parent={id} focus={{ kind: "province", id }} hrefFor={hrefFor} />
        </Section>
      </div>
      <div className="min-w-0 order-1 lg:order-none">
        <Section title="Party results" aside="Share of valid ballots counted so far">
          <PartyTable parties={p.parties} councils history={history} />
        </Section>
      </div>
      <div className="lg:col-span-2 order-3 lg:order-none">
        <Section title="All municipalities">
          <AreaTable rows={p.municipalities} hrefFor={hrefFor} kind="municipalities" status />
        </Section>
      </div>
      <div className="lg:col-span-2 order-4 lg:order-none">
        <Section title="The race so far" aside="Early figures shift as different areas report">
          <RaceChart points={timeline ?? []} />
        </Section>
      </div>
    </div>
  );
}
