"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { AreaHeader } from "@/components/AreaHeader";
import { FeedList } from "@/components/FeedList";
import { Section } from "@/components/Form";
import { Watchlist } from "@/components/Watch";
import { clientGet } from "@/lib/api";
import { eventMunicipality, useLive } from "@/lib/live";
import type { FeedEvent } from "@/lib/types";

const TYPES = [
  { key: "all", label: "Everything" }, { key: "council.decided", label: "Councils" },
  { key: "ward.declared", label: "Ward seats" }, { key: "lead.changed", label: "Changes of lead" }, { key: "watch", label: "My watchlist" },
];

export function FeedPage() {
  const { events: live, watchlist } = useLive();
  const [type, setType] = useState("all");
  const older = useInfiniteQuery({
    queryKey: ["feed-pages"],
    initialPageParam: 0,
    queryFn: ({ pageParam }) => clientGet<FeedEvent[]>(`/feed?limit=50${pageParam ? `&before=${pageParam}` : ""}`),
    getNextPageParam: (last) => (last.length === 50 ? last[last.length - 1].seq : undefined),
    refetchOnWindowFocus: false,
    staleTime: Infinity,
  });
  const all = useMemo(() => {
    const seen = new Set<number>();
    return [...live, ...(older.data?.pages.flat() ?? [])].filter((e) => (seen.has(e.seq) ? false : (seen.add(e.seq), true))).sort((a, b) => b.seq - a.seq);
  }, [live, older.data]);
  const shown = all.filter((e) => type === "all" || (type === "watch" ? watchlist.includes(eventMunicipality(e) ?? -1) : e.type === type));

  return (
    <div className="grid gap-8 lg:grid-cols-[minmax(0,7fr)_minmax(0,4fr)]">
      <div className="min-w-0">
        <AreaHeader title="Feed" sub="Each change as the IEC publishes it, newest first" />
        <div className="flex flex-wrap mb-3" role="group" aria-label="Filter the feed">
          {TYPES.map((t) => <button key={t.key} className="btn -ml-px first:ml-0" aria-pressed={type === t.key} onClick={() => setType(t.key)}>{t.label}</button>)}
        </div>
        <FeedList events={shown} />
        {older.hasNextPage && (
          <button className="btn mt-3" disabled={older.isFetchingNextPage} onClick={() => older.fetchNextPage()}>
            {older.isFetchingNextPage ? "Loading…" : "Show older"}
          </button>
        )}
      </div>
      <div className="min-w-0"><Section title="Your watchlist"><Watchlist /></Section></div>
    </div>
  );
}
