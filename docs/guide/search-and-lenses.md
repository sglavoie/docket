# Filters, search, lenses, and sorting

The three session-only axes over an Outliner projection — which rows a mode
holds, which projects load, and the order they are read in — plus the saved
projections that write all of them at once.

## Filters and search

Filters are session-only refinements of ordinary mode rows, and a slash query is
one more of them. Typing a query narrows the mode you are already in: its lanes,
its grouping, its folds, and its order all keep deciding the rows, and the query
only decides which of them survive. A hit's ancestors come along so the row you
found keeps the hierarchy it lives in. Every axis stays live while a query is
held — change modes, set facets, reorder, fold — because a query is a clause, not
a mode. Esc drops the query and restores the mode, facets, sort, cursor, scroll,
and folds the search was typed over, so a narrowing typed inside the search
leaves with it.

`or` is the other way to read the same query: one flat list in relevance order,
with no lanes, no headers, and no folds, over the whole project scope rather than
the mode's own membership. It is a presentation you ask for, never one you land
in by typing, and it goes away with the query that gave it rows. The flat list is
where search is also the fastest way to jump anywhere: `Enter`, `Shift+Enter`,
and `gp` on a result leave it on that task, so the restored projection lands on
the row you searched for. A mode that cannot list the result falls back to All; a
facet that still excludes it says `Task is hidden by the active facets` and
focuses nothing.

Find then narrow: the row facets and the sort axis compose with a query either
way, so any `f` facet applies as an AND gate before scoring exactly like a query
sigil. Under the flat list an `o` sort reorders within a relevance tier only — a
description hit never floats above a title hit — and it is the one place a
fixed-order mode's order can be replaced, since the flat list is not that mode's
order. The population facets, the `F` family, are the only thing the flat list
takes away, since admitting a project needs a reload rather than a rebuild; a
query that is merely narrowing leaves them alone.

Done, canceled, and deferred work is admitted when the query says so and never
otherwise. `$done`, `$canceled`, `?completed`, `fa`, and the deferred forms
`@@any`, `@@tomorrow`, and `f@`'s deferred row each open the population they
name; nothing else does, in either presentation.

| Keys | Effect |
| --- | --- |
| `f!0`, `f!1`–`f!3` | Clear priority filtering or keep High, Medium-or-higher, or Low-or-higher assigned priorities; Unset never matches |
| `f$` | Pick workflow states with OR semantics, including terminal states |
| `fa` | Pick completed at least 7, 30, or 90 local-calendar days ago, or all valid completion ages; also filters states to Done and Canceled |
| `f#` | Pick tags with AND semantics; the task context is a separate axis on `fc` |
| `fc` | Pick task contexts with OR semantics, since a task carries at most one; the leading `No context` row admits the tasks carrying none |
| `fr` | Toggle the `review` tag facet |
| `f@` | Open the Filter by Due or Deferred picker: five shortcut rows writing `@overdue`, `@today`, `@week`, `@0`, and `?deferred`, each labeled with the token it writes |
| `fh` | Pick one hierarchy position: parent, child, root, or leaf; parent and leaf read open children |
| `f~` | Pick one soft-reference position: `?blocked` names other work, `?blocking` is named by other work; each row is labeled with the token it writes |
| `fs` | Toggle open, undeferred work untouched for at least 90 local-calendar days; unavailable in Recent, whose entry clears only Stale and reports `Stale filter cleared: Recent only includes the last 30 days` |
| `f*` | Toggle the pinned facet, the same `*pin` a typed query spells |
| `fx` | Clear every facet |
| `/` | Narrow the mode by title, description, checklist, comment, tag, context, attachment, and project, gated by any capture sigils the query carries; `or` reads the same query as one flat ranked list |

A chord and a typed query are two ways of writing the same narrowing, so the
title row shows what you narrowed by in the sigil spelling you would have typed:
`Filters: !2+ #focus @today` rather than a vocabulary of its own. A facet you
set with `f` and the same narrowing typed into `/` read identically, which is
also what a saved lens holds.

`f!` writes a threshold and the typed sigil does not: `f!2` means
`!2+`, medium-or-higher, and typing `!2` alone matches medium exactly. The `+`
suffix is the whole difference, and it is spelled out in the chip so the chord's
reading is never implied.

Repeated `##context` names union. A task carries at most one context, so
intersecting two names could only ever match nothing; `##home ##errand` is the
work in either place, and repeating the name is not reported as a conflict the
way it is while you are writing a task. Tags are multi-valued and keep
intersecting, so `#a #b` still means both.

