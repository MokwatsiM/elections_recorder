# Design

The visual world is the **voting station results slip**: the pre-printed form pinned to a station door, where every result begins.

## Rules

- **The interface has no hue.** Paper, ink and rules only, so nothing in the chrome can lean towards a party. Colour belongs to parties.
- **Highlighter means "just changed".** A yellow mark fades over 2.4s. It is the only animated emphasis, and reduced motion turns it off.
- **Ruled cells, not cards.** Sections open with a 2px rule and a name. Cells share 1px borders with no gaps, corners or shadows.
- **Character boxes for the headline count.** Empty boxes mean "not reached yet", so a partial count looks partial.

## Tokens

Defined in `frontend/app/globals.css`, for light and dark.

| Role | Token |
|---|---|
| Surfaces | `--paper`, `--paper-2`, `--cell` |
| Text | `--ink`, `--ink-2`, `--ink-3` |
| Lines | `--rule`, `--rule-strong` |
| Just changed | `--highlight` |
| Warnings only | `--warn-bg`, `--warn-ink` |
| Map | `--no-result`, `--swing-down`, `--swing-mid`, `--swing-up` |

## Type

Archivo throughout, tabular numerals everywhere. Headings use the narrower width (`font-stretch: 88%`). Sentence case, and no eyebrow labels.

## Party colour

- Eight fixed slots in `frontend/lib/parties.ts`, assigned by party ID and never by rank. Validated for colour-blind separation in light and dark.
- Other parties are neutral grey and always named in text. A ninth hue would not stay distinguishable.
- The IEC's own colours aren't used: many are the same default blue, and several wouldn't be recognised as the party's.
- Colour never carries identity alone. Every mark has a name beside it and every map has a table.

## Language

- Say "ballots", not "votes", for totals. Say "leading on votes", "largest party" or "holds a majority", never "won", unless the IEC has allocated the seat.
- Council status comes only from IEC flags.
- Never show turnout: the IEC files don't contain it.
