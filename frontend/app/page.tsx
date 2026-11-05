import { serverGet } from "@/lib/api";
import type { NationalView, CouncilRow } from "@/lib/types";
import { NationalPage } from "./NationalPage";

export const dynamic = "force-dynamic";

export default async function Page() {
  const [national, metros] = await Promise.all([
    serverGet<NationalView>("/national"),
    serverGet<CouncilRow[]>("/councils?metro=true"),
  ]);
  return <NationalPage initial={national} initialMetros={metros} />;
}