Results rank title matches first, then each other field in turn, and a result
whose title does not carry the match says where it came from with a muted
`· description`, `· checklist`, `· comment`, `· tag`, `· context`,
`· attachment`, `· project`, or `· details` note. That note is the first thing a
narrow row drops, so the title stays readable.

The search box speaks the capture and edit grammar. Sigil tokens are lifted out
of the query and become an AND gate over the candidate set; whatever text is
left keeps the ordinary fuzzy ranking. A query made only of sigils is a pure
filter, and a query with no sigils behaves exactly as it did before:

| Token | Effect |
| --- | --- |
| `!1`–`!3` | Match that priority exactly, unlike `f!`'s threshold; Unset never matches |
| `!2+` | Match that priority and every more urgent one, the way `f!`'s threshold reads |
| `#tag` | Match a tag by normalized-name prefix; repeatable, combined with AND |
| `##context` | Match the task context by prefix, a separate axis from tags |
| `@overdue`, `@today`, `@week`, `@none` | Match the same due buckets `f@` offers |
| `@2026-03-05` | Match that exact calendar due day |
| `@+4h` | Match tasks due at or before that rolling window, including already-overdue work |
| `@@any`, `@@none` | Split the currently deferred population, the way `f@`'s deferred facet does |
| `@@tomorrow`, `@@+2h` | Match tasks whose wake instant falls on that day |
| `$state` | Match a lifecycle state exactly |
| `$today`, `$next` | Match membership in the durable planning list |
| `*pin`, `*unpin` | Match pinned or unpinned tasks |
| `~task` | Match tasks that depend directly on that target; repeatable, combined with AND |
| `%project` | Narrow the search to matching projects within the active scope |
| `?property` | Match a property computed from the workspace rather than stored on the task |
| `-token` | Remove whatever the same token without the `-` would have matched |

Search still hides terminal work, so `$done` and `$canceled` are the two tokens
that relax the rule and reveal it. `$today` and `$next` read planning
membership rather than a state, so a planned task also answers `$backlog`.
`%project` can only narrow the scope that is already loaded; it never reaches a
project the current scope or Focus excluded.

A leading `-` always means *remove*, and what it removes from is whatever you
are working on. While you are writing a task, `-#tag` takes a tag off it. In the
search box there is no task to edit, so the same `-` takes rows out of the
result: `-!1`, `-#tag`, `-##context`, `-@today`, `-@@tomorrow`, `-$doing`,
`-*pin`, `-%project`, `-~task`, and `-?stale` each subtract exactly what the
token without the `-` would have selected. That is the whole rule — every
negation accepts the same values, offers the same completions, and reports the
same problems as its positive twin, so there is nothing extra to learn. Several
negations narrow together, and they compose with the positive tokens in the same
AND gate: `#backend -$waiting` is the backend work you are not waiting on.

`?` asks a question the workspace answers rather than matching something stored
on the task:

| Token | Matches |
| --- | --- |
| `?stale` | Open, undeferred work untouched for at least 90 days, the same population `fs` toggles |
| `?deferred` | Tasks whose wake instant has not arrived |
| `?leaf` | Tasks with no open direct child |
| `?parent` | Tasks with at least one open direct child |
| `?completed:90d` | Tasks finished at least that many days ago; like `$done`, this one reveals terminal work |
| `?blocked` | Tasks that reference another task |
| `?blocking` | Tasks another task references |

`?blocked` and `?blocking` report that a reference exists, in one direction or
the other, one hop and never further down a chain. They say nothing about
whether the other task is finished, because a reference is a note to yourself and
never decides what is ready; they are also the two rows `f~` offers, so the
picker and a typed query narrow by the same words. `?completed` takes the `:Nd`
age and the other six take no value at all.

Only a token the grammar recognizes but cannot honor blocks the query: an
out-of-range priority such as `!9`, a retired destination such as `$ready`, the
reserved `##0`, the structural `>parent` and `<child` sigils that belong to the
edit grammar alone, a `~0` that would clear a stored set search does
not hold, a `~task` naming nothing the loaded population depends on, a `?stale:90d`
or `?completed:0d` whose qualifier the property cannot take, an `&link` naming something genuinely attachable, which
belongs to capture and to the task's Attachments panel, and a paired
`;comment;`, which belongs to capture and to the task's Comments panel. Those
refuse by name in
the search chrome and list nothing. Everything else stays prose, so `find @sarah`, `!important`, and
`$100` search for the words you typed.

