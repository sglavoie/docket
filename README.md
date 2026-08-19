# tt — Terminal Tasks

`tt` is a local-first, Outliner-first task manager for the terminal. Run `tt`
to open the Bubble Tea interface; readable Markdown remains the canonical task
store. The public command line is intentionally limited to diagnostics,
conflict recovery, workspace relocation, project export, and shell completion.

## Install and start

```bash
go build -o tt .
./tt
```

Move the binary into your `PATH` if desired. A new workspace starts locally in
`~/.tt/`. Use `--data-dir`, `TT_DATA_DIR`, or `tt workspace move` to select a
different location. Moving into iCloud Drive or another synced directory is an
explicit choice; an existing selected iCloud workspace is preserved.

Every run starts in Active Projects, All mode, with no restored search, filters,
selection, or navigation. Active Projects is the base task workspace: every
project you have not tagged `parked`. Press `a` to capture the
first task. At Active Projects scope, capture routes to the protected Unfiled
project with Inbox lifecycle; in a single-project scope, it routes to that
project.

## Daily loop

The Outliner is the only primary task presentation:

1. `a` captures one or more tasks.
2. `gi` moves to Inbox for triage.
3. `$` opens the shared workflow chord, the sigil the grammar already uses for a
   destination.
4. `$b`, `$n`, and `$t` organize work; `$g` starts it and `$d` completes it.
5. Direct metadata keys handle priority, state, dates, tags, and pin.
6. `Space` opens the six occasional task tools.

Together, these operations form one capture, triage, plan, execute, and
complete loop.

## Presentation grammar

Every full-screen presentation keeps one operational title row. Its left side
names the surface, scope, and mode once; active selection, search, filters,
focus, or editor context occupies the middle; and the right side reports the
workspace pulse as `workspace: inbox 3 · today 0 · overdue 1`. Today is shown
even at zero whenever the pulse region fits. Zero Inbox and overdue counts
disappear, and narrow terminals drop Inbox, then overdue, then Today before
shortening required interaction context.

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

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Ctrl+I` returns to the
relocated task.

`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due → Recent, and
`Shift+Tab` reverses that cycle. The `g` chord jumps straight to one mode:
`ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `g@` Due, `gr` Recent.

All mode shows open, undeferred work in this order:

```text
Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday
```

Pinned leads All and Today. Inbox, Next, Backlog, Due, and Recent each answer
one narrow question and show no Pinned section, so their own order stays whole;
a pinned task there wears a bold `*` left of its context badge and title.
Pinned tasks appear once in project groups, each promoted child wearing the bare
`‹` parent marker whose chain Peek names. Groups
sort by their newest pin. Tasks inside a group start newest-first and `J`/`K`
reorder them manually; a pinned subtree ranks by its own root's pin, and `J`/`K`
never move a task across a project group. `Ctrl+J`/`Ctrl+K` take the same route
in one press and one undo entry. All, Inbox, and Backlog preserve hierarchy and
manual sibling order. Today and Next preserve workspace-wide planning order. Review is the sparse set of open tasks
tagged `review`. That list is draw order, not claim order: a pinned task or one
you have planned into Today or Next claims its row there and never also appears
under Review. Due is date ordered. Done, Canceled,
and deferred work stay hidden unless a filter explicitly asks for them.

Every mode groups its tasks by project when you are looking at more than one
project. A project group is a foldable `[slug] (count)` header; focus a single
project and the headers disappear because the header line already names it.
Project groups appear in the order the mode's own order first names them, so
grouping never outranks the order the mode exists to provide: Pinned groups sort
by their newest pin, Due groups by their earliest due date, Recent groups by
their most recent activity, and All, Inbox, and Backlog groups in project
catalog order. Tasks inside a group keep hierarchy, the parent marker, and the
mode's own sort; `J`/`K` never move a task across a project group.

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

`Ctrl+P` goes the other way, from a task to the project holding it. It scopes the
Outliner to the cursor task's project, switches to All so the project's whole
tree is listed, and leaves the cursor on the task you pressed it from. `Esc`
unwinds it and returns to the mode, row and scroll position you left. A task All
would not list — a Done task you reached from Recent, say — keeps the mode you
were in and says so.

## Outliner grammar

| Area | Keys |
| --- | --- |
| Modes | `Tab` / `Shift+Tab` cycle the seven modes; `g` opens the go-to chord: `ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `g@` Due, `gr` Recent |
| Navigation | `j`/`k` or arrows move; `n`/`N` jump to the next/previous section or project header, wrapping at either end; `h`/`l` or left/right walk one fold ladder over sections, child tasks, and checklist items, and `h` on a direct child of a focus root steps out of that Focus frame onto the root; `S` toggles all visible sections; `gg`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `H`/`M`/`L` land on the highest, middle, and lowest visible row without scrolling; `z` opens the scroll chord, which does the opposite and moves the window under the cursor: `zt` puts the cursor's row at the top, `zz` centers it, `zb` puts it at the bottom; `Ctrl+O`/`Ctrl+I` jump backward/forward after a task relocation; `Ctrl+L` enters Recent on the newest activity; `Ctrl+P` opens the cursor task's project in All and keeps the cursor on it; `Enter` focuses descendants; `Shift+Enter` widens focus one level; `Ctrl+Enter` toggles Peek |
| Capture and edit | `a` capture, `A` child, `e` inline task edit, `E` description, `Ctrl+E` external editor, whose title line speaks the same edit grammar as `e`, `O` outline edit the cursor's project |
| Workflow | `$` opens the workflow chord over the ten destinations the `$` capture sigil names: `$i` Inbox, `$b` Backlog, `$n` toggle Next, `$t` toggle Today, `$g` Doing, `$w` Waiting, `$p` Paused, `$m` Someday, `$d` Done, `$c` Canceled; `$w` then asks for a check-back date |
| Metadata | `!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date and defer; `#` tags and context; `*` pin |
| Structure | `[` outdent, `]` indent, `>` Set Parent, `<` Adopt Children, `%` Move to Project, `J`/`K` reorder one step, `Ctrl+J`/`Ctrl+K` reorder to the far end of the visible peers |
| Sorting | `o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `ox` clear; repeat a field key to reverse it |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both |
| Selection | `v` toggles the cursor task; `V` starts/freezes an inclusive visible-task range; `Ctrl+A` replaces it with every visible task |
| Global | `p` Select Project, `Ctrl+N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit |

