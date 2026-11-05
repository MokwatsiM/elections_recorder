import { ImageResponse } from "next/og";
import { serverGet } from "@/lib/api";
import type { NationalView } from "@/lib/types";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = "Live count of the 2026 South African local elections";
export const dynamic = "force-dynamic";

export default async function Image() {
  const n = await serverGet<NationalView>("/national");
  const captured = n && n.progress.vdsExpected ? (n.progress.vds * 100) / n.progress.vdsExpected : 0;
  return new ImageResponse(
    <div style={{ width: "100%", height: "100%", display: "flex", flexDirection: "column", justifyContent: "space-between", background: "#f3f5f1", color: "#131a17", padding: 64, fontFamily: "sans-serif" }}>
      <div style={{ display: "flex", flexDirection: "column" }}>
        <div style={{ fontSize: 72, fontWeight: 700 }}>2026 Local Elections</div>
        <div style={{ fontSize: 36, marginTop: 12 }}>Live count, from the country to your voting district</div>
      </div>
      <div style={{ display: "flex", flexDirection: "column" }}>
        {n && <div style={{ fontSize: 44, fontWeight: 700 }}>{`${captured.toFixed(0)}% of voting districts captured`}</div>}
        <div style={{ fontSize: 28, marginTop: 12 }}>Unofficial. Figures from the IEC.</div>
      </div>
    </div>,
    size,
  );
}
