"use client";

import "maplibre-gl/dist/maplibre-gl.css";
import type { Map as MLMap, MapMouseEvent } from "maplibre-gl";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { useApi, useParties } from "@/lib/hooks";
import { abbr, hasOwnColour, partyColor, partyVar } from "@/lib/parties";
import { pct, signed } from "@/lib/format";
import type { AreaSummary, Level, PartyInfo } from "@/lib/types";

type MapLevel = Exclude<Level, "national">;
type Metric = "leader" | "share" | "swing";

const SOURCE_LAYER: Record<MapLevel, string> = { province: "provinces", municipality: "municipalities", ward: "wards", vd: "vds" };
const SA_BOUNDS: [number, number, number, number] = [16.3, -35, 33, -22];
const FEATURED = [7, 52, 945, 3, 957, 1511, 4, 1748];

const css = (v: string) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();

function mix(a: string, b: string, t: number) {
  const parse = (h: string) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
  const [x, y] = [parse(a), parse(b)];
  return `rgb(${x.map((v, i) => Math.round(v + (y[i] - v) * t)).join(",")})`;
}

function colourFor(row: AreaSummary, metric: Metric): string | null {
  if (!row.reported) return null;
  if (metric === "leader") return row.leaderId === undefined ? null : partyColor(row.leaderId);
  return null;
}