`@` opens one Set Date picker holding both fields. Its DUE section lists Clear,
Today, Tomorrow, In one week, and In one month; its DEFER section lists Clear
defer, In 1 hour, In 4 hours, In 8 hours, Tomorrow morning, Until tomorrow, For
one week, and For one month. The row you commit decides which field is written,
and the cursor opens in the due section, so a due date is one `Enter` away.
Tomorrow morning is 09:00 on the local wall clock, on daylight-saving transition
days too. A preset is highlighted as the current value only when it is
reproducible from the clock alone, so the three hour rows never highlight. On a
terminal task the defer rows are disabled and say so rather than disappearing,
and the due rows stay reachable beside them.

Inside the `@` picker, `a` opens a typed-date input for any date the relative
presets do not cover: `2026-01-15`, `tomorrow`, `+3d`, `+1w`, `fri`, `next mon`,
or `dec 15`. `+4h` and the other `+Nh` hour offsets parse only when the cursor
sits in the defer section, because a deferral is an instant and a due date is a
calendar day; the due section refuses them by name. Inside the `#` picker the
same key adds a new tag, or a new context when the cursor sits on a context row.

The operation descriptors are the single source for dispatch, contextual
guidance, chord hints, and `?` help. Every key an earlier vocabulary used and
the sigils replaced is simply unbound, with no compatibility action: the old
standalone `b`, `c`, `m`, `t`, `w`, and `P` actions, the comma state prefix, and
`:`, `C`, `R`, and `Ctrl+Esc`.
`d` is unbound: Done stays `$d` alone, `@` answers both dates, and root `z` is the scroll chord prefix.
`$` joins them as a chord prefix rather than a key of its own. Pressing an
unfinished chord and then a key it does not offer names the path you left, as
`$q is not a workflow action`; pressing a key that was never bound does nothing
and says nothing, because it never promised anything.
Review has no key of its own: it is the ordinary `review` tag set from `#`, and
`$review` is a rejected destination, so no `$` terminal could honor it.

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
view. `p` remains Select Project, which changes what you are looking at rather
than where a task lives.

