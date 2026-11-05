"use client";

import { AlertTriangle, RefreshCw } from "lucide-react";
import { useEffect, useState } from "react";
import { clock } from "@/lib/format";
import { useLive } from "@/lib/live";

function Message() {
  const { connection, status, updatedAt, checkedAt, refreshing, refreshFailed } = useLive();
  const lastChange = status?.lastChange && !status.lastChange.startsWith("0001") ? status.lastChange : null;
  if (status?.replay) return <span>Replay of a recorded session, not live</span>;
  if (refreshFailed) return <span>Couldn&rsquo;t refresh. Try again, or reload the page.</span>;
  if (status?.stale) {
    return (
      <span className="inline-flex items-center gap-1 px-1.5" style={{ background: "var(--warn-bg)", color: "var(--warn-ink)" }}>
        <AlertTriangle size={12} aria-hidden />
        Can&rsquo;t reach the IEC right now. Showing figures from {lastChange ? clock(lastChange) : "earlier"}.
      </span>
    );
  }
  if (connection === "off") {
    if (refreshing) return <span>Refreshing&hellip;</span>;
    return <span>{checkedAt ? `Updated ${clock(new Date(checkedAt).toISOString())}` : ""}</span>;
  }
  if (connection === "reconnecting") return <span>Reconnecting to live updates&hellip;</span>;
  const shown = updatedAt ? new Date(updatedAt).toISOString() : lastChange;
  return (
    <span>
      {connection === "live" ? "Live" : "Connecting"}
      {shown ? ` · last change ${clock(shown)}` : ""}
    </span>
  );
}

export function StatusLine() {
  const { live, setLive, refresh, refreshing } = useLive();
  const [, tick] = useState(0);
  useEffect(() => {
    const t = setInterval(() => tick((n) => n + 1), 30_000);
    return () => clearInterval(t);
  }, []);

  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      <span role="status"><Message /></span>
      {!live && (
        <button className="btn" onClick={refresh} disabled={refreshing}>
          <RefreshCw size={12} aria-hidden className={refreshing ? "motion-safe:animate-spin" : undefined} />
          Refresh
        </button>
      )}
      <button className="btn" aria-pressed={live} onClick={() => setLive(!live)}>
        Live updates: {live ? "on" : "off"}
      </button>
    </span>
  );
}
