"use client";

import { AreaHeader } from "@/components/AreaHeader";
import { Breadcrumb } from "@/components/Breadcrumb";
import { Section } from "@/components/Form";
import { NotFoundNote } from "@/components/NotFoundNote";
import { PartyTable } from "@/components/PartyTable";
import { useApi } from "@/lib/hooks";
import type { VDView } from "@/lib/types";

export function VDPage({ municId, vd, initial }: { municId: number; vd: number; initial: VDView | null }) {
  const { data: v, isError } = useApi<VDView>(`/municipalities/${municId}/vds/${vd}`, initial);
  if (!v) return isError ? <NotFoundNote what="this voting district" /> : <div className="skeleton h-96" />;
  return (
    <div className="max-w-3xl">
      <Breadcrumb items={[
        { href: "/", label: "South Africa" }, { href: `/province/${v.municipality.parentId}`, label: v.provinceName },
        { href: `/municipality/${municId}`, label: v.municipality.name }, { href: `/municipality/${municId}/ward/${v.wardId}`, label: `Ward ${v.wardNo}` },
        { label: v.name },
      ]} />
      <AreaHeader title={v.name} sub={`Voting district ${v.id}`} reportDate={v.reported ? v.reportDate : undefined} />
      <Section title="Party results" aside="Ward and PR ballots">
        {v.reported ? <PartyTable parties={v.parties} seats={false} /> : <p className="cell border border-rule text-sm py-4">No result has been captured for this voting district yet.</p>}
      </Section>
    </div>
  );
}