### Outline editing

`O` writes the cursor's project out as one indented outline, opens it in
`$EDITOR`, and applies the whole tree when you save and quit. Indentation is
parentage: two spaces per level, up to the usual depth of six. Every line
otherwise speaks the ordinary capture grammar, so `!2`, `#tag`, `##context`, `@friday`,
`$backlog`, and `*pin` all work, while `>parent` and `<child` are rejected
because the indentation already said it, and `@@defer` is rejected because the
buffer carries no deferral at all. The dump prints the values a task
currently holds, so a pinned task arrives carrying `*pin` and deleting that
token unpins it; there is no `*unpin` here.

```text
# Sonar
- Ship the release [[tt:4f2a9c11]] $doing !1
  - Cut the changelog [[tt:9b31de07]] @friday
  - Tag the build
- Draft the migration notes $backlog #docs
```

The `[[tt:...]]` markers identify tasks that already exist. Keep one and the
task is updated in place, wherever you moved it; drop it, or write a new line
without one, and a new task is created. Delete a line and the task is deleted:
the buffer is the project's whole tree, so absence is how you remove work and
there is no `$delete` sigil. Before anything is applied, `tt` names the tasks
that would go and the attachments that would go with them and asks you to
confirm. Declining cancels the entire apply, edits included, and keeps the file
so you can reopen it. An emptied buffer is the same rule at its limit and
deletes every task in the project; it is refused only when it would delete
nothing at all, which is the truncated-file case.

The buffer holds the project's entire task tree rather than the rows currently
on screen, which is why `O` refuses in Today, Next, Due, and Recent: those modes
order rows by something other than the tree, so the indentation you would be
editing is not the parentage you would be saving. It also refuses while a visual
selection is open, since a selection names rows and the outline names a project.

Nothing is written until the whole buffer validates. If any line is wrong you
get every problem at once, numbered by line, and the file is kept so you can
reopen it, fix the named lines, and run `O` again. A successful apply is one
undoable transaction.

### Task actions

`Space` contains exactly:

1. Comments
2. Attachments
3. Delete

The list filters as soon as printable text is typed. Frequent direct operations
do not repeat in this menu. Delete always asks for confirmation. The checklist
is absent because it is edited on its own rows, and a second route to the same
mutations is the shape this menu exists to avoid.

Press `e` to edit the selected task in place. Append `%project` to move its
subtree, `>parent` to reparent it, `>0` to move it to the project root, or
`<child` once per task you want to adopt; these structural changes commit
atomically with title and metadata edits. `Ctrl+E` opens the same task in an
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

### Filters and search

Filters are session-only refinements of ordinary mode rows. Slash search is a
temporary projection over every nonterminal task in the active project scope,
including deferred work; Esc restores the prior mode, facets, cursor,
scroll, and folds exactly. Search is also the fastest way to jump anywhere:
`Enter`, `Shift+Enter`, and `Ctrl+P` on a result leave search on that task
instead of refusing, so the restored projection lands on the row you searched
for. A mode that cannot list the result falls back to All; a facet that still
excludes it says `Task is hidden by the active facets` and focuses nothing.
Every other operation the projection owns — mode switches, folds, and
reordering — still asks you to exit search first.

| Keys | Effect |
| --- | --- |
| `f!0`, `f!1`–`f!3` | Clear priority filtering or keep High, Medium-or-higher, or Low-or-higher assigned priorities; Unset never matches |
| `f$` | Pick workflow states with OR semantics, including terminal states |
| `fc` | Pick completed at least 7, 30, or 90 local-calendar days ago, or all valid completion ages; also filters states to Done and Canceled |
| `f#` | Pick tags and task contexts with AND semantics; each selected value matches either field, and a value used as a context is listed under `CONTEXTS` above `TAGS` |
| `fr` | Toggle the `review` tag facet |
| `f@` | Open the due-date facet picker: overdue, today, this week, no due date, or deferred-only |
| `fh` | Pick one hierarchy position: parent, child, root, or leaf; parent and leaf read open children |
| `fx` | Clear every facet |
| `/` | Ranked title, description, tag, and project search over every scoped nonterminal task, gated by any capture sigils the query carries |

