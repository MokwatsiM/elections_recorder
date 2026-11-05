import Link from "next/link";

export function Breadcrumb({ items }: { items: { href?: string; label: string }[] }) {
  return (
    <nav aria-label="Breadcrumb" className="label mb-1 flex flex-wrap gap-x-1.5">
      {items.map((c, i) => (
        <span key={i} className="flex gap-1.5">
          {i > 0 && <span aria-hidden>/</span>}
          {c.href ? <Link href={c.href} className="underline">{c.label}</Link> : <span aria-current="page">{c.label}</span>}
        </span>
      ))}
    </nav>
  );
}
