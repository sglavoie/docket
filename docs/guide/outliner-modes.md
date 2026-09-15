# Modes

What each of the seven modes lists, in what order, and how you move between
them. The keys themselves are in [`outliner-keys.md`](outliner-keys.md).

`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due → Recent, and
`Shift+Tab` reverses that cycle. The `g` chord jumps straight to one mode;
[`outliner-keys.md`](outliner-keys.md) spells its continuations.

All mode shows open, undeferred work in this order:

```text
Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday
```

Pinned leads All and Today. Inbox, Next, Backlog, Due, and Recent each answer
one narrow question and show no Pinned section, so their own order stays whole;
a pinned task there wears a bold `*` left of its title.
Pinned tasks appear once in project groups, each promoted child wearing the bare
`‹` parent marker whose chain Peek names. Tasks inside a group start
newest-first and `J`/`K` reorder them manually; a pinned subtree ranks by its
own root's pin. `Ctrl+J`/`Ctrl+K` take the same route
in one press and one undo entry. All, Inbox, and Backlog preserve hierarchy and
manual sibling order. Today and Next preserve workspace-wide planning order. Review is the sparse set of open tasks
you have flagged with `=review` or the `=` key, plus scheduled check-ins whose
day has arrived. That list is draw order, not claim order: Pinned, Today, and
Next claim every candidate first, and Doing keeps a scheduled-only reminder in
place; a manual review flag retains Review precedence over Doing. A task never
appears twice. Old untouched work stays in its lifecycle group rather than
being promoted automatically; `fs`, `?stale`, and the Stale lens remain the
explicit cleanup routes. Due is date ordered. Done and Canceled work stay hidden
unless a filter explicitly asks for them; deferred work does too unless its
manual review flag raises it.

Every mode groups its tasks by project when you are looking at more than one
project. An open group draws no heading at all: each of its tasks carries its
project's `[slug]` just before the title, and the row you would have spent on a
heading goes to the tasks instead. Fold the group with `p` or `h` and that row
comes back as a `[slug] (count)` summary, which is also where you read a slug
too long to fit beside a title. Focus a single project and the whole axis goes
away — labels and summaries both — replaced by one `[slug]` header at the top
of the projection. That one is context rather than a group: it carries no fold
arrow and no count, `h` and `l` do nothing on it, and `Enter` has nowhere to go
because you are already inside that project. What it does carry is the project
itself, so `e`, `#`, `*`, `C`, `;`, and `Space` all act on the focused project
from that row. It stays there in every mode and in ranked search, and it
stays there when the mode, a query, a lens, or a subtree focus leaves nothing to
list — the explanation of why the view is empty is printed underneath it.
Project groups put pinned projects first and then sort alphabetically, and that
order stays the same across sections and modes. Tasks inside a group keep
hierarchy, the parent marker, and the mode's own sort; `J`/`K` never move a
task across a project group.

A project header also names the pressure its group holds, as `overdue 2` and
`today 3` after the count, so folding the group does not hide what is urgent
inside it. Both counts are bold, at the same weight as a due-today or overdue
date on a row, and they keep two hues because they answer two questions:
`overdue` counts deadlines that have passed, `today` counts the tasks you
planned for today, and a task can be either without being the other. A Today
group omits its own `today` count because every row there is
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

`g~` walks the reference graph the same way. It opens a picker over the cursor
task listing Dependencies — what this task stores, in the order it stores it —
and then Dependents — the tasks pointing back at it from the projects currently
loaded, which is why that section is labelled as such. Each row reads as its
project, short ID, title, and state, `/` searches the two sections, and a
reference whose project is not loaded stays on the list as a disabled row you
cannot hop to. `Enter` opens the chosen task's project in All and puts the cursor
on it, opening a collapsed section, project group, or folded parent only when the
target would otherwise be hidden and admitting a terminal or deferred target
only for that reason. A target that went away between the listing and the press
leaves the picker open and says why, moving nothing.

Every hop stacks a return, so `Esc` walks them back one at a time: A → B → C
comes back to B and then to A, landing on the task you pressed `g~` from with the
mode, query, filters, sort, scope and cursor you left. It comes after Peek and a
selection in the same unwind, and it never reopens the picker. A lens in force
when you hop is put aside for the trip and restored on the way back, without
being rewritten. `0` is the way out of a long walk: it drops the whole trail
along with the rest of the view.

The seven modes answer to the digits `1`–`7` as well, in the order `Tab` cycles
them: `1` All, `2` Today, `3` Inbox, `4` Next, `5` Backlog, `6` Due, `7` Recent.
A digit is the same act as the `g` chord naming the same mode rather than a
second one, so `5` and `gb` are two spellings of one jump and `?` help lists
them together as `gb/5`. They are root keys, so a digit typed into a search
query, a capture buffer, or an open picker belongs to that surface: inside the
`Q` readout a digit peels the row it numbers, and inside Named Lenses `1`–`9`
apply the lens on that row.

`0` leads that row and names no mode. It returns the whole view to the one a
lens-free launch produces — Active Projects, All, no query, no filters, no sort,
no focus or zoom, no project scope, and no lens applied — and names the axes it
had to clear, or says `Already at the launch view` when there was nothing left
to put down. A lens is only put down, not forgotten: the same lens opens again
at the next launch, which is what makes `0` a reset of the view rather than of
what you saved.

## Where the cursor lands after a mutation

Every mode keeps the cursor on the work you were reading. What decides this is
the group the row was in — the lifecycle lane, the planning lane, or the project
block the cursor sat in when you pressed the key — not whether the task is still
drawn somewhere.

A task that is still in that group is followed to wherever the group now holds
it, and the row you acted from goes on the jump stack, so `Ctrl+O` comes back to
it. That is what happens when you reorder, reparent, rename, or reprioritize
something in place.

A task that left the group — completed out of Backlog, moved to another project,
edited into a value your filters no longer admit, deferred out of the mode
entirely — is not followed. The cursor continues the sweep from the position it
vacated: first a child promoted into its parent's place, then the next task you
did not select, then the previous one. Group headers and checklist rows are
skipped, and in a bulk act the rest of the selection is skipped too, so the
cursor crosses a project group header instead of stopping on it. If nothing is
left to land on, the cursor stays where the rebuilt list puts it.

None of that is a relocation jump: a departure records nothing for `Ctrl+O`, and
undo restores the task without moving the cursor back to it. A mutation you
started and then walked away from cannot take the cursor either — moving it
yourself before the write lands cancels the follow.