The search box speaks the capture and edit grammar. Sigil tokens are lifted out
of the query and become an AND gate over the candidate set; whatever text is
left keeps the ordinary fuzzy ranking. A query made only of sigils is a pure
filter, and a query with no sigils behaves exactly as it did before:

| Token | Effect |
| --- | --- |
| `!1`–`!3` | Match that priority exactly, unlike `f!`'s threshold; Unset never matches |
| `#tag` | Match a tag by normalized-name prefix; repeatable, combined with AND |
| `-#tag` | Exclude every task carrying a tag with that prefix |
| `##context` | Match the task context by prefix, a separate axis from tags |
| `@overdue`, `@today`, `@week`, `@none` | Match the same due buckets `f@` offers |
| `@2026-03-05` | Match that exact calendar due day |
| `@@any`, `@@none` | Split the currently deferred population, the way `f@`'s deferred facet does |
| `@@tomorrow`, `@@+2h` | Match tasks whose wake instant falls on that day |
| `$state` | Match a lifecycle state exactly |
| `$today`, `$next` | Match membership in the durable planning list |
| `*pin`, `*unpin` | Match pinned or unpinned tasks |
| `%project` | Narrow the search to matching projects within the active scope |

Search still hides terminal work, so `$done` and `$canceled` are the two tokens
that relax the rule and reveal it. `$today` and `$next` read planning
membership rather than a state, so a planned task also answers `$backlog`.
`%project` can only narrow the scope that is already loaded; it never reaches a
project the current scope or Focus excluded.

Only a token the grammar recognizes but cannot honor blocks the query: an
out-of-range priority such as `!9`, a retired destination such as `$ready`, the
reserved `##0`, and the structural `>parent` and `<child` sigils that belong to
the edit grammar alone. Those refuse by name in the search chrome and list
nothing. Everything else stays prose, so `find @sarah`, `!important`, and
`$100` search for the words you typed.

Search reads case as a narrowing signal, applied to the free text after the
sigils are lifted out. An all-lowercase query matches any casing; a query
carrying an uppercase letter matches case-sensitively, so `API` finds `API` and
not `capitalize`. When a case-sensitive query matches nothing, the search
retries case-insensitively and marks the result `(case ignored)`; the retry
relaxes case alone and never widens the sigil gate. The rule covers the
Outliner's `/`, Notes search, and both help searches; the `project#42`
shorthand, autocomplete, and pickers stay case-insensitive.

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

`Fp`, `FP`, and selecting `parked` inside `F#` are the same switch, so they can
never disagree about which population is loaded. Inside `F#`, `v` toggles a tag
and `Space` over a tag row offers Rename everywhere, Remove everywhere, Park all
members, and Unpark all members across every project carrying it, as one undo
entry. Removing a tag that is a project's only tag asks where those projects
should land first.

Pressing a chord prefix shows its valid continuations in the guidance region. `Esc`
cancels that unfinished chord, and a second key the chord does not offer says so
instead of doing nothing.

### Sorting

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
you return to All, Inbox, or Backlog.

For intentional cleanup, use `fc` to choose an age — it filters states to Done
and Canceled for you — then `Ctrl+A` to select the visible matches, and
`Space` → Delete. Use `f$` afterwards to narrow to Done or Canceled alone.
The confirmation freezes root, descendant, visible/hidden, and attachment counts;
the entire deletion is one undoable transaction.

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
visual selection, or search is open, because rebuilding the rows underneath one
of those would move the ground the user is standing on.

A Waiting task reads as `waiting <age>`, measured from the moment it entered
Waiting rather than from the last time anything touched it, so recording a chase
in the comments does not reset how long the work has been blocked. If it also
carries a due date, that date is the check-back, so the row replaces the plain
date with `check <age>` counting down to chasing it, `check today`, or an
overdue `check +<age>`. Moving a task to Waiting with `$w` therefore asks for the
check-back date immediately; `Esc` there declines the date and the Waiting move
still stands.

