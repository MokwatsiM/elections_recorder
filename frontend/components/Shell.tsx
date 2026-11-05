"use client";

import { Moon, Sun } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { Search } from "./Search";
import { StatusLine } from "./StatusLine";

const NAV = [
  { href: "/", label: "Count" },
  { href: "/councils", label: "Councils" },
  { href: "/feed", label: "Feed" },
  { href: "/compare", label: "Compare" },
  { href: "/coalitions", label: "Coalitions" },
];

function ThemeToggle() {
  const [theme, setTheme] = useState<string | null>(null);
  useEffect(() => setTheme(document.documentElement.dataset.theme ?? "light"), []);
  const next = theme === "dark" ? "light" : "dark";
  return (
    <button
      className="btn"
      aria-label={`Switch to ${next} mode`}
      onClick={() => {
        document.documentElement.dataset.theme = next;
        try { localStorage.setItem("theme", JSON.stringify(next)); } catch {}
        setTheme(next);
        window.dispatchEvent(new Event("themechange"));
      }}
    >
      {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
    </button>
  );
}

export function Shell({ children }: { children: ReactNode }) {
  const path = usePathname();
  const active = (href: string) => (href === "/" ? !NAV.slice(1).some((n) => path.startsWith(n.href)) : path.startsWith(href));
  return (
    <div className="mx-auto max-w-[1440px] px-4 lg:px-6">
      <p className="label py-1.5 flex flex-wrap gap-x-2 justify-between border-b border-rule">
        <span>
          <strong className="font-semibold text-ink">Unofficial.</strong> Figures come from the Electoral Commission of South Africa (IEC).{" "}
          <a className="underline" href="https://results.elections.org.za/dashboards/lge/LGEResultsDashboard.aspx" rel="noopener noreferrer">Official results</a>
        </span>
        <StatusLine />
      </p>
      <header className="flex flex-wrap items-center gap-x-5 gap-y-2 py-3">
        <Link href="/" className="display text-xl no-underline">2026 Local Elections</Link>
        <nav aria-label="Main" className="flex gap-1 order-3 lg:order-none w-full lg:w-auto overflow-x-auto -mx-1 px-1 [scrollbar-width:none]">
          {NAV.map((n) => (
            <Link key={n.href} href={n.href} aria-current={active(n.href) ? "page" : undefined}
              className={`px-2 py-1 text-sm whitespace-nowrap no-underline border-b-2 ${active(n.href) ? "border-ink font-semibold" : "border-transparent text-ink-2 hover:text-ink"}`}>
              {n.label}
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-2 grow lg:grow-0 lg:w-80">
          <Search />
          <ThemeToggle />
        </div>
      </header>
      <main className="pb-16">{children}</main>
      <footer className="label border-t border-rule py-4 max-w-prose">
        This site is not run by or affiliated with the IEC, and it makes no projections or calls of its own. Results stay provisional until the IEC declares them. Vote totals count ballots, not voters: each voter casts more than one ballot.
      </footer>
    </div>
  );
}
