"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { clientGet, NotFoundError } from "./api";
import type { PartyInfo } from "./types";

export function useApi<T>(path: string | null, initial?: T | null) {
  return useQuery<T>({
    queryKey: [path],
    queryFn: () => clientGet<T>(path!),
    enabled: path !== null,
    initialData: initial ?? undefined,
    initialDataUpdatedAt: initial ? Date.now() : undefined,
    staleTime: 5_000,
    retry: (count, err) => !(err instanceof NotFoundError) && count < 3,
    placeholderData: (prev) => prev,
  });
}

export function useParties() {
  const q = useQuery<PartyInfo[]>({
    queryKey: ["/parties"],
    queryFn: () => clientGet<PartyInfo[]>("/parties"),
    staleTime: 10 * 60_000,
  });
  return q.data;
}

/** Returns true for a moment after value changes, to mark what just updated. */
export function useFlash(value: unknown, ms = 2400) {
  const prev = useRef(value);
  const [on, setOn] = useState(false);
  useEffect(() => {
    if (prev.current === value) return;
    prev.current = value;
    setOn(true);
    const t = setTimeout(() => setOn(false), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return on;
}

export function useLocalStorage<T>(key: string, initial: T) {
  const [value, setValue] = useState<T>(initial);
  useEffect(() => {
    try {
      const raw = localStorage.getItem(key);
      if (raw) setValue(JSON.parse(raw));
    } catch {}
  }, [key]);
  const update = (next: T) => {
    setValue(next);
    try {
      localStorage.setItem(key, JSON.stringify(next));
    } catch {}
  };
  return [value, update] as const;
}
