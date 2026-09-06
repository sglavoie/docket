# Modes

What each of the seven modes lists, in what order, and how you move between
them. The keys themselves are in [`outliner-keys.md`](outliner-keys.md).

`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due → Recent, and
`Shift+Tab` reverses that cycle. The `g` chord jumps straight to one mode;
[`outliner-keys.md`](outliner-keys.md) spells its continuations.

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
you have flagged with `=review` or the `=` key. That list is draw order, not claim order: a pinned task or one
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

## Inbox and the cursor advance

Inbox is the one mode that hands the cursor along. When a triage act takes the
row out of Inbox, the cursor lands on the next task in the sweep rather than
on whatever the row index happens to hold, so it crosses a project group header
instead of stopping on it and lands on a child promoted into its departing
parent's place. Every other mode leaves the cursor where it was, because you
read those modes as much as you empty them. The advance is not a relocation
jump: a departure records nothing for `Ctrl+O`, and undo restores the task
without moving the cursor back to it.
