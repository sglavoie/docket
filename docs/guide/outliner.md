# Outliner

How the Outliner draws work, the keys it answers, what a task row says, and
the detail Peek adds. Start here after the README.

## Inbox and the cursor advance

Inbox is the one mode that hands the cursor along. When a triage act takes the
row out of Inbox, the cursor lands on the next task in the sweep rather than
on whatever the row index happens to hold, so it crosses a project group header
instead of stopping on it and lands on a child promoted into its departing
parent's place. Every other mode leaves the cursor where it was, because you
read those modes as much as you empty them. The advance is not a relocation
jump: a departure records nothing for `Ctrl+O`, and undo restores the task
without moving the cursor back to it.

## Presentation grammar

Every full-screen presentation keeps one operational title row. Its left side
names the surface, scope, and mode once; active selection, search, filters,
focus, or editor context occupies the middle; and the right side reports the
workspace pulse as `workspace: inbox 3 · today 0 · overdue 1 · done 4`. Today
and the current local day's Done count are shown even at zero whenever the pulse
region fits; zero Inbox and overdue counts disappear. Narrow terminals drop
Done, Inbox, overdue, then Today before shortening required interaction context.

Today mode also compares its open load with the workspace's 30-day completion
pace in the middle region, for example `today 9 · median 3/day (30d)`. A fresh
workspace with no completion history shows no comparison. Under pressure the
workspace pulse disappears first; the comparison then sheds `(30d)`, then
`median`, and finally disappears before required interaction context shortens.

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, expanded detail, and pending
chords may use two. Required commit and cancellation controls survive before
optional hints. Transient pickers and action sheets own their local guidance
while leaving the host title and workspace pulse visible.

The Outliner's own feedback — a focus notice, or a line such as
`Latest activity: …` — renders in a status strip pinned directly above the
footer, introduced by a dim thin divider. The strip never floats mid-screen on a
short list and never leaves blank padding under itself: it takes its rows from
the task list, and in a narrow terminal it wraps rather than pushing the footer
off screen. It clears itself after a couple of seconds, and immediately on a
mode change, so a message naming a mode can never misreport the one you switched
to. A live visual selection is the exception: its summary stays for as long as
the selection does.

The strip carries one line that is not feedback at all: while the workspace
holds unresolved merge conflicts, an indicator counting them leads the strip and
names `tt conflict list`. It is workspace state rather than a message, so it
neither expires nor clears on a mode change; it leads the transient lines so
they cannot shift it, and it disappears only once the last bundle is resolved.
See [`storage.md`](storage.md) for what a bundle is and how to resolve one.

The strip is also where `Q` expands the narrowing readout: one numbered row per
narrowing the view is under, with the conflict indicator keeping the lead above
it. It grows the strip rather than floating over the list, so the rows it pushes
off are rows the strip was already entitled to.
[`search-and-lenses.md`](search-and-lenses.md) has the readout and its peels.

