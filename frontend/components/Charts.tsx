"use client";

import { useRef, useState } from "react";
import { clock, pct, stamp } from "@/lib/format";
import { useParties } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import type { HistoryView, Party, TimelinePoint } from "@/lib/types";

interface Series { id: number; label: string; points: (number | null)[] }

const W = 720, H = 260, M = { l: 34, r: 74, t: 10, b: 24 };
const niceMax = (v: number) => Math.min(100, Math.ceil(v / 10) * 10 || 10);

/** Share over an ordered x axis. One y-axis, direct labels, and a shared crosshair. */
function LineChart({ series, xLabels, xTicks, hoverLabel, dashLast, label }: {
  series: Series[]; xLabels: string[]; xTicks: number[]; hoverLabel: (i: number) => string; dashLast?: boolean; label: string;
}) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const n = xLabels.length;
  const max = niceMax(Math.max(...series.flatMap((s) => s.points.map((p) => p ?? 0)), 1));
  const x = (i: number) => M.l + (n <= 1 ? 0 : (i / (n - 1)) * (W - M.l - M.r));
  const y = (v: number) => M.t + (1 - v / max) * (H - M.t - M.b);

  const onMove = (e: React.PointerEvent) => {
    const box = ref.current?.getBoundingClientRect();
    if (!box || n === 0) return;
    const sx = ((e.clientX - box.left) / box.width) * W;
    setHover(Math.max(0, Math.min(n - 1, Math.round(((sx - M.l) / (W - M.l - M.r)) * (n - 1)))));
  };

  const path = (pts: (number | null)[], from = 0, to = pts.length) => {
    let d = "", pen = false;
    for (let i = from; i < to; i++) {
      const v = pts[i];
      if (v === null) { pen = false; continue; }
      d += `${pen ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`;
      pen = true;
    }
    return d;
  };

  // Keep end labels from colliding by pushing them apart.
  const ends = series
    .map((s) => { const last = [...s.points].reverse().find((p) => p !== null); return last == null ? null : { s, v: last, y: y(last) }; })
    .filter((e): e is NonNullable<typeof e> => !!e)
    .sort((a, b) => a.y - b.y);
  for (let i = 1; i < ends.length; i++) if (ends[i].y - ends[i - 1].y < 13) ends[i].y = ends[i - 1].y + 13;

  const ticks = [0, max / 2, max];
  return (
    <div className="relative">
      <svg ref={ref} viewBox={`0 0 ${W} ${H}`} className="w-full h-auto touch-pan-y" role="img" aria-label={label}
        onPointerMove={onMove} onPointerLeave={() => setHover(null)}>
        {ticks.map((t) => (
          <g key={t}>
            <line x1={M.l} x2={W - M.r} y1={y(t)} y2={y(t)} stroke="var(--rule)" />
            <text x={M.l - 6} y={y(t) + 4} textAnchor="end" fontSize="11" fill="var(--ink-3)">{t}%</text>
          </g>
        ))}
        {xTicks.map((i) => <text key={i} x={x(i)} y={H - 6} textAnchor="middle" fontSize="11" fill="var(--ink-3)">{xLabels[i]}</text>)}
        {series.map((s) => (
          <g key={s.id}>
            <path d={path(s.points, 0, dashLast ? n - 1 : n)} fill="none" stroke={partyVar(s.id)} strokeWidth="2" strokeLinejoin="round" />
            {dashLast && <path d={path(s.points, n - 2, n)} fill="none" stroke={partyVar(s.id)} strokeWidth="2" strokeDasharray="3 3" />}
          </g>
        ))}
        {ends.map((e) => (
          <text key={e.s.id} x={W - M.r + 6} y={e.y + 4} fontSize="11" fontWeight="600" fill="var(--ink)">{e.s.label}</text>
        ))}
        {hover !== null && (
          <g>
            <line x1={x(hover)} x2={x(hover)} y1={M.t} y2={H - M.b} stroke="var(--ink)" />
            {series.map((s) => s.points[hover] != null && (
              <circle key={s.id} cx={x(hover)} cy={y(s.points[hover]!)} r="4" fill={partyVar(s.id)} stroke="var(--cell)" strokeWidth="2" />
            ))}
          </g>
        )}
      </svg>
      {hover !== null && (
        <div className="absolute top-1 z-10 bg-cell border border-rule-strong px-2 py-1 text-xs pointer-events-none"
          style={{ left: `${(x(hover) / W) * 100}%`, transform: hover > n / 2 ? "translateX(calc(-100% - 8px))" : "translateX(8px)" }}>
          <div className="font-semibold mb-0.5 whitespace-nowrap">{hoverLabel(hover)}</div>
          {[...series].filter((s) => s.points[hover] != null).sort((a, b) => b.points[hover]! - a.points[hover]!).map((s) => (
            <div key={s.id} className="flex justify-between gap-3">
              <span className="flex items-center gap-1"><span className="swatch" style={{ background: partyVar(s.id) }} />{s.label}</span>
              <span>{pct(s.points[hover]!)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** The share of each party as districts came in. */
export function RaceChart({ points }: { points: TimelinePoint[] }) {
  const parties = useParties();
  if (points.length < 2) return <p className="label py-3">This chart fills in as more results arrive.</p>;
  const latest = points[points.length - 1].parties;
  const top = [...latest].filter((p) => p.id !== -2).sort((a, b) => b.pct - a.pct).slice(0, 5);
  const series: Series[] = top.map((p: Party) => ({
    id: p.id, label: abbr(parties, p.id),
    points: points.map((pt) => pt.parties.find((x) => x.id === p.id)?.pct ?? null),
  }));
  const n = points.length;
  return (
    <LineChart
      label={`Share of the vote over time for ${series.map((s) => s.label).join(", ")}`}
      series={series}
      xLabels={points.map((p) => clock(p.at))}
      xTicks={[...new Set([0, Math.round((n - 1) / 3), Math.round((2 * (n - 1)) / 3), n - 1])]}
      hoverLabel={(i) => `${stamp(points[i].at)}, ${((points[i].progress.vds * 100) / (points[i].progress.vdsExpected || 1)).toFixed(0)}% captured`}
    />
  );
}

/** Each party's share at past elections, with today's partial count dashed. */
export function HistoryChart({ history }: { history: HistoryView }) {
  const years = [2000, 2006, 2011, 2016, 2021];
  const top = history.parties.filter((p) => p.partyId !== -2 && p.partyId !== 0).slice(0, 6);
  const series: Series[] = top.map((s) => ({
    id: s.partyId, label: s.abbr,
    points: [...years.map((y) => s.years.find((v) => v.year === y)?.pct ?? null), s.current],
  }));
  const labels = [...years.map(String), "2026 so far"];
  return (
    <>
      <LineChart label={`Support by election for ${history.name}`} series={series} xLabels={labels} xTicks={labels.map((_, i) => i)} hoverLabel={(i) => labels[i]} dashLast />
      <details className="mt-2">
        <summary className="label cursor-pointer">Show as a table</summary>
        <div className="overflow-x-auto mt-1">
          <table className="ledger">
            <thead><tr><th>Party</th>{labels.map((l) => <th key={l} className="n">{l}</th>)}</tr></thead>
            <tbody>
              {series.map((s) => (
                <tr key={s.id}><td className="font-medium">{s.label}</td>{s.points.map((p, i) => <td key={i} className="n">{p === null ? "–" : pct(p)}</td>)}</tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </>
  );
}
