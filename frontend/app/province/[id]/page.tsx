import type { Metadata } from "next";
import { serverGet } from "@/lib/api";
import type { ProvinceView } from "@/lib/types";
import { ProvincePage } from "./ProvincePage";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const p = await serverGet<ProvinceView>(`/provinces/${id}`);
  return { title: p?.name ?? "Province" };
}

export default async function Page({ params }: Props) {
  const { id } = await params;
  return <ProvincePage id={Number(id)} initial={await serverGet<ProvinceView>(`/provinces/${id}`)} />;
}
