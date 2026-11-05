"use client";

import { stamp } from "@/lib/format";
import type { ReactNode } from "react";

export function AreaHeader({ title, sub, reportDate, action }: { title: string; sub?: ReactNode; reportDate?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 mb-3">
      <div className="min-w-0">
        <h1 className="display text-3xl md:text-4xl text-balance">{title}</h1>
        <p className="text-ink-2 text-sm mt-1">
          {sub}{sub && reportDate ? " · " : ""}
          {reportDate && <>IEC figures as of {stamp(reportDate)}</>}
        </p>
      </div>
      {action}
    </div>
  );
}
