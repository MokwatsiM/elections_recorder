import type { Metadata, Viewport } from "next";
import { Archivo } from "next/font/google";
import type { ReactNode } from "react";
import { Shell } from "@/components/Shell";
import { LiveProvider } from "@/lib/live";
import "./globals.css";

const archivo = Archivo({ subsets: ["latin"], axes: ["wdth"], variable: "--font-archivo", display: "swap" });

export const metadata: Metadata = {
  metadataBase: new URL(process.env.SITE_URL ?? "http://localhost:3000"),
  title: { default: "2026 Local Elections: live count", template: "%s | 2026 Local Elections" },
  description: "Unofficial live view of the IEC's 2026 South African local election results, from the country down to your voting district.",
};

export const viewport: Viewport = { width: "device-width", initialScale: 1 };

// Runs before paint so a dark-mode viewer never sees a flash of light.
const themeScript = `try{var t=localStorage.getItem("theme");if(t)t=JSON.parse(t);if(!t||t==="auto")t=matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.dataset.theme=t}catch(e){}`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en-ZA" className={archivo.variable} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <LiveProvider>
          <Shell>{children}</Shell>
        </LiveProvider>
      </body>
    </html>
  );
}
