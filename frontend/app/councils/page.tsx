import type { Metadata } from "next";
import { serverGet } from "@/lib/api";
import type { CouncilRow, NationalView } from "@/lib/types";
import { CouncilsPage } from "./CouncilsPage";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Councils" };

export default async function Page() {
  const [rows, national] = await Promise.all([serverGet<CouncilRow[]>("/councils"), serverGet<NationalView>("/national")]);
  return <CouncilsPage initial={rows} initialNational={national} />;
}
