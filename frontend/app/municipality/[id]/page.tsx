import type { Metadata } from "next";
import { serverGet } from "@/lib/api";
import type { MunicipalityView } from "@/lib/types";
import { MunicipalityPage } from "./MunicipalityPage";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const m = await serverGet<MunicipalityView>(`/municipalities/${id}`);
  return { title: m?.name ?? "Municipality", description: m ? `Live count for ${m.name}: wards, voting districts and council seats.` : undefined };
}

export default async function Page({ params }: Props) {
  const { id } = await params;
  return <MunicipalityPage id={Number(id)} initial={await serverGet<MunicipalityView>(`/municipalities/${id}`)} />;
}