A row that cannot fit every indicator sheds them in a fixed order rather than
squeezing the title: checklist, comment, and attachment counts go first because
the checklist rows and Peek still carry them, then the recency and defer
markers, then the waiting age, and the check-back date goes last.

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
lane — Inbox, Next, Backlog, Due, Recent, a search projection, or a filter that
admits a terminal or deferred pinned row into All or Today — wears a bold `*`
before its context badge and title. Rows inside the Pinned lane wear nothing,
because the section header already says it. The marker is reserved columns like
the relation mark and is never dropped.

`Enter` focuses the cursor task's descendants and `Esc` restores the exact
parent cursor and scroll position. A leaf reports `No children to focus`. While
focused, `a` captures a direct child of the focus root rather than a root task
elsewhere. Pressed on a search result, `Enter` dismisses search first and lands
the ordinary projection on that task, so `Esc` unwinds onto its own row; a leaf
still reports `No children to focus`, with the jump already made.
`Shift+Enter` widens in the other direction. Outside Focus it opens Focus on the
cursor task's parent, so the task's own siblings come into view; a root task
reports `No parent to focus`. Inside Focus it climbs one level from the focus
root, again keeping the cursor where it was, until the focus root is a root task
and the next press leaves Focus entirely. A press after that is an ordinary
entry once more, focusing the cursor task's parent.
`Ctrl+Enter` toggles a session-only Peek directly beneath the cursor task. Peek
follows navigation and shows at most five lines of additive detail: up to three
lines of description, a dates line reading
`created 12d · updated 2d · due 2026-08-14 · woke 1d` with empty fields omitted,
attachment names, comment counts, and tags. Checklist items are not among them,
because they are already rows of their own beneath the task. A promoted row's
ancestor chain is named there in full, root-first, as
`Parent: Launch platform › User authentication`; it and a clipped title's
continuation are the two blocks that raise the five-line cap rather than spend
it. `Esc` closes Peek before unwinding Outliner navigation.

State, Today/Next, priority, due, tags, defer, and pin actions apply atomically
to the complete project-qualified selection, including hidden members. `v`
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

## Capture grammar

One nonblank line creates one task. Multiple lines are fully parsed and
validated before one atomic transaction is committed.

```text
!1-3  #tag  ##context  @date  @@defer  $destination  *pin  %project  >parent  <child
```

- `!1` through `!3` set High, Medium, or Low priority.
- `#tag` creates and sets a lowercase ASCII kebab tag on the fly. Input is
  canonicalized (`#Needs / Review` becomes `needs-review`); `#review` feeds the
  Review lane.
- `##context` sets one optional task-level context, normalized by the same rules
  as a tag. It is distinct from the retired v4 project-level context. `##0`
  clears the value on the edit surfaces only; outline editing clears it by
  omission. Therefore `"0"` is permanently reserved as a context name, unlike
  the otherwise legal clearing/reference forms `!0`, `@0`, and `>0`.
- `@date` accepts the supported date expressions and stores a local calendar
  date.
- `@@defer` hides the task until an instant rather than a day, so it also
  accepts the `+Nh` hour offsets a due date refuses. Deferring carries the same
  lifecycle effect the picker's defer section does: the task moves to Backlog, loses any
  Today or Next membership, and a terminal task refuses outright. `@@0` clears
  the deferral on the edit surfaces; the outline buffer has no deferral channel
  and rejects the sigil by name.
- `$destination` accepts `inbox`, `backlog`, `today`, `next`, `doing`,
  `waiting`, `paused`, `someday`, `done`, or `canceled`. Today and Next create
  Backlog tasks with an explicit planning membership.
- `*pin` pins the task, exactly as the `*` key does on an existing row. Only
  `pin` and `unpin` are recognized, and any other value stays in the title, so
  `Refactor *args handling` keeps its `*args`.
