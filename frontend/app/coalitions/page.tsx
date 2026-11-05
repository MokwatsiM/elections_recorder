import type { Metadata } from "next";
import { Suspense } from "react";
import { CoalitionsPage } from "./CoalitionsPage";

export const metadata: Metadata = { title: "Coalition calculator" };
export default function Page() { return <Suspense><CoalitionsPage /></Suspense>; }