export function ResultsMap({ level, parent, focus, scope, hrefFor, levels, onLevel, height = "min(62vh, 620px)" }: {
  level: MapLevel;
  parent?: number;
  focus?: { kind: "province" | "municipality" | "ward"; id: number };
  scope?: (r: AreaSummary) => boolean;
  hrefFor: (r: AreaSummary) => string | null;
  levels?: { level: MapLevel; label: string }[];
  onLevel?: (l: MapLevel) => void;
  height?: string;
}) {
  const router = useRouter();
  const parties = useParties();
  const container = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MLMap | null>(null);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [metric, setMetric] = useState<Metric>("leader");
  const [partyId, setPartyId] = useState(7);
  const [hover, setHover] = useState<{ x: number; y: number; row: AreaSummary } | null>(null);
  const [theme, setTheme] = useState(0);

  const needsParty = metric !== "leader";
  const qs = new URLSearchParams();
  if (parent) qs.set("parent", String(parent));
  if (needsParty) qs.set("party", String(partyId));
  const { data } = useApi<AreaSummary[]>(`/map/${level}${qs.size ? `?${qs}` : ""}`);
  const rows = useMemo(() => (scope ? (data ?? []).filter(scope) : (data ?? [])), [data, scope]);
  const byId = useMemo(() => new Map(rows.map((r) => [r.id, r])), [rows]);
  const swingAvailable = level === "province" || level === "municipality";
  const activeMetric: Metric = metric === "swing" && !swingAvailable ? "leader" : metric;

  const live = useRef({ byId, hrefFor, level });
  live.current = { byId, hrefFor, level };

  useEffect(() => {
    const repaint = () => setTheme((t) => t + 1);
    window.addEventListener("themechange", repaint);
    return () => window.removeEventListener("themechange", repaint);
  }, []);

  // Create the map once; live updates only change feature state, never geometry.
  useEffect(() => {
    let cancelled = false;
    let created: MLMap | undefined;
    (async () => {
      const [maplibregl, { Protocol }] = await Promise.all([import("maplibre-gl"), import("pmtiles")]);
      if (cancelled || !container.current) return;
      // Bundlers can't resolve MapLibre's worker, so it's served from public/.
      maplibregl.setWorkerUrl("/maplibre-gl-worker.mjs");
      const protocol = new Protocol();
      maplibregl.addProtocol("pmtiles", protocol.tile);
      const layers = (Object.values(SOURCE_LAYER)).flatMap((l) => [
        { id: `fill-${l}`, type: "fill" as const, source: "sa", "source-layer": l, layout: { visibility: "none" as const },
          paint: { "fill-color": ["coalesce", ["feature-state", "color"], css("--no-result")] as any, "fill-opacity": 1 } },
        { id: `line-${l}`, type: "line" as const, source: "sa", "source-layer": l, layout: { visibility: "none" as const },
          paint: { "line-color": css("--paper"), "line-width": 0.6 } },
      ]);
      const map = new maplibregl.Map({
        container: container.current,
        attributionControl: false,
        cooperativeGestures: window.matchMedia("(pointer: coarse)").matches,
        dragRotate: false,
        bounds: SA_BOUNDS,
        fitBoundsOptions: { padding: 12 },
        style: {
          version: 8,
          sources: { sa: { type: "vector", url: `pmtiles://${location.origin}/tiles/sa.pmtiles`, promoteId: "id" } },
          layers: [
            { id: "bg", type: "background", paint: { "background-color": css("--paper-2") } },
            ...layers,
            { id: "province-edges", type: "line", source: "sa", "source-layer": "provinces", paint: { "line-color": css("--ink"), "line-width": 1 } },
          ],
        },
      });
      created = map;
      map.touchZoomRotate.disableRotation();
      map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
      map.on("error", (e) => { if (/pmtiles|sa\b|source/i.test(String(e.error?.message))) setFailed(true); });
      map.on("load", () => { mapRef.current = map; setReady(true); });

      const rowAt = (e: MapMouseEvent) => {
        const layer = `fill-${SOURCE_LAYER[live.current.level]}`;
        const f = map.queryRenderedFeatures(e.point, { layers: [layer] })[0];
        return f ? live.current.byId.get(Number(f.id)) : undefined;
      };
      map.on("mousemove", (e) => {
        const row = rowAt(e);
        map.getCanvas().style.cursor = row && live.current.hrefFor(row) ? "pointer" : "";
        setHover(row ? { x: e.point.x, y: e.point.y, row } : null);
      });
      map.on("mouseout", () => setHover(null));
      map.on("click", (e) => {
        const row = rowAt(e);
        const href = row && live.current.hrefFor(row);
        if (href) router.push(href);
      });
    })().catch(() => setFailed(true));
    return () => {
      cancelled = true;
      created?.remove();
      mapRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    Object.entries(SOURCE_LAYER).forEach(([lv, l]) => {
      const vis = lv === level ? "visible" : "none";
      map.setLayoutProperty(`fill-${l}`, "visibility", vis);
      map.setLayoutProperty(`line-${l}`, "visibility", vis);
      map.setPaintProperty(`fill-${l}`, "fill-color", ["coalesce", ["feature-state", "color"], css("--no-result")]);
      map.setPaintProperty(`line-${l}`, "line-color", css("--paper"));
    });
    map.setPaintProperty("bg", "background-color", css("--paper-2"));
    map.setPaintProperty("province-edges", "line-color", css("--ink"));
  }, [ready, level, theme]);

  // Fit to the area in view, using bounds generated with the tiles.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    if (!focus) return void map.fitBounds(SA_BOUNDS, { padding: 12, duration: 300 });
    const file = focus.kind === "ward" ? "/bounds-wards.json" : "/bounds.json";
    fetch(file).then((r) => r.json()).then((b) => {
      const box = focus.kind === "ward" ? b[focus.id] : b[focus.kind]?.[focus.id];
      if (box && mapRef.current) mapRef.current.fitBounds(box, { padding: 24, duration: 400 });
    }).catch(() => {});
  }, [ready, focus?.kind, focus?.id]);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    const sourceLayer = SOURCE_LAYER[level];
    map.removeFeatureState({ source: "sa", sourceLayer });
    const paper = css("--no-result");
    const baseHex = /^#/.test(paper) ? paper : "#dfe3dc";
    const pc = partyColor(partyId);
    const [down, mid, up] = [css("--swing-down"), css("--swing-mid"), css("--swing-up")];
    for (const row of rows) {
      let color = colourFor(row, activeMetric);
      if (row.reported && activeMetric === "share") {
        color = row.partyPct === undefined ? null : mix(baseHex, pc, Math.min(row.partyPct / 60, 1));
      }
      if (row.reported && activeMetric === "swing" && row.partySwing !== undefined) {
        const t = Math.min(Math.abs(row.partySwing) / 20, 1);
        color = mix(mid, row.partySwing < 0 ? down : up, t);
      }
      if (color) map.setFeatureState({ source: "sa", sourceLayer, id: row.id }, { color });
    }
  }, [ready, rows, activeMetric, partyId, level, theme]);

  const leaders = useMemo(() => {
    const seen = new Map<number, number>();
    rows.forEach((r) => { if (r.reported && r.leaderId !== undefined) seen.set(r.leaderId, (seen.get(r.leaderId) ?? 0) + 1); });
    return [...seen.entries()].sort((a, b) => b[1] - a[1]);
  }, [rows]);
  const options = useMemo<PartyInfo[]>(() => {
    const list = parties ?? [];
    return [...list.filter((p) => FEATURED.includes(p.id)), ...list.filter((p) => !FEATURED.includes(p.id) && p.id > 0).sort((a, b) => a.abbr.localeCompare(b.abbr))];
  }, [parties]);

  return (
    <div>
      <div className="flex flex-wrap gap-2 items-center mb-2">
        {levels && (
          <div className="flex" role="group" aria-label="Map detail">
            {levels.map((l) => (
              <button key={l.level} className="btn -ml-px first:ml-0" aria-pressed={l.level === level} onClick={() => onLevel?.(l.level)}>{l.label}</button>
            ))}
          </div>
        )}
        <div className="flex" role="group" aria-label="Colour the map by">
          <button className="btn -ml-px first:ml-0" aria-pressed={activeMetric === "leader"} onClick={() => setMetric("leader")}>Leading party</button>
          <button className="btn -ml-px" aria-pressed={activeMetric === "share"} onClick={() => setMetric("share")}>Party share</button>
          {swingAvailable && <button className="btn -ml-px" aria-pressed={activeMetric === "swing"} onClick={() => setMetric("swing")}>Since 2021</button>}
        </div>
        {needsParty && (
          <select className="input !w-auto text-sm" aria-label="Party" value={partyId} onChange={(e) => setPartyId(Number(e.target.value))}>
            {options.map((p) => <option key={p.id} value={p.id}>{p.abbr}</option>)}
          </select>
        )}
      </div>

      <div className="relative border border-rule" style={{ height }}>
        <div ref={container} className="h-full w-full" role="img" aria-label="Map of results. The same figures are in the table on this page." />
        {!ready && !failed && <div className="absolute inset-0 skeleton grid place-items-center label">Loading map&hellip;</div>}
        {failed && <div className="absolute inset-0 bg-cell grid place-items-center label p-4 text-center">The map couldn&rsquo;t load. All of its figures are in the table on this page.</div>}
        {hover && (
          <div className="absolute z-10 pointer-events-none bg-cell border border-rule-strong px-2 py-1 text-sm max-w-64"
            style={{ left: Math.min(hover.x + 12, (container.current?.clientWidth ?? 300) - 220), top: Math.max(hover.y - 50, 4) }}>
            <div className="font-semibold">{hover.row.name}</div>
            {hover.row.reported ? (
              <div className="label">
                {abbr(parties, hover.row.leaderId!)} leads, {pct(hover.row.leaderPct)}
                {activeMetric === "share" && hover.row.partyPct !== undefined && <><br />{abbr(parties, partyId)}: {pct(hover.row.partyPct)}</>}
                {activeMetric === "swing" && <><br />{abbr(parties, partyId)} since 2021: {hover.row.partySwing !== undefined ? `${signed(hover.row.partySwing)} pts` : "not comparable"}</>}
                {hover.row.vdPct !== undefined && <><br />{pct(hover.row.vdPct, 0)} captured</>}
              </div>
            ) : <div className="label">No result yet</div>}
          </div>
        )}
      </div>

      <div className="flex flex-wrap gap-x-3 gap-y-1 mt-2 label items-center" aria-label="Legend">
        {activeMetric === "leader" && leaders.slice(0, 10).map(([id]) => (
          <span key={id} className="inline-flex items-center gap-1"><span className="swatch" style={{ background: partyVar(id) }} />{hasOwnColour(id) ? abbr(parties, id) : abbr(parties, id)}</span>
        ))}
        {activeMetric === "share" && <span className="inline-flex items-center gap-1.5">0%<span className="h-2.5 w-24" style={{ background: `linear-gradient(90deg, var(--no-result), ${partyVar(partyId)})` }} />60%+ for {abbr(parties, partyId)}</span>}
        {activeMetric === "swing" && <span className="inline-flex items-center gap-1.5">&minus;20 pts<span className="h-2.5 w-28" style={{ background: "linear-gradient(90deg, var(--swing-down), var(--swing-mid), var(--swing-up))" }} />+20 pts for {abbr(parties, partyId)}</span>}
        <span className="inline-flex items-center gap-1"><span className="swatch" style={{ background: "var(--no-result)", outline: "1px solid var(--rule)" }} />No result yet</span>
      </div>
    </div>
  );
}
