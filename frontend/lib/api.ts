// Server components call the backend directly; browsers go through the rewrite.
const SERVER_BASE = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function serverGet<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`${SERVER_BASE}/api/v1${path}`, { next: { revalidate: 10 } });
    return res.ok ? ((await res.json()) as T) : null;
  } catch {
    return null; // the client retries and shows its own loading and error states
  }
}

export class NotFoundError extends Error {}

export async function clientGet<T>(path: string): Promise<T> {
  const res = await fetch(`/api/v1${path}`);
  if (res.status === 404) throw new NotFoundError(path);
  if (!res.ok) throw new Error(`Request failed (${res.status})`);
  return res.json() as Promise<T>;
}
