import type { Metadata } from "next";
import { serverGet } from "@/lib/api";
import type { WardView } from "@/lib/types";
import { WardPage } from "./WardPage";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ id: string; wardId: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id, wardId } = await params;
  const w = await serverGet<WardView>(`/municipalities/${id}/wards/${wardId}`);
  return { title: w ? `Ward ${w.no}, ${w.municipality.name}` : "Ward" };
}

export default async function Page({ params }: Props) {
  const { id, wardId } = await params;
  return <WardPage municId={Number(id)} wardId={Number(wardId)} initial={await serverGet<WardView>(`/municipalities/${id}/wards/${wardId}`)} />;
}
