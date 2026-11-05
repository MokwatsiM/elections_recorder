"use client";

import Link from "next/link";
import { clock } from "@/lib/format";
import { useParties } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import { describeEvent } from "@/lib/live";
import type { FeedEvent } from "@/lib/types";

const eventParty = (e: FeedEvent): number | null =>
  e.type === "ward.declared" ? e.data.partyId
  : e.type === "lead.changed" ? e.data.toId
  : e.type === "council.decided" ? (e.data.controlParty ?? e.data.largestParty ?? null)
  : null;

function hrefFor(e: FeedEvent) {
  const a = e.data?.area;
  if (e.type === "ward.declared") return `/municipality/${a.id}/ward/${e.data.wardId}`;
  if (a?.level === "municipality") return `/municipality/${a.id}`;
  return "/";
}

export function FeedList({ events, limit }: { events: FeedEvent[]; limit?: number }) {
  const parties = useParties();
  const shown = limit ? events.slice(0, limit) : events;
  if (shown.length === 0) return <p className="label py-3">Ward seats, council results and changes of lead appear here as the IEC publishes them.</p>;
  return (
    <ol className="divide-y divide-[var(--rule)] border-y border-rule" aria-live="off">
      {shown.map((e) => {
        const { title, detail } = describeEvent(e, (id) => abbr(parties, id));
        const p = eventParty(e);
        return (
          <li key={e.seq} className="py-1.5 flex gap-2 items-baseline">
            <time className="label shrink-0 w-10" dateTime={e.at}>{clock(e.at)}</time>
            <span className="swatch self-center" style={{ background: p !== null ? partyVar(p) : "transparent" }} aria-hidden />
            <span className="min-w-0 text-sm">
              <Link href={hrefFor(e)} className="font-medium hover:underline">{title}</Link>
              <span className="text-ink-2"> {detail}</span>
            </span>
          </li>
        );
      })}
    </ol>
  );
}
