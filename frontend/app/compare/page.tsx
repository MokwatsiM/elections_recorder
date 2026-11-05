import type { Metadata } from "next";
import { Suspense } from "react";
import { ComparePage } from "./ComparePage";

export const metadata: Metadata = { title: "Compare with past elections" };
export default function Page() { return <Suspense><ComparePage /></Suspense>; }
