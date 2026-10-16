# Interaction layout details

Layout contracts for capture forms, contextual guidance, and task rows.
User-facing operations are documented in [the guide](../guide/README.md).

## Expanded form viewport

Every field on every tab is reachable at any supported height. When a tab holds
more than the terminal can show — a long checklist, a dozen attachments, a
comments list — the form scrolls instead of cutting: `Tab` and `Shift+Tab` move
the focus as usual and the window follows it, and the last row reads `3 above · 1
below` so what is off screen is named rather than lost. A tab that already fits
shows no such row. A refused save keeps its message on screen and moves the focus
to the field it names, so the two stay readable together, and a taller terminal
simply gives the rows back.

## Expanded form width

Every field is reachable at any supported width too. The expanded composer opens
wherever the rest of `tt` opens — down to the same 60-column minimum every other
surface accepts — rather than replacing itself with a "terminal too small"
notice on anything under 68 columns. A narrow terminal shortens what a row can
spare and nothing else: the grey key hints beside a focused row go before its
value does, and a value too long for the row is shortened there and read whole by
pressing `Enter` on it, which opens it in its own editor. The nested pickers —
priority, state, flags, due date, the check-in editor, and the checklist,
subtask, attachment, and comment editors — take the whole screen while they are
open, so at every width they keep a complete frame and a complete list; `Esc`
hands the form back exactly as it was.

## Guidance ownership

Only one set of shortcuts is on screen at a time, and it always belongs to
whatever your next key press reaches. Open a picker, an action sheet, a lens
list, the check-in editor, or the conflict browser over the outline and the
outline's own row of shortcuts goes quiet: it is describing keys that picker has
taken. The same is true of a confirmation, and of a picker opened inside the
expanded capture form. The row does not collapse
while it is quiet — the space stays reserved, so nothing on screen jumps when
the picker opens — and closing the picker brings the outline's shortcuts back
exactly as they were.

## Chord guidance

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, and expanded detail may use
two, and a pending chord may use three, which is what lets `w` name all ten of
its destinations, Done included, at the narrowest terminal. The `f` and `g`
rows are divided the way `w` is, by a heading riding the first entry of each
run: `f` reads `Field:` over the values a task stores and `Derived:` over the
facts worked out from it, with `x clear all` and `% projects` closing the row
outside both, and `g` reads `Mode:`, `Task:`, and `Sync:` behind `g top`. A row
too narrow for all of it never draws a heading without its first entry. At the
narrowest terminal `f` keeps `x clear all` and gives up `a completion age` and
`% projects`, and `g` over a task row gives up the `Sync:` pair whole; every
shed key still works and `?` help still lists it. Required commit and cancellation controls survive before
optional hints, and they survive whole at the narrowest terminal `tt` runs in,
so the way out of a state is never half a sentence. Transient pickers and action
sheets own their local guidance while leaving the host title and workspace pulse
visible.

## Fold-column alignment

Wherever one task in the list can be folded — it shows a child, or it has a
checklist — every task row keeps the two columns the `▾`/`▸` glyph takes, blank
on a row with nothing to fold, so state glyphs and titles line up down the
list. A list in which nothing folds keeps those columns for its titles, and so
does a terminal under 60 columns. The reference count stands two columns clear
of the title, the gap the deadline and review chips keep.