- `%project` overrides scope routing.
- `>parent` resolves inside the selected project after project routing.
- `<child` adopts a task that already exists, and may be repeated. `>parent`
  stays singleton because a task has one parent, while a task can gain many
  children at once.

Both structural sigils name a task by exact short ID or by a unique
case-insensitive title prefix, and both resolve in the effective project — the
one `%project` selected, not the one the cursor was in. `<child` is therefore
never a way to pull a task across a project boundary: a reference that names a
task outside the effective project is rejected, and adopting a task never moves
it or its subtree between projects.

Adoption is additive. Children you do not mention keep their parent, an adopted
task keeps its own subtree, and every task already in the project keeps its
place: an adoption re-nests a task, it does not resequence the project. The one
move you can still see is the standing rule that finished work sinks below open
work, so adopting a finished task under an open one lifts it out of the finished
block, and reopening a finished parent lifts it and its subtree with it.
A `$done` or `$canceled` line cannot adopt unfinished work: capture and the edit
grammar both refuse it and name the open task, because a finished parent above an
open child is the one shape the completion rule forbids. Adopting work that is
already finished is fine. A task that is already done and gains an open child
some other way is reopened rather than left in that shape.
There is no clear form: `<0` is an ordinary reference that resolves to nothing
and is reported as such, and a task is detached by giving it a new `>parent` or
`>0` instead. Aliases that name the same task collapse to one adoption, keeping
the first occurrence, so `<2 <Login` adopts task 2 once. A task cannot be both
`>parent` and `<child` of the same line, and across a multiline capture two
lines cannot adopt the same task; both are reported before anything is written.

`^type`, `~recurrence`, priorities `!4`–`!5`, `$ready`, and `$in_progress` are
rejected with replacement guidance. Other aliases such as `priority:`, `due:`,
`state:`, and `proj:` are literal title text, not task attributes. Root capture
defaults to Inbox. A new child defaults to Backlog under its selected parent.

`a` at base scope captures into Unfiled: a task cursor never lends capture its
project. A project header row is the one exception, because it names a project
explicitly rather than incidentally, so `a` there captures into that project.
The composer header names whichever destination it resolved — `Capture →
Unfiled` or `Capture → Client Alpha` — so the two never look alike, and
`%project` still overrides both. `A` on a project header row keeps refusing: a
project has no task to parent a child under.

Inside a task Focus, `a` captures a direct child of the focused task and the
header names it, as `Capture → child of Launch platform`, so the new task shows
up in the focused projection immediately. Focus wins over a project header row
under the cursor. A focus root already at the nesting cap degrades to plain
capture into its project, `Capture → Rich Project`, rather than refusing. `A` is
unchanged and still means a child of the cursor task, one level deeper.

Exact case-insensitive `bug`, `epic`, `story`, `feature`, and `idea` tags color
task titles in that precedence order. Namespaced values such as `type:bug` and
partial matches remain ordinary tags and add no prefix or marker icon. Both tag
pickers group those five under a `SEMANTIC` section above the rest: `#` Set Tags
always offers all five with a one-line summary each, whether or not any task
carries them, while `f#` Filter by Tags stays usage-derived and lists only the
ones in use.

`f#` also gives task contexts their own `CONTEXTS` section above `TAGS`; a value
carried as both a tag and a context is one row there, with the combined count,
because the facet matches either field. In both tag pickers `w`, `s`, `c`, and
`t` jump the cursor to the first `WORKFLOW`, `SEMANTIC`, `CONTEXTS`, and `TAGS`
row, and a hint line inside the overlay names the sections that picker actually
has. The two pickers mean different things by `CONTEXTS`: in `f#` those rows are
facet values to match, while in `#` they are the `##name` radio rows that write
the field, listed above the tag rows the section also collects.

## Workflow and hierarchy

The eight persisted workflow states are:

```text
inbox  backlog  doing  waiting  paused  someday  done  canceled
```

Today and Next are separate project-qualified planning collections, not
lifecycle states. Today and Next are mutually exclusive, and terminal
transitions remove both references. Waiting is an explicit lifecycle state.
Done and Canceled are rejected while a direct child remains nonterminal.

