"use client";

import Link from "next/link";
import { useState } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { AreaTable } from "@/components/AreaTable";
import { RaceChart } from "@/components/Charts";
import { FeedList } from "@/components/FeedList";
import { Captured, Field, Section, StatusTag } from "@/components/Form";
import { PartyTable } from "@/components/PartyTable";
import { ResultsMap } from "@/components/ResultsMap";
import { SeatBar } from "@/components/SeatBar";
import { Watchlist } from "@/components/Watch";
import { num, pct } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import { useLive } from "@/lib/live";
import type { AreaSummary, CouncilRow, HistoryView, NationalView, TimelinePoint } from "@/lib/types";

type L = "province" | "municipality" | "ward";

const href = (level: L) => (r: AreaSummary) =>
  level === "province" ? `/province/${r.id}` : level === "municipality" ? `/municipality/${r.id}` : `/municipality/${r.parentId}/ward/${r.id}`;

export function NationalPage({ initial, initialMetros }: { initial: NationalView | null; initialMetros: CouncilRow[] | null }) {
  const { data: n } = useApi<NationalView>("/national", initial);
  const { data: metros } = useApi<CouncilRow[]>("/councils?metro=true", initialMetros);
  const { data: history } = useApi<HistoryView>("/history/national/0");
  const { data: timeline } = useApi<TimelinePoint[]>("/timeline/national/0");
  const { events } = useLive();
  const [level, setLevel] = useState<L>("municipality");

  if (!n) return <div className="skeleton h-96 grid place-items-center label">Loading the latest results&hellip;</div>;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:gap-x-8">
      <div className="lg:col-span-2">
        <AreaHeader title="South Africa" reportDate={n.reportDate} />
        <div className="cells sm:grid-cols-2 lg:grid-cols-[2fr_1fr_1fr_1fr]">
          <Captured progress={n.progress} />
          <Field label="Ballots counted" value={num(n.votes.cast)} hint={`${num(n.votes.spoilt)} spoilt`} />
          <Field label="Seats allocated" value={num(n.progress.seats)} hint={`of ${num(n.progress.seatsExpected)}`} />
          {n.tally && <Field label="Councils decided" value={`${n.tally.majority + n.tally.hung + n.tally.drawn} of ${n.progress.councilsExpected}`} hint={`${n.tally.majority} majority, ${n.tally.hung} hung, ${n.tally.drawn} drawn`} />}
        </div>
      </div>

      <div className="min-w-0 order-2 lg:order-none">
        <Section title="Results by area">
          <ResultsMap
            level={level} hrefFor={href(level)} onLevel={(l) => setLevel(l as L)}
            levels={[{ level: "province", label: "Provinces" }, { level: "municipality", label: "Municipalities" }, { level: "ward", label: "Wards" }]}
          />
        </Section>
      </div>

      <div className="min-w-0 order-1 lg:order-none">
        <Section title="Party results" aside="Share of valid ballots counted so far">
          <PartyTable parties={n.parties} councils history={history} />
        </Section>
      </div>

      <div className="min-w-0 order-3 lg:order-none">
        <Section title="Metros" aside={<Link className="underline" href="/councils">All councils</Link>}>
          <ul className="grid sm:grid-cols-2 gap-px bg-[var(--rule)] border border-rule">
            {(metros ?? []).map((m) => (
              <li key={m.id} className="cell">
                <div className="flex justify-between gap-2 items-start">
                  <Link href={`/municipality/${m.id}`} className="font-semibold hover:underline">{m.name}</Link>
                  <StatusTag status={m.council.status} />
                </div>
                <div className="label my-1.5">{pct(m.vdPct, 0)} of districts captured</div>
                <SeatBar council={m.council} parties={m.parties} />
              </li>
            ))}
          </ul>
        </Section>
      </div>

      <div className="min-w-0 order-4 lg:order-none">
        <Section title="Latest" aside={<Link className="underline" href="/feed">Feed</Link>}>
          <FeedList events={events} limit={10} />
        </Section>
      </div>

      <div className="min-w-0 order-5 lg:order-none">
        <Section title="The race so far" aside="Early figures shift as different areas report">
          <RaceChart points={timeline ?? []} />
        </Section>
      </div>

      <div className="min-w-0 order-6 lg:order-none">
        <Section title="Your watchlist"><Watchlist /></Section>
      </div>

      <div className="lg:col-span-2 order-7 lg:order-none">
        <Section title="Provinces">
          <AreaTable rows={n.provinces} hrefFor={(r) => `/province/${r.id}`} kind="provinces" filterable={false} />
        </Section>
      </div>
    </div>
  );
}
