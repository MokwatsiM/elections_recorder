"use client";

import { CircleDashed, Scale, Split, Users } from "lucide-react";
import { useParties, useFlash } from "@/lib/hooks";
import { abbr, partyVar } from "@/lib/parties";
import { num } from "@/lib/format";
import type { CouncilStatus, Progress } from "@/lib/types";
import type { ReactNode } from "react";

/** A number in character boxes. Spare boxes on the left stay empty until reached. */
export function Comb({ value, of, label }: { value: number; of: number; label: string }) {
  const width = Math.max(String(of).length, String(value).length);
  const digits = String(value).padStart(width, " ").split("");
  const flash = useFlash(value);
  return (
    <span className={`comb ${flash ? "mark-changed" : ""}`} role="img" aria-label={`${label}: ${value.toLocaleString("en-ZA")} of ${of.toLocaleString("en-ZA")}`}>
      {digits.map((d, i) => (
        <span key={i} className={(width - i) % 3 === 0 && i > 0 ? "gap" : ""}>{d.trim()}</span>
      ))}
    </span>
  );
}

export function Meter({ value, cells = 40 }: { value: number; cells?: number }) {
  const on = Math.round((value / 100) * cells);
  return (
    <div className="meter" aria-hidden>
      {Array.from({ length: cells }, (_, i) => <i key={i} className={i < on || (i === 0 && value > 0) ? "on" : ""} />)}
    </div>
  );
}

export function Captured({ progress, children }: { progress: Progress; children?: ReactNode }) {
  const pct = progress.vdsExpected ? (progress.vds * 100) / progress.vdsExpected : 0;
  return (
    <div className="cell">
      <div className="flex items-baseline justify-between gap-2 label">
        <span>Voting districts captured</span>
        <span className="font-semibold text-ink">{pct.toFixed(1)}%</span>
      </div>
      <div className="flex items-center gap-2 mt-1.5 text-lg flex-wrap">
        <Comb value={progress.vds} of={progress.vdsExpected} label="Voting districts captured" />
        <span className="label">of {num(progress.vdsExpected)}</span>
      </div>
      <div className="mt-2"><Meter value={pct} /></div>
      {children}
    </div>
  );
}

export function Field({ label, value, hint }: { label: string; value: ReactNode; hint?: string }) {
  const flash = useFlash(typeof value === "string" || typeof value === "number" ? value : null);
  return (
    <div className={`cell ${flash ? "mark-changed" : ""}`}>
      <div className="label">{label}</div>
      <div className="text-lg font-semibold mt-0.5">{value}</div>
      {hint && <div className="label mt-0.5">{hint}</div>}
    </div>
  );
}

const STATUS: Record<CouncilStatus, { text: string; Icon: typeof Users }> = {
  majority: { text: "Majority", Icon: Users },
  hung: { text: "Hung", Icon: Split },
  drawn: { text: "Drawn", Icon: Scale },
  pending: { text: "Not yet calculated", Icon: CircleDashed },
};

export function StatusTag({ status }: { status: CouncilStatus }) {
  const { text, Icon } = STATUS[status];
  return <span className={`tag ${status}`}><Icon size={11} aria-hidden />{text}</span>;
}

export function PartyMark({ id, name }: { id: number; name?: string }) {
  const parties = useParties();
  return (
    <span className="inline-flex items-center gap-1.5 min-w-0">
      <span className="swatch" style={{ background: partyVar(id) }} aria-hidden />
      <span className="truncate">{abbr(parties, id, name)}</span>
    </span>
  );
}

export function Section({ title, aside, children, id }: { title: string; aside?: ReactNode; children: ReactNode; id?: string }) {
  return (
    <section className="section" id={id}>
      <div className="section-head"><h2>{title}</h2>{aside && <div className="label text-right">{aside}</div>}</div>
      {children}
    </section>
  );
}

export function EmptyNote({ children }: { children: ReactNode }) {
  return <p className="cell border border-rule label !text-sm py-4">{children}</p>;
}
