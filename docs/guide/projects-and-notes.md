# Projects, scope, and Notes

How project scope narrows the Outliner, what Select Project administers, what a
project tag decides, and the workspace-wide Notes manager.

## Common actions

| Goal | Keys | Example |
| --- | --- | --- |
| Work in one project | `P`, choose a project, `Enter` | Focus `web` before capturing its next task |
| Reach a pinned project | `Ctrl+P` | Open a frequently used project |
| Create or edit a project | `P`, then `a` or `e` | Set its name, slug, and description |
| Park a project | `#` on the project, add `parked` | Remove inactive work from Active Projects |
| Read notes | `Ctrl+N` | Open Notes from the Outliner or a project |

Project scope and project search are different: `P` searches project metadata;
`/` in the Outliner searches the work. Unfiled is protected and cannot be parked
or deleted. [Project tags](#project-tags) and [All Notes](#all-notes) cover the
remaining actions and exceptions.


## Scope and Select Project

The Outliner opens on Active Projects, and scope is session-only. `Ctrl+P`
opens Select Pinned Project with only pinned non-archived projects, including
parked ones. It keeps the search, open-task filter, previews, selection, and
actions of the full Select Project surface below — the Project Hub, or Hub for
short; `Enter` focuses the selected project. With no active pins it explains
that `Esc` returns to the Outliner, where `P` still opens the full Hub.

`P` opens Select Project on a live cursor, browsing active, parked, and archived
projects under one heading each. Each row carries its open task count beside its
total and its tag chips, so a project whose tasks are all finished reads `0 open ·
2 tasks` instead of passing for one with work left; a project with missed
deadlines adds them in the overdue color, `3 open · 2 overdue · 5 tasks`,
counted as the Outliner's project headers count them. Parked projects are dimmed
under their own Parked Projects heading and drop the `#parked` chip it would
repeat. Administering a project is one press:
`Enter` focuses it in the same Outliner, `a` creates one, `e` edits the one
under the cursor, `*` toggles its pin, `#` sets its tags, and `Ctrl+N` opens its
Notes. `Ctrl+T` cycles between every project, only those holding open tasks, and
only those holding none open. The catalog ID, not a display name, protects the
system Unfiled project. The preview beside the list lists the project's open
tasks and folds finished ones into one `✓ N done` row. A row `v` selects wears
`✓` where the cursor arrow sits; nothing is reserved for a checkbox.

### Find a project

`/` searches a project by anything the project itself says: its name, its slug,
its description, and each of its tags spelled either way, so `#billing` finds
what the row's own chip reads and `billing` finds it too. It never searches the
work inside a project — a task title, description, tag, context, or comment is
the Outliner's `/`, not this one — so a row appearing here always names the
words you typed. The pinned Hub and the archived tail answer the same
vocabulary.

### Preview project work

On a terminal wide enough for two panes, the right one belongs to the project
under the cursor. It opens with the project's name and a single muted line —
`Active · 4 open · 4 tasks · slug docket` (`Parked` or `Archived` for those), with
` · pinned` appended only when the project is pinned — followed by up to two
lines of its description. The rest of the pane is the project's own tasks, every
one of them, in tree order with the same connectors, badges, and indicators the
Outliner draws, because they are the Outliner's rows. A pane too narrow for a
whole title sheds that row's reference count and review first, in the Outliner's
own order, but never its deadline: an overdue date is what a glance at a project
is for, so the title is cut before the date is dropped. `Ctrl+F` and `Ctrl+B` page that list a pane at a time, `↑ N
more` and `↓ N more` count what sits off either end, and moving the list cursor
swaps the tasks and returns to the top of the new project. A project holding
nothing reads `No tasks.`, and one whose tasks could not be read says `Tasks not
loaded.` rather than passing for the first. The list pane is no wider than its widest row
needs, up to its default 55% share, so short project names leave the preview
more room. `<` and `>` move the divider between
the two panes left and right, 5% of the width a press from where it is drawn,
between a quarter and three quarters of it; the position holds until Docket exits, so reopening Select
Project finds it where you left it. While you type a query they are just
characters. Below the two-pane minimum width the pane is gone and the paging and
split keys go quiet with it.

Those acts also reach a project from the Outliner itself: `Space` on a project
group header opens Project Actions, whose sheet holds Archive and Delete, and
`e` on that same header edits the project, so none of the three needs a trip
through Select Project. Archiving is one-way
from there — an Outliner header is only ever drawn for an active project — so
Unarchive stays here, where an archived project is visible.

Deleting a project is the one act with no second chance, and it asks first,
naming the cascade by name: the project's tasks, its notes, and its attachments
all go. The removal and the catalog and planning rewrites beside it are one
transaction, so `u` brings every byte back. The system Unfiled project cannot be
deleted, and an archived project still can be — archiving is a step toward
removal, never a state that traps a project in the catalog.

`/` opens project search and `Esc` leaves the query for the matches it found.
`v` selects the project under the cursor and `Ctrl+A` selects every project
matching the current search. With a selection, `Space` offers Add Tag, Remove
Tag, Park and Unpark across all of it, and the whole edit commits as one undo
entry. Without one, `Space` offers Archive on an active project and Unarchive on
an archived one — the two lifecycle actions no direct key reaches. `Esc` unwinds
one rung at a time: the action sheet, then the query, then the selection, then
the surface itself. `?` lists the grammar above rather than the Outliner's, and
inside the query it is an ordinary character to search for. `u` and `Ctrl+R` sit
on that same line: inside the query they are ordinary characters, and outside it
they undo and redo the workspace ledger, which is what makes the `u undo` the
strip offers after a park or a bulk tag answerable without leaving the Hub.


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

A parked project is not hidden away. It still appears in Select Project, under
its own Parked Projects heading, still
owns its Notes, and still accepts `%project` during capture and inline editing.
`Enter` on it in Select Project opens it as an
explicit one-project focus, anchored with the `parked` tag, and `Esc` returns
to the Active base with nothing left behind.

Archiving is separate. It preserves a project's tags and restoring returns
them unchanged.


### All Notes

When browsing a single project, its notes appear in a separate **Notes** section
below the task lanes, newest first, even when the project has no tasks. Fold or
zoom the section like a task lane, and press `Enter` on a note to open it in
Notes. Task filters, subtree focus, lenses, and search hide this browsing section;
ranked search retains its matching-note results. Archived projects are excluded.

`Ctrl+N` opens All Notes, a workspace-wide manager that scans Global plus every
unarchived project, including the protected Unfiled project and every parked
project. On a project header row it opens that project's notes instead. Rows
are ordered by most recent
modification and carry an owner label. `p` opens a searchable All, Global, or
single-project scope picker, while `/` applies a persistent title/body search
inside the selected scope.

`a` always asks for a Global or unarchived-project
destination before starting the editor, `e` renames the note under the cursor,
and `%` moves the selected note to
another such owner as one atomic workspace transaction. `Enter` opens and edits the selected
note in
`DOCKET_EDITOR`, then `VISUAL`, then `EDITOR`; if none is set, Docket reports how to
configure one. `y` and `Y` copy that note's title and its full file path, and
because a global note and a project note live in different directories the
copied path says which collection it came from without a word of context.
`Space` opens the note action sheet, which holds Delete alone and
confirms before deleting; deletion has no direct key. Notes has no multi-select,
so that sheet always acts on the one note under the cursor. Notes support
open/edit, create, rename, move, copy, delete, undo, and redo. Note tags, task
links, pins, bulk actions, and archived-project ownership remain excluded.

At 72 columns and wider, Notes shows the selected note beside the list in the
same split every other list surface uses: its title, owner, and modification
time above the note's rendered Markdown. Narrower terminals render the list
alone. A note with no body says so, and a note whose frontmatter failed to parse
shows the reason as a warning rather than a rendering of text nothing can
trust. `Ctrl+F` and `Ctrl+B` page that body with a row of overlap, and exact
`↑`/`↓` counts name what is above and below; both presses work while you are
still typing a search, and are offered only where they would move something,
never on an unparsed note's warning and never when the pane is off screen. A search that matched the body opens the pane around its first match
with one line of context above it, under the same case reading the result list
used; a title-only match, no query at all, and a match the rendered text cannot
be located in all open at the top. The pane is reading only: it follows the
cursor, re-anchors whenever the note, its bytes, the query, the scope, or the
layout changes, holds your manual paging while none of those move, and remembers
nothing between launches.

`T` toggles exactly two maintained machine-local themes: dark and light.