`gc` answers the question that indicator raises. It opens a read-only overlay
listing every recorded bundle newest first — its short id, the project, how many
conflicts it holds, the action that hit it, and when it was recorded. `Enter`
inspects one, showing exactly what `tt conflict show` prints: the revisions on
each side, whether the original operation still has to be retried, and every
conflicting field with its base, local, and remote values. The pane scrolls with
the ordinary list keys, `Esc` steps back to the list, and a second `Esc` closes
the overlay. Nothing here resolves anything: the browser reads the store and the
record it inspects is reread at the moment you open it, so a bundle another
terminal resolved in the meantime refuses rather than painting a stale record.
Resolving stays in `tt conflict resolve`. With no unresolved conflicts, `gc`
says so in the strip instead of opening an empty overlay.

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Ctrl+I` returns to the
relocated task.

The Outliner marks its cursor row in the margins rather than over the row. One
bright column at the far left says where the cursor is, and the empty columns
past the row's text carry a dim recess saying how far the row reaches. Nothing
between them changes: the multi-select box, the tree connectors and the fold
arrow keep the colors they wear on every other row, and so do the state badge,
the priority-colored title, the tag chips, the due tier, the project badge and
the search-match highlight — all of which read the same under the cursor as
beside it, leaving a waiting or overdue row visibly waiting or overdue while you
stand on it. The bar is the row's leftmost column on every row type: a blank
margin, except on a section header, where it reverses the fold arrow. A cursor
flash floods the whole gutter rather than changing the bar's color, so a pulse
reads as more of the same mark. Every other list — the pickers, the tag and
parent selectors, notes, the project hub — still inverts the whole row, because a
flat list row has no margins to paint.

Every tag carries a color, and it is the same color everywhere a tag is rendered
as itself: the chips on a task row, the `Tags:` line in Peek, and every row of
the tag list the composer's expanded detail opens. Nothing needs configuring — a
tag earns its color from its own name the first time you type it and keeps it
for good, and `T` re-resolves the whole set for the new theme so a tag stays
recognizably itself in dark and light.

A row inside `#`, `f#`, or `F#` is an offer rather than a label, so it shows you
the color it is about to paint. Most of those rows paint nothing else and simply
wear their own chip: the descriptive tag rows, and the tag rows that `#` collects
under `CONTEXTS` because the same value is also in use as a context. A tag that does
something louder shows that instead — a row under `SEMANTIC` wears the title
color it applies, `review` under `WORKFLOW` wears its lane's purple, and a
`parked` project row wears the warning badge it hangs on the project. So the
same tag can read as one color in a picker and another as a chip; when they
disagree, the picker is naming the effect. Two rows stay plain on purpose: the
`##name` radio rows in `#`, which set a field rather than name a tag, and `F#`'s
`Untagged` row, which names the projects carrying no tag at all. `fc`'s rows are
plain for the same reason: a context is a field, not a chip. A Select
Project row is the one place a tag is named without its color, because its tags
share the one secondary line that carries the counts beside them.

Read a chip color as a mnemonic, not a legend. It tells you `#billing` is
reliably that one color; it never tells you what that color means. Six chip
colors cover an unlimited tag vocabulary, so two unrelated tags will sometimes
share one, and nothing in `tt` ever decides anything from a chip color. The
colors that do carry meaning stay louder than the chips: a `bug`, `question`,
`epic`, `story`, `feature`, or `idea` tag colors the whole title, and a chip
never overrides the title beside it.

A row shows at most two chips, and it chooses them instead of showing the first
two you typed: descriptive tags come first in alphabetical order, then the
semantic ones, with a `+N` counting whatever did not fit. A task tagged `ux`,
`bug`, and `billing` reads `#billing #ux +1` however you typed it, because the
red title has already said `bug`. Peek names the whole set in the same order and
the same colors, so a row is always the beginning of its own Peek line. A task
carrying nothing but semantic tags still shows them, because under `--no-color`
the chip is the only channel left.