Parents are project-local and hierarchy depth is capped at six.

## Scope, projects, and Notes

The Outliner opens on Active Projects, and scope is session-only. `p` opens
Select Project with search focused immediately
and includes active and archived projects. Each row carries its task count and
its tag chips. `Enter` focuses
the matching project in the same Outliner; `Esc` clears search and leaves the
project surface open. Project administration supports create, edit, pin,
archive, restore, and project Notes. The catalog ID, not a display
name, protects the system Unfiled project.

`v` selects the project under the cursor and `a`/`A` select or clear every
project matching the current search. With a selection, `Space` offers Add tag,
Remove tag, Park and Unpark across all of it, and the whole edit commits as one
undo entry.

### Project tags

Three different things decide where work shows up, and each answers its own
question:

| Concept | Applies to | Question it answers |
| --- | --- | --- |
| Workflow state | a task | What is happening with this task? |
| Tags | a project | How would you label this project, and is it in your working set? |
| Archive | a project | Is this project retired? |

A project carries any number of tags. Most are simply labels you choose. One is
special: tag a project `parked` and it leaves the base Outliner entirely. Its
Today and Next entries are kept exactly as they were, hidden while the project
is parked and restored the moment you untag it. The protected Unfiled project
cannot be parked.

Press `#` on a project header row to edit that project's tags; the same key on a
task row still edits task tags. The picker lists `parked` first with a sentence
explaining what it does, and `a` mints any other tag on the fly. The project
editor also has a comma-separated Tags field, and Select Project tags a whole
selection at once.

Applying `parked` asks first and names what you are about to lose sight of —
"Park Client Alpha? Its 1 task leaves Active Projects." Parking several at once
asks the same way and counts them all. Every other tag applies
straight away. Once parked, the status names both ways back
("Fp to show · u to undo"), the cursor moves to the next project rather than
wherever the vanished rows left it, and the Outliner scope reads
`Active Projects · 1 parked` so the hidden work is never silent.

A parked project is not hidden away. It still appears in Select Project, still
owns its Notes, and still accepts `%project` during capture and inline editing.
`Enter` on it in Select Project opens it as an
explicit one-project focus, anchored with the `parked` tag, and `Esc` returns
to the Active base with nothing left behind.

Archiving is separate. It preserves a project's tags and restoring returns
them unchanged.

`Ctrl+N` opens All Notes, a workspace-wide manager that scans Global plus every
unarchived project, including the protected Unfiled project and every parked
project. On a project header row it opens that project's notes instead. Rows
are ordered by most recent
modification and carry an owner label. `v` opens a searchable All, Global, or
single-project scope picker, while `s` applies a persistent title/body search
inside the selected scope.

`n` (or its `a` alias) always asks for a Global or unarchived-project
destination before starting the editor, and `m` moves the selected note to
another such owner as one atomic workspace transaction. `Enter` opens and edits the selected
note in
`TT_EDITOR`, then `VISUAL`, then `EDITOR`; if none is set, `tt` reports how to
configure one. Notes support open/edit, create, rename, move, delete, undo, and
redo. Note tags, task links, pins, bulk actions, previews, and archived-project
ownership remain excluded.

`T` toggles exactly two maintained machine-local themes: dark and light.

## Undo, recovery, and conflicts

`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed
transactions. Task, project, note, and attachment changes share that history.
A new forward mutation clears redo; a failed operation creates no entry.

Workspace commits validate paths and revisions, stage a durable manifest, and
publish every affected artifact atomically. Startup rolls an interrupted
prepared transaction forward before loading the workspace. Concurrent edits
that cannot be merged produce durable conflict bundles:

```bash
tt conflict list
tt conflict show <id>
tt conflict resolve <id> --use local
tt conflict resolve <id> --use remote
tt conflict resolve <id> --use file --file merged.md
```

## Schema 12 storage and migration

The live portable layout is deliberately small:

```text
<data>/
├── .tt/
│   ├── workspace.json
│   ├── planning.json
│   ├── global-notes/
│   ├── transactions/
│   ├── undo/
│   ├── conflicts/
│   └── backups/
└── projects/<slug>/
    ├── tasks.md
    ├── notes/
    └── .attachments/
