import type { Metadata } from "next";
import { serverGet } from "@/lib/api";
import type { VDView } from "@/lib/types";
import { VDPage } from "./VDPage";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ id: string; vd: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id, vd } = await params;
  const v = await serverGet<VDView>(`/municipalities/${id}/vds/${vd}`);
  return { title: v?.name ?? "Voting district" };
}

export default async function Page({ params }: Props) {
  const { id, vd } = await params;
  return <VDPage municId={Number(id)} vd={Number(vd)} initial={await serverGet<VDView>(`/municipalities/${id}/vds/${vd}`)} />;
}