`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due → Recent, and
`Shift+Tab` reverses that cycle. The `g` chord jumps straight to one mode; the
key table below spells its continuations.

All mode shows open, undeferred work in this order:

```text
Pinned, Today, Doing, Review, Stale, Inbox, Next, Paused, Backlog, Waiting, Someday
```

Pinned leads All and Today. Inbox, Next, Backlog, Due, and Recent each answer
one narrow question and show no Pinned section, so their own order stays whole;
a pinned task there wears a bold `*` left of its context badge and title.
Pinned tasks appear once in project groups, each promoted child wearing the bare
`‹` parent marker whose chain Peek names. Tasks inside a group start
newest-first and `J`/`K` reorder them manually; a pinned subtree ranks by its
own root's pin. `Ctrl+J`/`Ctrl+K` take the same route
in one press and one undo entry. All, Inbox, and Backlog preserve hierarchy and
manual sibling order. Today and Next preserve workspace-wide planning order. Review is the sparse set of open tasks
tagged `review`. That list is draw order, not claim order: a pinned task or one
you have planned into Today or Next claims its row there and never also appears
under Review. Stale follows Review and collects open, undeferred work whose
`UpdatedAt` touch time is at least 90 local-calendar days old. Pinned, Today,
Next, Review, and Doing keep stronger claims, so a task appears only once. Due is date ordered. Done, Canceled,
and deferred work stay hidden unless a filter explicitly asks for them.

Every mode groups its tasks by project when you are looking at more than one
project. A project group is a foldable `[slug] (count)` header; focus a single
project and the headers disappear because the header line already names it.
Project groups put pinned projects first and then sort alphabetically, and that
header order stays the same across sections and modes. Tasks inside a group keep
hierarchy, the parent marker, and the mode's own sort; `J`/`K` never move a
task across a project group.

A project header also names the pressure its group holds, as `overdue 2` and
`today 3` after the count, so folding the group does not hide what is urgent
inside it. A Today group omits its own `today` count because every row there is
already planned for today. When the header is too narrow for the whole suffix
the counts degrade rather than clip: `today` drops first, then `overdue`, so a
count you can read is always the real number.

Recent answers "what did I touch lately". It lists every task updated in the
last 30 days newest-first under Today, Yesterday, Last 7 Days, and Last 30 Days,
grouped by project inside each bucket, and includes Done, Canceled, and deferred
work because all of it is activity. The
order freezes when you enter the mode, so triaging a row from there leaves it
exactly where you are reading it; `gr` re-enters and re-reads activity. `Ctrl+L`
does both from any mode in one press: it re-reads activity and puts the cursor on
the task you touched last, opening a collapsed section or a folded parent only
when that task would otherwise be hidden.

`gp` goes the other way, from a task to the project holding it. It scopes the
Outliner to the cursor task's project, switches to All so the project's whole
tree is listed, and leaves the cursor on the task you pressed it from. `Esc`
unwinds it and returns to the mode, row and scroll position you left. A task All
would not list — a Done task you reached from Recent, say — keeps the mode you
were in and says so.

## Outliner grammar

| Area | Keys |
| --- | --- |
| Modes | `Tab` / `Shift+Tab` cycle the seven modes; `g` opens the go-to chord: `ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `g@` Due, `gr` Recent, `gp` the cursor task's project, `gc` the read-only merge-conflict browser |
| Navigation | `j`/`k` or arrows move; `n`/`N` jump to the next/previous section or project header, wrapping at either end; `h`/`l` or left/right walk one fold ladder over sections, child tasks, and checklist items, and `h` on a direct child of a focus root steps out of that Focus frame onto the root; `s`, `p`, and `t` are the three bulk fold toggles — every visible section header, every project group header, every task subtree — and each expands its whole set if any part of it is folded and otherwise folds the set; `gg`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `H`/`M`/`L` land on the highest, middle, and lowest visible row without scrolling; `z` opens the scroll chord, which does the opposite and moves the window under the cursor: `zt` puts the cursor's row at the top, `zz` centers it, `zb` puts it at the bottom; `Ctrl+O`/`Ctrl+I` jump backward/forward after a task relocation; `Ctrl+L` enters Recent on the newest activity; `gp` opens the cursor task's project in All and keeps the cursor on it; `Enter` zooms in on whatever the cursor names — descendants on a task row, that project on a project sub-header, that section alone on a section header; `Shift+Enter` widens focus one level; `Ctrl+Enter` toggles Peek |
| Capture and edit | `a` capture, `A` child, `e` inline task edit, `E` description, `Ctrl+E` external editor, whose title line speaks the same edit grammar as `e`, `O` outline edit the cursor's project |
| Workflow | `$` opens the workflow chord over the ten destinations the `$` capture sigil names: `$i` Inbox, `$b` Backlog, `$n` toggle Next, `$t` toggle Today, `$g` Doing, `$w` Waiting, `$p` Paused, `$m` Someday, `$d` Done, `$c` Canceled; `$w` then asks for a check-back date |
| Metadata | `!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date and defer; `#` tags and context; `*` pin — the last two read the cursor, so on a project group header they set that project's tags and toggle its pin |
| Structure | `[` outdent, `]` indent, `>` Set Parent, `<` Adopt Children, `%` Move to Project, `~` Set Dependencies, `J`/`K` reorder one step, `Ctrl+J`/`Ctrl+K` reorder to the far end of the visible peers |
| Sorting | `o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `or` flat ranked list while a query is held, `ox` clear; repeat a field key to reverse it |
| Content | `&` opens the cursor task's Attachments panel, `;` its Comments panel — the two capture sigils reaching, as keys, the panels that own add, reorder, unlink, and delete |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both |
| Selection | `v` toggles the cursor task; `V` starts/freezes an inclusive visible-task range; `Ctrl+A` replaces it with every visible task |
| Global | `Ctrl+P` Select Pinned Project, `P` Select Project, `Q` the narrowing readout, `\` Named Lenses, `Ctrl+N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit |

`@` opens one Set Due or Defer picker holding both fields. Its DUE section lists
Clear, Today, Tomorrow, In one week, and In one month; its DEFER section lists
Clear defer, In 1 hour, In 4 hours, In 8 hours, Tomorrow morning, Until
tomorrow, For one week, and For one month. The row you commit decides which
field is written, and the cursor opens in the due section, so a due date is one
`Enter` away.
Tomorrow morning is 09:00 on the local wall clock, on daylight-saving transition
days too. A preset is highlighted as the current value only when it is
reproducible from the clock alone, so the three hour rows never highlight. On a
terminal task the defer rows are disabled and say so rather than disappearing,
and the due rows stay reachable beside them.

Inside the `@` picker, `a` opens a typed-date input for any date the relative
presets do not cover: `2026-01-15`, `tomorrow`, `+3d`, `+1w`, `fri`, `next mon`,
or `dec 15`. The due input also accepts an optional trailing `HH:MM` clock time
(`fri 15:30`, `2026-01-15 09:00`) to set an explicit time-of-day; a task with a
time compares at that instant, so it is overdue once the time passes rather than
at day's end. `+4h` and the other `+Nh` hour offsets parse only when the cursor
sits in the defer section, because a deferral is an instant and a due date is a
calendar day; the due section refuses them by name. Inside the `#` picker the
same key adds a new tag, or a new context when the cursor sits on a context row.

`?` opens the generated key reference for the surface you are on, sectioned by
the same categories the operations declare, with a **Capture & Edit Grammar**
section holding the sigils — `@@defer` among them — that no key spells. `\`
jumps to a section, `/` filters the rows, and the filter reads vocabulary the
labels hide: `defer` finds `@` **Set Due or Defer** and `hidden` finds `f@`
**Filter by Due or Deferred**, each row naming the word that matched it.


Task updates that move the focused task—such as `$g` moving it into In
Progress—add a session-only jump entry. `Ctrl+O` returns to the task that
followed its old position (or the preceding/nearest surviving row), and
`Ctrl+I` returns to the relocated task. Changing mode, scope, search, filters,
sorting, folds, or focus clears this history. Terminals that do not report
enhanced keyboard events encode `Ctrl+I` as `Tab`, so mode cycling remains the
fallback there.

`#` opens Set Tags & Context for the current task or visual selection. Inside
the overlay, `Space` toggles tags, `a` creates and selects a new tag, and
`Enter` applies the pending changes. A task carries any number of tags but at
most one context, so the `CONTEXTS` section is a radio group answering `#`
rather than `Space`: the row you press it on becomes the only checked context,
and the leading `No context` row clears the field the way `##0` does. Tags and
context apply together, in one transaction that undoes in one step.

Over a visual selection those checkboxes are tri-state, because the tasks need
not agree. A tag row is checked when every selected task carries it, `[-]` when
only some do, and empty when none do; applying adds every row you checked,
removes every row you unchecked, and leaves every `[-]` row exactly as it is, so
a tag only part of the selection carries survives an edit to the tags beside it.
`Space` on a mixed row checks it for the whole selection: a mixed row can be
left but not re-entered, and canceling the overlay is how you restore an
untouched set. The `CONTEXTS` rows read `[-]` the same way, marking a context
only some of the selection already holds, and they write nothing unless you pick
one; picking `No context` clears the field across the whole selection the way
`##0` does. A context named `0` is refused, because that name is what clears the
field.

`[` and `]` move the cursor task one step along the tree it is already in: `[`
outdents it to its parent's level, `]` nests it under the peer above. They are
the two structural keys that answer no sigil, because a relative nudge has no
capture form to name. `>` opens
Set Parent, the picker form of the `>parent` edit sigil. It lists every open
task in the cursor task's project, above a leading `No parent (project root)`
row that does what `>0` does. A parent the move would reject — the task itself,
a task inside its own subtree, or one that would push the subtree past six
levels — is still listed, disabled and carrying the reason, so the refusal is
visible before you press `Enter`. Set Parent never crosses a project boundary;
`%` is what moves a subtree between projects.

`<` opens Adopt Children, the picker form of the `<child` edit sigil, and points
the other way: it hands the cursor task new children instead of choosing its
parent. Because `<child` is repeatable, the list is multi-select; because it has
no clearing form, the list is strictly additive. Nothing is preselected, `Space`
checks the tasks to adopt, and `Enter` adopts the whole checked set in one
transaction that undoes in one step. Tasks the cursor task already parents are
absent from the list, so unchecking can never mean detaching — a child leaves
its parent through `>parent` or `>0` and nowhere else. Candidates come from the
cursor task's own project only, and one the adoption would reject is listed
disabled with the reason.