Search reads case as a narrowing signal, applied to the free text after the
sigils are lifted out. An all-lowercase query matches any casing; a query
carrying an uppercase letter matches case-sensitively, so `API` finds `API` and
not `capitalize`. When a case-sensitive query matches nothing, the search
retries case-insensitively and marks the result `(case ignored)`; the retry
relaxes case alone and never widens the sigil gate. The rule covers the
Outliner's `/`, Notes search, and both help searches; the `project#42`
shorthand, autocomplete, and pickers stay case-insensitive.

The input remembers the queries you submitted. Inside the `/` box, `↑` walks
back through them newest first and `↓` walks forward again, ending on whatever
you had half-typed when the walk started, so reaching for an earlier query never
costs the one in your hands. Each distinct query is held once however often you
submit it, only a submitted query joins the ring — one abandoned with `Esc` does
not — and typing ends the walk, so the next `↑` starts from the newest again.
The ring is session-only and written nowhere: it survives closing and reopening
the input, and a relaunch starts empty. A query worth keeping is a lens. While
the suggestion pane is open it owns `↑` and `↓` for its own cursor; one `Esc`
closes it and gives both keys back to the walk.

The lowercase `f` chord refines the rows a projection already holds. Uppercase
`F` decides which projects load at all, so each of its facets reloads:

| Keys | Effect |
| --- | --- |
| `F#` | Pick project tags with AND semantics, plus an `Untagged` entry for projects carrying none |
| `Fp` | Add parked projects to the population |
| `FP` | Isolate the parked projects |
| `Fa` | Add archived projects to the population |
| `FA` | Isolate the archived projects |
| `Fx` | Clear every project filter |

Each population axis has three states — excluded, included, isolated — and the
key that selects a state also clears it when pressed again. The two axes compose
with AND, so `FP` with `FA` names the projects that are both archived and
parked.

Choosing a project and choosing a population are two different questions. `gp`
on a row, `%slug` inside a query, and the scope a lens saves all answer the
first one — which projects this view is over — and they write the same axis, so
they can never point at different sets. Scope is a frame you are standing in
rather than a clause of a query: a search runs inside the frame it was typed in,
a lens moves the frame, and the frames stack, so changing scope under a held
query re-answers that query in the new frame instead of editing it. `F` answers
the second question: which projects exist to be looked at at all, archived and
parked included. That is why the `F` facets are not available while the flat
ranked list is on screen; that list is built over the rows that are loaded, and a
facet that changes which rows load cannot compose into it. Leave the flat list
and the facet is there again.

`Fp`, `FP`, and selecting `parked` inside `F#` are the same switch, so they can
never disagree about which population is loaded. Inside `F#`, `v` toggles a tag
and `Space` over a tag row offers Rename everywhere, Remove everywhere, Park all
members, and Unpark all members across every project carrying it, as one undo
entry. Removing a tag that is a project's only tag asks where those projects
should land first.

Pressing a chord prefix shows its valid continuations in the guidance region. `Esc`
cancels that unfinished chord, and a second key the chord does not offer says so
instead of doing nothing.

### Reading and peeling the narrowing

Narrowings stack, and after a few of them the title row is a summary rather than
an inventory. `Q` expands that summary into the status strip under the heading
`Narrowing this view`: one numbered row per narrowing you can take away on its
own, read off the live view every time it is drawn. When nothing is narrowing the
view it says so — `nothing: every axis is at its default` — rather than showing an
empty list.

The rows are ordered the way the title reads, widest context first: the lens, the
project scope, the mode, the query, then the facets. A facet row is spelled in
the same sigil tokens the chip above it carries, so `2 filter !3+` and
`3 filter *pin` are the query you would have typed. A population row is labeled
`population` and marked `(reload)`, because taking it away changes which projects
load rather than which rows survive. Nothing is inferred: a mode narrows by
membership rather than by a clause, so its row names the mode and says
`(peels to All)` instead of spelling clauses the mode never declared.

| Keys | Effect while the readout is open |
| --- | --- |
| `1`–`9` | Peel that row alone, leaving every other narrowing standing; a digit past the stack reports `Nothing to peel there` and writes nothing |
| `x` | Peel every row, from the top down, until the view is at its defaults |
| `s` | Save the narrowing as a named lens, the same Create the lens picker runs |
| `Q`, `Esc` | Put the readout down; any other key closes it rather than acting behind it |

