# Product

## Platform

Responsive web. Phones are the main device during the count; desktops second, including people leaving it open on a second screen.

## Stack

Next.js 16, React 19, Tailwind 4, MapLibre GL. Go backend streams changes over SSE.

## Users

South Africans following the 2026 Local Government Elections count, plus journalists and party workers who check one municipality or ward repeatedly. Most arrive with one question: who's ahead where I live, and who will run my council? Many are on mobile data. (Assumption: inferred from the brief, not from research.)

## Product Purpose

Show the IEC's published results faster to read, easier to drill into and live when the viewer switches it on, from country down to voting district.

## Positioning

The most legible way to follow the count. It is not official: the IEC is the authority, and this product shows the IEC's figures faithfully.

## Operating Context

Long sessions over several days, peaking in evenings. Figures update in batches about every six minutes. Viewers are often on phones, in varied light, sharing what they see.

## Capabilities and Constraints

- Results are incomplete until the count ends, so every figure needs its share captured and its as-of time.
- The IEC files have no turnout figure. Cast divided by registered exceeds 100% because voters cast several ballots, so never present it as turnout.
- A voting district with no votes has not necessarily been captured. Never show it as 0%.
- Votes count ballots (ward plus PR), not voters.
- "Largest party" and "controls the council" are different things.

## Brand Commitments

None yet. No IEC logo or branding, and no party logos until rights are clear.

## Evidence on Hand

Live IEC data, recorded since 5 November 2026. No testimonials or usage data.

## Product Principles

1. **Never overstate a result.** No projections or winner calls of our own, and a partial count always looks partial.
2. **Politically even-handed.** No party gets favourable placement, tone or colour emphasis. Ranking comes from the numbers.
3. **Answer first.** The viewer's area and who leads it are visible without hunting.
4. **Change is the story.** Make it obvious what just changed, without constant distraction.
5. **Light on data.** Mobile data costs viewers real money.

## Accessibility & Inclusion

WCAG 2.2 AA. Party identity is never carried by colour alone: every coloured mark has a name or abbreviation next to it, and every map has an equivalent table. Respect reduced motion. English first; numbers formatted for South Africa.
