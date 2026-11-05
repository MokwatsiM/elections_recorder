"use client";

import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { usePathname } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocalStorage } from "./hooks";
import type { FeedEvent, Status } from "./types";

const FEED_TYPES = ["ward.declared", "council.decided", "lead.changed", "seats.changed", "progress.milestone"];
const KEEP_EVENTS = 100;
// Refetches are spread out so every viewer doesn't hit the API in the same second.
const REFRESH_MIN_MS = 8_000;
const REFRESH_JITTER_MS = 5_000;

export type Connection = "off" | "connecting" | "live" | "reconnecting";

interface LiveValue {
  connection: Connection;
  status: Status | null;
  events: FeedEvent[];
  updatedAt: number | null;
  /** When this browser last fetched fresh figures while live updates were off. */
  checkedAt: number | null;
  live: boolean;
  setLive: (on: boolean) => void;
  refresh: () => Promise<void>;
  refreshing: boolean;
  refreshFailed: boolean;
  watchlist: number[];
  toggleWatch: (id: number) => void;
  notify: boolean;
  setNotify: (on: boolean) => void;
}

const LiveContext = createContext<LiveValue | null>(null);

export function useLive() {
  const v = useContext(LiveContext);
  if (!v) throw new Error("useLive must be used inside LiveProvider");
  return v;
}

export function eventMunicipality(e: FeedEvent): number | null {
  const area = e.data?.area;
  return area?.level === "municipality" ? area.id : null;
}

export function describeEvent(e: FeedEvent, abbrOf: (id: number) => string): { title: string; detail: string } {
  const d = e.data;
  switch (e.type) {
    case "ward.declared":
      return { title: `${d.area.name}, Ward ${d.wardNo}`, detail: `Ward seat allocated to ${abbrOf(d.partyId)}${d.candidate ? ` (${d.candidate})` : ""}` };
    case "council.decided": {
      const what =
        d.to === "majority" ? `${abbrOf(d.controlParty)} holds a majority`
        : d.to === "hung" ? `Hung council, no party has a majority${d.largestParty ? `. Largest party: ${abbrOf(d.largestParty)}` : ""}`
        : d.to === "drawn" ? "Drawn: a party holds exactly half the seats"
        : "Status not yet calculated";
      return { title: d.area.name, detail: what };
    }
    case "lead.changed":
      return { title: d.area.name, detail: `${abbrOf(d.toId)} now ahead of ${abbrOf(d.fromId)} on votes, with ${d.reportedPct.toFixed(0)}% of districts captured` };
    case "seats.changed":
      return { title: d.area.name, detail: "PR seats allocated: " + d.deltas.map((x: any) => `${abbrOf(x.partyId)} ${x.seats}`).join(", ") };
    case "progress.milestone":
      return { title: "South Africa", detail: `${d.percent}% of voting districts captured` };
    default:
      return { title: e.type, detail: "" };
  }
}

function topicsForPath(path: string) {
  const topics = ["national", "feed"];
  const m = /^\/municipality\/(\d+)/.exec(path);
  const p = /^\/province\/(\d+)/.exec(path);
  if (m) topics.push(`muni:${m[1]}`);
  if (p) topics.push(`province:${p[1]}`);
  return topics.join(",");
}

async function getJson<T>(url: string): Promise<T> {
  const r = await fetch(url);
  if (!r.ok) throw new Error(`${url}: ${r.status}`);
  return r.json();
}

function mergeEvents(fresh: FeedEvent[], cur: FeedEvent[]) {
  const seen = new Set<number>();
  return [...fresh, ...cur]
    .filter((e) => (seen.has(e.seq) ? false : (seen.add(e.seq), true)))
    .sort((a, b) => b.seq - a.seq)
    .slice(0, KEEP_EVENTS);
}