Peeling is the ordinary key's own act, so it can never reach a state you could
not have typed: peeling the lens row leaves the lens the way `Esc` does,
restoring the view it opened over rather than stripping one clause out of the
middle of it, and peeling the mode row returns All. Peeling is idempotent at
zero — with nothing left to peel, the digits and `x` report that and change
nothing. Peel-all can report a higher count than the rows you were looking at,
since leaving a search restores the projection it was typed over and that can put
a mode row back for the loop to peel in turn.

A stack deeper than nine rows keeps the digits as its vocabulary and names the
remainder on a line of its own — `N more narrowing(s) below the peel digits; peel
one to reach them` — rather than dropping rows the readout cannot offer a key
for. Under the flat ranked list, peeling a population row refuses with
`Exit search before changing filters`, the same refusal the `F` family gives
there and for the same reason.


### Named lenses

`\` opens Named Lenses, the saved-projection list. A lens holds one whole
Outliner projection: the mode, its own narrowing query, both filter families,
and the sort axis. Applying one writes every axis it names at once, so a lens is
a view you return to rather than a filter you rebuild.

The list opens on two action rows — `Save current view` and, directly beneath
it, `Add built-in view` — followed by every saved lens in alphabetical order.

| Keys | Effect inside the picker |
| --- | --- |
| `Enter` | Apply the lens under the cursor; on `Save current view`, name and save what you are looking at; on `Add built-in view`, open the built-in gallery |
| `1`–`9` | Apply the Nth saved lens shown, exactly as `Enter` on that row would; the digit is drawn in a column ahead of the row it names. Both action rows are unnumbered, and a narrowed list renumbers from one, so a digit always counts the saved rows on screen. A tenth saved lens carries no digit |
| `Space` | Open the management sheet for the lens under the cursor: Rename, Replace from current view, Delete. On either action row it says which key that row answers to instead |
| `/` | Narrow the saved list by name; both action rows stay put while the saved rows filter. `Esc` clears the typed query, then leaves the search, then closes the picker |
| `Esc` | Close the picker without changing the view |

Applying, saving, renaming, replacing, and deleting write the lens list alone,
outside the workspace history, so none of them records an undo entry and `u`
never takes a lens back. A write that fails changes nothing: the list, the view,
and the saved records stay exactly as they were, and the picker keeps the
failure on screen.

An active lens names itself in the title row as `Lens: <name>`, and gains a `*`
as soon as the live view stops matching what was saved — a mode change, a
filter, a sort, or an edited query. `Esc` restores the exact view the lens
replaced, not an intermediate one: switching straight from one lens to another
keeps the original restoration point, so a single `Esc` still lands where you
left. Lenses never stack.

While a lens is active `/` edits the lens's own narrowing query instead of
opening a scope-wide search, and the title row reads `Lens query:` while you
type. `Enter` keeps what was typed, `Esc` puts back the query the edit opened
on. A lens query marks the titles it matched exactly as a search does, since
those are the rows it narrowed to.
The two queries never stack: an ordinary search cannot survive underneath a
lens, so applying or saving one from accepted search results installs the
projection and leaves the search behind — and the same `Esc` that dismisses the
lens hands that search back with the rest of the view. Saving keeps the query
itself, though: a lens saved or replaced from accepted search results records
the query those rows came from as its own narrowing query, so reopening it
narrows the saved mode and facets by it.

A lens owns project scope too, and saving is what decides whether it does.
Saving or replacing a lens from inside a single-project scope records that
project, so reopening the lens returns to it. One saved from the unscoped
Outliner records no scope at all, and no scope means the lens leaves yours
alone rather than forcing you back to every project. A lens whose project has
since been archived, parked, or deleted refuses by name instead of opening
somewhere else, and refusing costs you nothing: the view you were in is
untouched. Entering or leaving a project scope while a lens is live marks it
modified (`*`), the same way changing its mode or a facet does — `Replace from
current view` is what adopts the scope you moved to.

Applying a lens that names no scope inside a single-project scope leaves that
scope alone, and the lens's project facets stay inert until you leave it.
Swapping one project scope for another keeps them inert; explicitly clearing the
scope is what finally hands them the population, and the scope stands until the
projects they select have loaded, so a read that fails leaves you exactly where
you were.

A workspace that has never saved a lens is seeded with three ordinary starters —
`Waiting`, `High Priority`, and `Stale`. Nothing treats them as special: rename,
replace, and delete reach them like any other record, and deleting every lens is
durable rather than a request to reseed.

Those three starters are the only ones seeded for you, but they are three of a
catalog of nine. `Add built-in view` opens `Add Built-in View`, the gallery of
all nine, in alphabetical order with the projection each one installs:

| Built-in view | Projection |
| --- | --- |
| `Active Work` | All · states: doing · sort: priority ↑ |
| `Cleanup 90d` | All · states: done,canceled · done: 90d · sort: updated ↑ |
| `Deferred` | All · deferred · sort: updated ↑ |
| `High Priority` | All · !1 · sort: priority ↑ |
| `Overdue` | Due · due: overdue |
| `Parked Work` | All · parked: only |
| `Review Queue` | All · #review · sort: updated ↑ |
| `Stale` | All · stale · sort: updated ↑ |
| `Waiting` | All · states: waiting |

`/` searches the gallery by name and by that projection, so `doing` finds
`Active Work` and `canceled` finds `Cleanup 90d`. `Enter` adds the template
under the cursor and applies it in one act: the record is written first, the
view follows, both overlays close, and the strip reads
`Added built-in view: <name>`. A write that fails leaves the gallery for the
picker it opened over, keeps the reason on screen, and changes neither the saved
list nor the view. `Esc` leaves the gallery for that picker too.

What the gallery adds is an ordinary saved lens from that moment on: rename,
replace, and delete reach it exactly as they reach a lens you saved yourself.
Each template carries a stable identity of its own, and that identity — not its
name — is what the gallery reads to decide whether you already have it. A
template you added is greyed as `Already added as "<current name>"`, and stays
greyed under whatever you have since renamed or replaced it with; a template
whose name some unrelated lens of yours already answers to is greyed as
`A lens named "<name>" already exists`. Deleting an added template hands it back
to the gallery.

Definitions live in the workspace and survive a restart, and so does the
lens you had open. A launch still builds the same clean Active Projects, All
view it always did — no navigation, folds, cursor, or selection come back — and
then reapplies that lens's saved axes on top of it: its mode, its narrowing
query, both filter families, its sort, and its project scope. Its `Esc` is the
one you would expect: it lands on the Active Projects, All view a lens-free
launch would have shown, so a lens you no longer want costs one key. Dismissing
it is durable too, and a lens the pointer names but that can no longer be opened
— deleted on another machine, or scoped to a project since archived, parked, or
deleted — is quietly dropped, and the launch is the ordinary one.

Because the definitions live in the workspace, a lens you save on one machine is
a lens waiting for you on the next one, with its whole projection intact. The
lens you had open is the half that stays behind: it is a note about where you
were on that machine, so a second machine opens on the clean Active Projects,
All view and on whichever lens you last left open there. The first time a
machine opens a workspace it merges any lenses it had saved for itself into the
shared list without dropping either side, so nothing you built on either machine
is lost — see [Storage, undo, and migration](storage.md).


## Sorting


Lowercase `f` decides which rows a projection holds and uppercase `F` decides
which projects load, so the third axis, `o`, decides the order those rows are
read in:

| Keys | Effect |
| --- | --- |
| `o@` | Order by due date |
| `o!` | Order by priority |
| `oc` | Order by creation date |
| `ou` | Order by last touch |
| `ot` | Order alphabetically by title |
| `ox` | Clear the sort and restore the mode's own order |
| `or` | Read the held query as one flat relevance-ordered list instead of the mode's own lanes; only offered while a query is held |

A field key installs its order ascending, and pressing the same key again
reverses it, so there is no separate reverse key. The active order joins the
title row as a `Sort: <field> ↑` badge.

Sorting is a session-only refinement, like a filter. It orders tasks inside the
group they already belong to, so it never changes which project group comes
first and never touches an order you built by hand: Pinned, Today, and Next keep
their manual sequence whatever the axis says.

Today, Next, Due, and Recent each own their order — a plan you arranged, a due
date, an activity clock — so a sort key there records your choice and reports
that the mode keeps its own order. The recorded order installs itself as soon as
you return to All, Inbox, or Backlog, and `or`'s flat list takes it immediately,
since that list is not the mode's order to begin with.

For intentional cleanup, use `fa` to choose an age — it filters states to Done
and Canceled for you — then `Ctrl+A` to select the visible matches, and
`Space` → Delete. Use `f$` afterwards to narrow to Done or Canceled alone.
The confirmation freezes root, descendant, visible/hidden, and attachment counts;
the entire deletion is one undoable transaction.

