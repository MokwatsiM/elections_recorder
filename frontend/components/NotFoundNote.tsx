import Link from "next/link";

export function NotFoundNote({ what }: { what: string }) {
  return (
    <div className="cell border border-rule py-8 text-center">
      <p className="font-semibold">We can&rsquo;t find results for {what}.</p>
      <p className="label mt-1">It may not have reported yet, or the link may be wrong. <Link className="underline" href="/">Back to South Africa</Link></p>
    </div>
  );
}