function Stream({ children }: { children: ReactNode }) {
  const client = useQueryClient();
  const path = usePathname();
  const topics = useMemo(() => topicsForPath(path), [path]);
  const [connection, setConnection] = useState<Connection>("connecting");
  const [status, setStatus] = useState<Status | null>(null);
  const [events, setEvents] = useState<FeedEvent[]>([]);
  const [updatedAt, setUpdatedAt] = useState<number | null>(null);
  const [watchlist, setWatchlist] = useLocalStorage<number[]>("watchlist", []);
  const [notifyStored, setNotifyStored] = useLocalStorage<boolean>("notify", false);
  const [live, setLiveStored] = useLocalStorage<boolean>("live", false);
  const [checkedAt, setCheckedAt] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFailed, setRefreshFailed] = useState(false);
  // Notifications come from the stream, so they only work while it's on.
  const notify = notifyStored && live;

  const lastSeq = useRef(0);
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastRefresh = useRef(0);
  const watch = useRef({ watchlist, notify });
  watch.current = { watchlist, notify };

  const load = useCallback(async () => {
    const [s, evs] = await Promise.all([getJson<Status>("/api/v1/status"), getJson<FeedEvent[]>("/api/v1/feed?limit=30")]);
    setStatus(s);
    setEvents((cur) => mergeEvents(evs, cur));
    setCheckedAt(Date.now());
  }, []);

  useEffect(() => {
    load().catch(() => {});
  }, [load]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    setRefreshFailed(false);
    try {
      await load();
      await client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "/parties" });
    } catch {
      setRefreshFailed(true);
    } finally {
      setRefreshing(false);
    }
  }, [client, load]);

  const setLive = (on: boolean) => {
    lastSeq.current = 0;
    setLiveStored(on);
    if (on) refresh();
    else setCheckedAt(Date.now());
  };

  useEffect(() => {
    if (!live) return;
    setConnection("connecting");
    const scheduleRefresh = () => {
      if (refreshTimer.current) return;
      const wait = Math.max(0, lastRefresh.current + REFRESH_MIN_MS - Date.now()) + Math.random() * REFRESH_JITTER_MS;
      refreshTimer.current = setTimeout(() => {
        refreshTimer.current = null;
        lastRefresh.current = Date.now();
        setUpdatedAt(Date.now());
        client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "/parties" });
      }, wait);
    };

    const url = `/api/v1/stream?topics=${topics}${lastSeq.current ? `&lastEventId=${lastSeq.current}` : ""}`;
    const es = new EventSource(url);
    es.onopen = () => setConnection("live");
    es.onerror = () => setConnection("reconnecting");

    const handle = (type: string) => (msg: MessageEvent) => {
      const e = JSON.parse(msg.data) as FeedEvent;
      if (e.seq <= lastSeq.current) return; // replays can repeat what we've applied
      lastSeq.current = e.seq;
      scheduleRefresh();
      if (type === "area.updated") return;
      setEvents((cur) => [e, ...cur].slice(0, KEEP_EVENTS));
      const id = eventMunicipality(e);
      if (id && watch.current.notify && watch.current.watchlist.includes(id) && document.hidden && "Notification" in window && Notification.permission === "granted") {
        new Notification(e.data.area.name, { body: e.type === "ward.declared" ? `Ward ${e.data.wardNo} seat allocated` : "New result", tag: `lge-${id}` });
      }
    };
    es.addEventListener("area.updated", handle("area.updated"));
    FEED_TYPES.forEach((t) => es.addEventListener(t, handle(t)));
    es.addEventListener("status", (m) => setStatus(JSON.parse((m as MessageEvent).data)));
    es.addEventListener("resync", () => scheduleRefresh());

    return () => {
      es.close();
      if (refreshTimer.current) {
        clearTimeout(refreshTimer.current);
        refreshTimer.current = null;
      }
    };
  }, [topics, client, live]);

  const value: LiveValue = {
    connection: live ? connection : "off",
    status, events, updatedAt, checkedAt, live, setLive, refresh, refreshing, refreshFailed, watchlist, notify,
    toggleWatch: (id) => setWatchlist(watchlist.includes(id) ? watchlist.filter((x) => x !== id) : [...watchlist, id]),
    setNotify: async (on) => {
      if (on && "Notification" in window && Notification.permission !== "granted") {
        const res = await Notification.requestPermission();
        if (res !== "granted") return setNotifyStored(false);
      }
      setNotifyStored(on);
      if (on && !live) setLive(true);
    },
  };
  return <LiveContext.Provider value={value}>{children}</LiveContext.Provider>;
}

export function LiveProvider({ children }: { children: ReactNode }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false, refetchOnReconnect: false } } }));
  return (
    <QueryClientProvider client={client}>
      <Stream>{children}</Stream>
    </QueryClientProvider>
  );
}