`%` opens Move to Project, the picker form of the `%project` edit sigil, and is
the one key that carries a task out of its project. It moves the whole subtree
in one transaction that undoes in one step, and the destination mints its own
`#N` numbers for every task it receives. The list names each project by slug and
skips the one the task already lives in, since a move to it is not a move. It is
drawn from every non-archived project rather than the ones the Outliner shows,
so a parked project is still a destination — the task simply follows it out of
view. `P` remains Select Project, which changes what you are looking at rather
than where a task lives.

Over a visual selection the same `%` moves every selected subtree into one
destination, in one transaction with one undo entry, however many projects the
selection spanned. The picker names the selection rather than a title
(`Move to Project: 3 selected task(s)`) and withholds a project only when the
whole selection already lives in it, since for a selection spanning several
projects every project is a real destination for the tasks not already there.
A task selected together with one of its own ancestors moves once, as part of
that ancestor's subtree, so the count the status strip reports
(`Move to Project: 3 task(s) → client-alpha`) is subtrees moved, not rows
ticked. A committed move clears the selection, because every address it held
names the project the tasks just left.

`~` opens Set Dependencies, the picker form of the `~task` edit sigil, and edits
the soft references the cursor task stores. It is multi-select and, unlike Adopt
Children, it arrives with the stored set already checked: the edit sigil owns
both `-~task` and `~0`, so unchecking a row has a meaning to carry. `Space`
toggles a row, and `Enter` commits checking as adding and unchecking as removing
— both halves in one transaction that undoes in one step. Closing the picker
having changed nothing commits nothing and reports `Dependencies unchanged`.

Over a visual selection it edits every selected task at once, the way the tags
editor does. A row every selected task stores arrives checked, one only some of
them store arrives in a third state that is neither, and leaving that row alone
is what keeps the tasks that disagree disagreeing: only the rows you actually
move are applied, checking one as an addition everywhere and unchecking one as a
removal everywhere. No selected task is offered as a row, because naming one
would be a reference inside the selection, and adding one by any other route
refuses by name.

The list is two halves. Every reference the task already stores gets a row,
whatever state its target is in and even when the target cannot be read from
here — a reference into a project the Outliner has not loaded keeps its stored
`project-id:task-id` locator and reads `unavailable`, and it stays checkable, so
a stranded reference can always be dropped. The other half is every open task in
every loaded project except the tasks being edited, grouped under its project's
name, which is the whole population an addition may name. References cross
project boundaries, so that grouping is what keeps a filtered list unambiguous.
Dependents are never listed: they are computed from what other tasks store, and
no key edits them.


### Task actions

`Space` contains exactly:

1. Delete

The list filters as soon as printable text is typed. Frequent direct operations
do not repeat in this menu. Delete always asks for confirmation. The checklist
is absent because it is edited on its own rows, and a second route to the same
mutations is the shape this menu exists to avoid. Comments and Attachments left
this menu when they gained `;` and `&`, which is that same rule: a panel reached
by a key is not also a row, so the single-task sheet is now identical to the
bulk one.

The same key on a project group header opens a different sheet, because the row
names a project rather than a task. Project Actions holds Edit project, then
Archive project and Delete project under Danger. Pinning left this menu when the
header gained `*`, the same rule that took Comments and Attachments out of the
task menu. Opening the project and opening its notes are absent for the reason
above: `Enter` and `Ctrl+N` already reach both from that row.

The Attachments panel that `&` opens carries the same split. `Enter` opens
the attachment under the cursor, `a` adds one, `E` opens it in the external
editor, `y` and `Y` copy its path and its `path:line` form, and `J`/`K` reorder
it. Its own `Space` sheet holds the five acts a direct key would have spent for
nothing:

1. Reveal original
2. Open original
3. Open attachments folder
4. Unlink
5. Delete

Unlink and Delete both confirm, and neither has a direct key: removing an
attachment is now a deliberate menu row rather than a single press. `u` and
`Ctrl+R` reach the same workspace ledger they reach everywhere else, so an
attachment added or removed by mistake comes back without leaving the panel, and
the status strip names the transaction that moved.

