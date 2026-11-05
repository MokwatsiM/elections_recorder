"use client";

import { Bell, BellOff, Star } from "lucide-react";
import Link from "next/link";
import { useLive, eventMunicipality } from "@/lib/live";
import { useApi } from "@/lib/hooks";
import type { CouncilRow } from "@/lib/types";
import { pct } from "@/lib/format";
import { PartyMark, StatusTag } from "./Form";

export function WatchButton({ id }: { id: number }) {
  const { watchlist, toggleWatch } = useLive();
  const on = watchlist.includes(id);
  return (
    <button className="btn" aria-pressed={on} onClick={() => toggleWatch(id)}>
      <Star size={14} fill={on ? "currentColor" : "none"} aria-hidden />
      {on ? "Watching" : "Watch"}
    </button>
  );
}

export function Watchlist() {
  const { watchlist, notify, setNotify, events } = useLive();
  const { data } = useApi<CouncilRow[]>(watchlist.length ? "/councils" : null);
  if (watchlist.length === 0) {
    return <p className="label py-1">Follow your municipality: open it and choose Watch. You can get a browser notification when it changes while this tab is in the background.</p>;
  }
  const rows = (data ?? []).filter((r) => watchlist.includes(r.id));
  const supported = typeof window !== "undefined" && "Notification" in window;
  return (
    <div>
      <ul className="divide-y divide-[var(--rule)] border-y border-rule">
        {rows.map((r) => {
          const recent = events.filter((e) => eventMunicipality(e) === r.id).length;
          return (
            <li key={r.id} className="py-1.5 flex flex-wrap gap-x-3 gap-y-1 items-center justify-between">
              <Link href={`/municipality/${r.id}`} className="font-medium hover:underline">{r.name}</Link>
              <span className="flex items-center gap-2 text-sm">
                {r.parties[0] && r.parties[0].votes > 0 && <PartyMark id={r.parties[0].id} />}
                <span className="label">{pct(r.vdPct, 0)} captured{recent > 0 ? `, ${recent} new` : ""}</span>
                <StatusTag status={r.council.status} />
              </span>
            </li>
          );
        })}
      </ul>
      {supported && (
        <button className="btn mt-2" aria-pressed={notify} onClick={() => setNotify(!notify)}>
          {notify ? <Bell size={14} aria-hidden /> : <BellOff size={14} aria-hidden />}
          {notify ? "Notifications on" : "Notify me about these"}
        </button>
      )}
    </div>
  );
}
