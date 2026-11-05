"use client";

import { useRouter } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { useApi } from "@/lib/hooks";
import type { SearchHit } from "@/lib/types";

export function hitHref(h: SearchHit) {
  switch (h.level) {
    case "province": return `/province/${h.id}`;
    case "municipality": return `/municipality/${h.id}`;
    case "ward": return `/municipality/${h.municId}/ward/${h.wardId}`;
    case "vd": return `/municipality/${h.municId}/ward/${h.wardId}/vd/${h.id}`;
    default: return "/";
  }
}

const LEVEL_LABEL: Record<string, string> = { province: "Province", municipality: "Municipality", ward: "Ward", vd: "Station" };

export function Search() {
  const router = useRouter();
  const listId = useId();
  const [text, setText] = useState("");
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setQuery(text.trim()), 200);
    return () => clearTimeout(t);
  }, [text]);
  useEffect(() => {
    const away = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, []);

  const q = useApi<SearchHit[]>(query.length >= 2 ? `/search?q=${encodeURIComponent(query)}` : null);
  const hits = query.length >= 2 ? (q.data ?? []) : [];

  const go = (h: SearchHit) => {
    setOpen(false);
    setText("");
    router.push(hitHref(h));
  };

  return (
    <div ref={box} className="relative grow">
      <input
        className="input text-sm" type="search" role="combobox" aria-expanded={open && hits.length > 0} aria-controls={listId}
        aria-label="Search for a place, ward or voting station" placeholder="Place, ward or voting station"
        value={text}
        onChange={(e) => { setText(e.target.value); setOpen(true); setActive(0); }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => Math.min(a + 1, hits.length - 1)); }
          if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
          if (e.key === "Enter" && hits[active]) go(hits[active]);
          if (e.key === "Escape") setOpen(false);
        }}
      />
      {open && query.length >= 2 && (
        <ul id={listId} role="listbox" className="absolute z-30 left-0 right-0 mt-px bg-cell border border-rule-strong max-h-96 overflow-auto">
          {hits.map((h, i) => (
            <li key={`${h.level}-${h.id}`} role="option" aria-selected={i === active}>
              <button onMouseEnter={() => setActive(i)} onClick={() => go(h)}
                className={`w-full text-left px-2 py-1.5 flex justify-between gap-3 cursor-pointer ${i === active ? "bg-paper-2" : ""}`}>
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{h.name}</span>
                  {h.context && <span className="block truncate label">{h.context}</span>}
                </span>
                <span className="label shrink-0">{LEVEL_LABEL[h.level]}</span>
              </button>
            </li>
          ))}
          {hits.length === 0 && !q.isFetching && <li className="px-2 py-2 label">No match for &ldquo;{query}&rdquo;. Try a municipality, a ward number or a voting station name.</li>}
        </ul>
      )}
    </div>
  );
}