The Comments panel that `;` opens splits the same way. `Enter` expands the comment under the
cursor, `a` adds one, `e` edits it inline, `E` opens it in the external editor,
and `y` and `Y` copy that comment and every comment as Markdown. Its own `Space`
sheet holds Delete alone, which confirms and has no direct key, so a comment
leaves the list only through a deliberate menu row. `u` and `Ctrl+R` reach the
same workspace ledger here too, so the `u undo` the strip offers after a comment
is added, edited, or removed is answerable without leaving the panel, and the
list rebuilds around what came back. `?` lists that grammar
rather than the Outliner's, and both it and `u` stay literal characters while a
comment is being typed.

Both panels and All Notes move the way every other list does: `j`/`k` or arrows
step one row, and `gg`/`Home` and `G`/`End` reach the first and last. A `g` you
do not complete is abandoned silently and does not swallow the key after it, so
a mistyped prefix costs one press rather than the act the next key names.

Press `e` to edit the selected task in place. Append `%project` to move its
subtree, `>parent` to reparent it, `>0` to move it to the project root, or
`<child` once per task you want to adopt; these structural changes commit
atomically with title and metadata edits. `~task` adds a soft dependency here
too, `-~task` removes one, and `~0` clears them all; a line mentioning none of
the three leaves the stored set exactly as it was, which is what keeps an
external-editor title edit from quietly dropping references it never displayed. `Ctrl+E` opens the same task in an
external editor and reads its title line with the same grammar, so the title,
its sigils, and the body below commit as one transaction and one undo step. The
edit surface is also the only one that accepts `*unpin`: capture has nothing to
clear, and the outline buffer unpins by dropping the `*pin` token from the line.

A title `Ctrl+E` returns is parsed only when it differs from the one the buffer
was opened with, so a literal `Fix bug #123` saved untouched keeps its `#123`. A
title whose value the grammar recognizes but cannot honor — an unknown
`$destination`, a `%project` that does not exist, a reparenting that would make
a cycle — commits nothing and reopens the same buffer with the refusal in its
instruction comment and the description you already typed still in place. Saving
that buffer again unchanged abandons the edit and reports the refusal.


## Rows, checklists, and Peek

### Inline checklist rows

A task that carries checklist items folds them open beneath itself, as `[ ]` and
`[x]` rows indented one level past the task. There is no new key: `l` and `h`
walk a single ladder, so on a task holding both children and a checklist `l`
opens the children first and then the items, and `h` closes the items first and
then the children. `h` on a checklist row returns to the owning task rather than
folding anything, since the item is one rung below it. Checklists start
collapsed, and the task row keeps its `1/3` progress badge either way.

A checklist row is not a task row. Every task action refuses there with one
sentence — "move the cursor to a task row to use this action" — instead of
quietly applying to the owner.

What a checklist row does own is nine operations of its own. Eight of them
answer a key its task-row counterpart also holds:

| Key | Operation |
| --- | --- |
| `x` | Toggle Item — tick or untick it |
| `e` | Edit Item — rewrite its text |
| `J` | Move Item Down — swap it with the item below |
| `K` | Move Item Up — swap it with the item above |
| `Ctrl+J` | Move Item to Bottom — send it past every item below |
| `Ctrl+K` | Move Item to Top — send it past every item above |
| `A` | Add Item — append an item to this checklist |
| `Space` | Checklist Actions — the item menu: Edit Item and Delete Item |

Delete Item is the ninth and answers no key. It is reachable only from the
`Space` menu and confirms before it removes anything.

Visual selection is one of the actions a checklist row refuses, because a
selection names task rows. Entering it from a task row hides any open checklist
items, and `Esc` brings back exactly the rows that were open.

### Peek and multi-select

The Outliner uses the full terminal width at every supported size. Task rows
show workflow state, priority, due and defer dates, and
checklist progress; short task IDs are hidden.

The bold `[context]` segment is reserved like the relation marker: it never
drops when a row narrows. Its value uses at most 12 columns in the row, while
Peek shows the complete value when truncation was necessary. Tags remain a
separate, lower-salience segment and may drop first.

A deferral reads as `defer <date>` only while it is still pending, and the date
carries a clock whenever the task comes back partway through a day. A deferral
to a whole day stays a bare `defer Sep 2`; one that returns later today reads as
`defer 15:04`, and one on another day as `defer Sep 2 15:04`, with the year
added when it falls outside this one. Once the deferral lapses the row says
`woke <age>` for three days, because a task that came back is work again and a
past-dated `defer` would read as still hidden. After that the marker disappears
and the task is ordinary work.