```

Project task files use strict schema-11 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, project tags, and the
protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice.

Startup migrates supported schema-11 workspaces to schema 12, supported schema-6
through schema-10 workspaces through the intermediate boundaries, and supported
schema-4/5 workspaces through the whole chain. Each step validates the complete
candidate before mutation and publishes its own immutable rollback archive and
audit first: `pre-simplification-v5-<UTC>.tar.gz` for v4→v5,
`pre-consolidation-v6-<UTC>.tar.gz` for v5→v6, `pre-scope-v7-<UTC>.tar.gz`
for v6→v7, `pre-tag-v8-<UTC>.tar.gz` for v7→v8,
`pre-state-clock-v9-<UTC>.tar.gz` for v8→v9,
`pre-defer-rename-v10-<UTC>.tar.gz` for v9→v10, and
`pre-defer-instant-v11-<UTC>.tar.gz` for v10→v11, and
`pre-context-v12-<UTC>.tar.gz` for v11→v12. The schema-6 conversion preserves
notes and attachment bytes,
converts retained decision text into comments, repairs planning references,
renames the protected project to Unfiled, and deliberately resets the
incompatible pre-v6 undo ledger. The schema-7 conversion folds the retired
per-project grouping fields and the retired workspace scope table into ordinary
project tags, keeping `parked` as the sole behavioral project tag.
Schema 8 canonicalizes all task and project tags to lowercase ASCII kebab form,
audits renames and collisions, upgrades task Markdown to schema 7, and clears
undo/redo history whose retained bytes predate that canonicalization.
Schema 9 upgrades task Markdown to schema 8 by giving every task a durable
`state_changed` clock, backfilled from `updated`. The audit states that the
backfill is an approximation which self-corrects on the task's next state
transition, and undo/redo history is cleared again because its retained bytes
predate the new field.
Schema 10 upgrades task Markdown to schema 9 by renaming the durable deferral
metadata key from `snoozed` to `deferred`, and clears undo/redo history once
more because the retained bytes are raw pre-rename Markdown.
Schema 11 upgrades task Markdown to schema 10 by rewriting each deferral value
from a calendar date to an RFC3339Nano instant, which is what lets a deferral
expire later the same day. A bare date always meant the moment that day began
where it was read, so each one lifts to exactly that instant and every existing
deferral wakes when it always would have; the due date stays a calendar date.
Undo/redo history is cleared again because the retained bytes are raw
calendar-date Markdown.
Schema 12 upgrades task Markdown to schema 11 by adding the optional,
single-valued task-level `context` field. Existing tasks receive an empty value,
so the rewrite preserves their meaning; this field is unrelated to the retired
v4 project-level context that schema 7 folded into project tags. Undo/redo
history is cleared because its retained raw Markdown carries the schema-10
marker that the schema-11 reader rejects.

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent.

There is no down-migration. An older `tt` refuses a migrated workspace as newer
than its supported schema, and no build converts a workspace back. Recovery is
manual: extract the matching archive — `pre-context-v12-<UTC>.tar.gz` for the
schema-12 boundary — into a separate directory and open that copy with a
compatible build. Migration snapshots themselves are never modified.

Run `tt doctor` for read-only integrity checks or `tt doctor --fix` for
explicitly supported repairs. Migration snapshots are never modified.

## Maintenance CLI

```text
tt
tt doctor [--fix]
tt conflict list|show|resolve
tt workspace [location|move]
tt completion bash|zsh|fish
tt project export [slug-or-id]
tt --version
```

Global flags are `--data-dir` and `--no-color`. Run `tt help <command>` for
per-command syntax.

## Development

```bash
just test
./scripts/check_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario script replays the committed final-product TUI workflows plus the
snapshot-first migration and interrupted-activation recovery scenarios. It
re-selects tests `just test` and `just ci` already run, for a faster loop while
authoring one. See
[docs/tui-testing/README.md](docs/tui-testing/README.md).

For data boundaries, transaction mechanics, migration details, and platform
support, read [docs/architecture.md](docs/architecture.md).