A row that wakes while the Outliner is open returns on its own within a minute,
without a key press. The return waits if an overlay, picker, inline row editor,
visual selection, or the flat ranked list is open, because rebuilding the rows
underneath one of those would move the ground the user is standing on. A query
that is only narrowing the mode's own lanes is not one of those states: it is
re-answered on every rebuild, so the woken task is matched against it like any
other row and returns with them.

A Waiting task reads as `waiting <age>`, measured from the moment it entered
Waiting rather than from the last time anything touched it, so recording a chase
in the comments does not reset how long the work has been blocked. If it also
carries a due date, that date is the check-back, so the row replaces the plain
date with `check <age>` counting down to chasing it, `check today`, or an
overdue `check +<age>`. Moving a task to Waiting with `$w` therefore asks for the
check-back date immediately; `Esc` there declines the date and the Waiting move
still stands.

Due dates and check-backs both render as a filled chip rather than as colored
text, so a date reads as one bounded object beside the titles, tags, and context
badges that share the row. All three tiers are chips — overdue, today, and
upcoming — and they share one hue that no section header uses, so a date is
never mistaken for a section; urgency is carried by how saturated the fill is
rather than by its hue. Under the cursor the chip
flattens into the selection colors like every other colored segment, and under
`--no-color` the fill drops away and the `⊡` glyph marks the date on its own.

A Doing task similarly reads as `doing <age>`, measured from the moment it
entered Doing rather than from its last touch. The neutral age remains beside a
due date when both apply; it reports how long work has stayed active without
turning age alone into another urgency badge.

From the third deferral onward, a row also reads `deferred ×N`. Moving the date
of a still-pending deferral does not add to the count; deferring the task again
after it woke does. This supporting count drops before every other row indicator
under width pressure, while Peek keeps the same token on its existing dates line.

A task holding soft dependencies ends its indicators with `~S/N`: `N` counts the
references it stores and `S` how many of them are settled. `N` is a count of what
the task says, so a reference to finished, deleted, or unloaded work counts
exactly like a reference to open work. `S` is the reading of what those targets
are doing, and it counts a finished task and a task no longer in a loaded project
alike, since neither has anything left to wait on. A set where the two agree
reads as the bare `~N` instead, because a fraction whose halves match says only
that nothing is left, and a trailing `?` says at least one of the references
points into a project this view did not load. Both readings stop at one hop: a
target's own references are its business, and following them would report on
projects this view may never have loaded.

The color says which kind of remainder the fraction has. Amber means at least one
referenced task is still open. Green means everything it points at is finished —
or is no longer in the workspace at all. Muted means nothing readable is left,
but at least one reference points into a project this view did not load, such as
an archived or parked one; the row stays muted rather than green there because it
cannot see whether that work is done, and saying green would claim otherwise.
Peek is still where the targets themselves are named.

A row that cannot fit every indicator sheds them in a fixed order rather than
squeezing the title: the repeated-deferral count goes first, then checklist,
comment, and attachment counts because the checklist rows and Peek still carry
them, then the dependency count because Peek still names it, then the recency
and defer markers, then the waiting age, due date, and Doing age; the check-back
date goes last.

A task shown under a section it did not root — a child promoted into Doing,
Today, Pinned, or a tag facet — ends its title with a bare `‹`, which says only
that the row has a parent the outline above it does not show. Nested rows carry
nothing, because their tree connector already states parentage.

The mirror mark reads the other way: a row whose open children are all placed
somewhere else ends with `›N`, naming how many are hidden — a parent that
started work and sits in Doing while its children wait in Backlog. Only open
children count, so a task whose children are all done or canceled stays quiet,
and a row that shows even one child carries nothing because its connectors and
fold indicator already say it is a parent. A row that is both a promoted child
and a detached parent reads `‹›N`.

Both marks share one segment of fixed columns reserved before the title, so
they neither eat the title on a wide row nor disappear on a crowded one.

The pin marker reads the same way. A pinned task drawn anywhere but the Pinned
lane — Inbox, Next, Backlog, Due, Recent, the flat ranked list, or a filter that
admits a terminal or deferred pinned row into All or Today — wears a bold `*`
before its context badge and title. Rows inside the Pinned lane wear nothing,
because the section header already says it. The marker is reserved columns like
the relation mark and is never dropped.

A pinned project wears the same `*`, so one glyph means pinned everywhere: on
the Outliner project group header, where it sits in the column a pinned task's
mark occupies one row below, on the Select Project rows, and in the `%` Move to
Project list. It is never dropped there either — a crowded project header sheds
its `today` and `overdue` counts first. An archived project keeps the marker,
because the pin is real state that survives archiving, but `*` itself does not
work there and the footer stops offering it: the marker shows the state, the key
hides because the act is unavailable.

`Enter` focuses the cursor task's descendants and `Esc` restores the exact
parent cursor and scroll position. A leaf reports `No children to focus`. While
focused, `a` captures a direct child of the focus root rather than a root task
elsewhere. Pressed on a result in the flat ranked list, `Enter` dismisses the
query first and lands the ordinary projection on that task, so `Esc` unwinds onto
its own row; a leaf still reports `No children to focus`, with the jump already
made. A query that is only narrowing the mode's own lanes has nothing to leave,
so `Enter` there is the ordinary zoom.
`Enter` is that same zoom-in act on the two header rows. On a project sub-header
it scopes the Outliner to that project. On a section header it narrows the mode
to that one section: `Enter` on `Next` in All shows the Next lane for every
project, and in a single-project scope shows only that project's. A collapsed
section zooms open without disturbing its fold, and the title carries a
`Section: Next` segment for as long as the zoom holds. `Esc` restores the whole
mode on the header the zoom was entered from, unwinding after Focus and before
the project scope. Zoom and project scope never compose: `Enter` on a project
sub-header inside a zoom scopes and drops the zoom, and the `Esc` that pops that
scope brings the zoom back with it. An emptied zoom stays put and reports
`No tasks are left in this section. Press Esc to return.` rather than unwinding
under you.
`Shift+Enter` widens in the other direction. Outside Focus it opens Focus on the
cursor task's parent, so the task's own siblings come into view; a root task
reports `No parent to focus`. Inside Focus it climbs one level from the focus
root, again keeping the cursor where it was, until the focus root is a root task
and the next press leaves Focus entirely. A press after that is an ordinary
entry once more, focusing the cursor task's parent.
`Ctrl+Enter` toggles a session-only Peek directly beneath the cursor task. Peek
follows navigation and shows at most five lines of additive detail: up to three
lines of description, a dates line reading
`created 12d · updated 2d · due 2026-08-14 · woke 1d · deferred ×3` with empty fields omitted,
attachment names, comment counts, a `Dependencies 1/3:` line, a `Dependents 2:`
line, and tags. Both reference lines are read-only wherever Peek is open, and
both carry their own count, so a set the entry cap elided still reports its whole
size. `Dependencies` names what this task stores, in the order it stores it, and
its fraction is the row's very numbers; that outgoing set is edited from the row
itself with `~`, never from inside Peek. `Dependents` is computed from the
projects currently loaded, so its count is of the tasks that point back from
there and no key edits it at all. Each entry reads as its project, short ID, title, and
state; a reference whose project is not loaded keeps its stored
`project-id:task-id` locator and reads `(unavailable)` rather than being dropped,
and past the first two entries the line ends `+N more`. Checklist items are not among them,
because they are already rows of their own beneath the task. A promoted row's
ancestor chain is named there in full, root-first, as
`Parent: Launch platform › User authentication`; it and a clipped title's
continuation are the two blocks that raise the five-line cap rather than spend
it. `Esc` closes Peek before unwinding Outliner navigation.

State, Today/Next, priority, due, tags, defer, pin, and Move to Project actions
apply atomically to the complete project-qualified selection, including hidden
members. `v`
toggles the cursor task in or out of the selection, so a selection can name rows
that are not adjacent; movement never changes what `v` picked. `V` starts a
fresh inclusive range at the cursor; movement recalculates it over task rows
only, and `V` or `Esc` freezes it. `v` during a live range freezes it and drops
the cursor row. `Ctrl+A` replaces the selection with all visible task rows. The
selection survives at zero members until `Esc` leaves the mode. Entering
multi-select closes Peek. Structural, edit, Peek,
relation, and clipboard operations do not fall back silently to the cursor
while a selection exists. Successful Delete clears selection; other successes,
cancellations, and failures preserve it.

