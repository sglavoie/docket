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
2. `Tab` moves to Inbox for triage.
3. `s` opens the shared workflow chord.
4. `sb`, `sn`, `st`, and `sr` organize work; `sg` starts it and `sd` completes it.
5. Direct metadata keys handle priority, state, due date, tags, snooze, and pin.
6. `Space` opens the six occasional task tools.

Together, these operations form one capture, triage, plan, execute, and
complete loop.

## Presentation grammar

Every full-screen presentation keeps one operational title row. Its left side
names the surface, scope, and mode once; active selection, search, filters,
zoom, or editor context occupies the middle; and the right side reports the
workspace pulse as `workspace: inbox 3 · today 0 · overdue 1`. Today is shown
even at zero whenever the pulse region fits. Zero Inbox and overdue counts
disappear, and narrow terminals drop Inbox, then overdue, then Today before
shortening required interaction context.

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, expanded detail, and pending
chords may use two. Required commit and cancellation controls survive before
optional hints. Transient pickers and action sheets own their local guidance
while leaving the host title and workspace pulse visible.

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Ctrl+I` returns to the
relocated task.

`Tab` cycles exactly All → Inbox → Today → Next → Backlog → Due → Recent, and
`Shift+Tab` reverses that cycle. The `g` chord jumps straight to one mode:
`ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `gd` Due, `gr` Recent.

All mode shows open, unsnoozed work in this order:

```text
Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday
```

Pinned tasks appear once in project groups, with parent breadcrumbs. Groups
sort by their newest pin. Tasks inside a group start newest-first and `J`/`K`
reorder them manually; a pinned subtree ranks by its own root's pin, and `J`/`K`
never move a task across a project group. All,
Inbox, and Backlog preserve hierarchy and manual sibling order. Today and Next
preserve workspace-wide planning order. Review is the sparse set of open tasks
tagged `review`; pinned tasks still appear only in Pinned. Due is date ordered. Done, Canceled,
and snoozed work stay hidden unless a filter explicitly asks for them.

Recent answers "what did I touch lately". It lists every task updated in the
last 30 days newest-first under Today, Yesterday, Last 7 Days, and Last 30 Days,
and includes Done, Canceled, and snoozed work because all of it is activity. The
order freezes when you enter the mode, so triaging a row from there leaves it
exactly where you are reading it; `gr` re-enters and re-reads activity.

## Outliner grammar

| Area | Keys |
| --- | --- |
| Modes | `Tab` / `Shift+Tab` cycle the seven modes; `g` opens the go-to chord: `ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `gd` Due, `gr` Recent |
| Navigation | `j`/`k` or arrows move; `h`/`l` or left/right collapse/expand; `S` toggles all visible sections; `gg`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `Ctrl+O`/`Ctrl+I` jump backward/forward after a task relocation; `Enter` inspects |
| Capture and edit | `a` capture, `A` child, `e` title, `E` description, `Ctrl+E` external editor |
| Workflow | `s` opens the workflow chord: `si` Inbox, `sb` Backlog, `sn` toggle Next, `st` toggle Today, `sg` Doing, `sr` toggle Review, `sw` Waiting, `sp` Paused, `sm` Someday, `sd` Done, `sc` Canceled |
| Metadata | `!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date; `#` tags; `z` snooze; `P` pin |
| Structure | `<` outdent, `>` indent, `J`/`K` reorder |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both |
| Selection | `v` toggles the cursor task; `V` starts/freezes an inclusive visible-task range; `Ctrl+A` replaces it with every visible task |
| Global | `p` Select Project, `N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit |

Inside the `@` and `z` pickers, `a` opens a typed-date input for any date the
relative presets do not cover: `2026-01-15`, `tomorrow`, `+3d`, `+1w`, `fri`,
`next mon`, or `dec 15`. Inside the `#` picker the same key adds a new tag.

The operation descriptors are the single source for dispatch, contextual
guidance, chord hints, and `?` help. The old standalone `b`, `d`, `n`, and `t`
actions and comma state prefix have no compatibility action; neither do removed
keys such as `m`, `:`, `R`, `p`, and `Ctrl+Esc`.

Task updates that move the focused task—such as `sg` moving it into In
Progress—add a session-only jump entry. `Ctrl+O` returns to the task that
followed its old position (or the preceding/nearest surviving row), and
`Ctrl+I` returns to the relocated task. Changing mode, scope, search, filters,
sorting, folds, or zoom clears this history. Terminals that do not report
enhanced keyboard events encode `Ctrl+I` as `Tab`, so mode cycling remains the
fallback there.

`#` opens Set Tags for the current task or visual selection. Inside the
overlay, `Space` toggles tags, `a` creates and selects a new tag, and `Enter`
applies the pending changes.

### Task actions

`Space` contains exactly:

1. Move Project
2. Set Parent
3. Checklist
4. Comments
5. Attachments
6. Delete

The list filters as soon as printable text is typed. Frequent direct operations
do not repeat in this menu. Delete always asks for confirmation.

### Filters and search

Filters are session-only refinements of ordinary mode rows. Slash search is a
temporary projection over every nonterminal task in the active project scope,
including snoozed work; Esc restores the prior mode, facets, cursor,
scroll, and folds exactly.

| Keys | Effect |
| --- | --- |
| `fp0`, `fp1`–`fp3` | Clear priority filtering or keep High, Medium-or-higher, or Low-or-higher assigned priorities; Unset never matches |
| `fs` | Pick workflow states with OR semantics, including terminal states |
| `fc` | Pick completed at least 7, 30, or 90 local-calendar days ago, or all valid completion ages; also filters states to Done and Canceled |
| `ft` | Pick tags with AND semantics |
| `fr` | Toggle the `review` tag facet |
| `fd` | Open the due-date facet picker: overdue, today, this week, or no due date |
| `fz` | Snoozed-only |
| `fx` | Clear every facet |
| `/` | Ranked title, description, tag, and project search over every scoped nonterminal task |

The lowercase `f` chord refines the rows a projection already holds. Uppercase
`F` decides which projects load at all, so each of its facets reloads:

| Keys | Effect |
| --- | --- |
| `Ft` | Pick project tags with AND semantics, plus an `Untagged` entry for projects carrying none |
| `Fp` | Show or hide parked projects |
| `Fa` | Show or hide archived projects |
| `Fx` | Clear every project filter |

`Fp` and selecting `parked` inside `Ft` are the same switch, so the two can
never disagree about which population is loaded. Inside `Ft`, `v` toggles a tag
and `Space` over a tag row offers Rename everywhere, Remove everywhere, Park all
members, and Unpark all members across every project carrying it, as one undo
entry. Removing a tag that is a project's only tag asks where those projects
should land first.

Pressing a chord prefix shows its valid continuations in the guidance region. `Esc`
cancels that unfinished chord, and a second key the chord does not offer says so
instead of doing nothing.

For intentional cleanup, use `fc` to choose an age — it filters states to Done
and Canceled for you — then `Ctrl+A` to select the visible matches, and
`Space` → Delete. Use `fs` afterwards to narrow to Done or Canceled alone.
The confirmation freezes root, descendant, visible/hidden, and attachment counts;
the entire deletion is one undoable transaction.

### Peek and multi-select

The Outliner uses the full terminal width at every supported size. Task rows
show workflow state, conditional high priority, due and snooze dates, and
checklist progress; short task IDs are hidden.

`Enter` toggles a session-only Peek directly beneath the cursor task. Peek
follows navigation and shows at most five lines of additive detail:
description, tags, attachment names, comment counts, and checklist items. `Esc`
closes Peek before unwinding Outliner navigation.

State, Today/Next/Review, priority, due, tags, snooze, and pin actions apply atomically
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
!1-3  #tag  @date  $destination  %project  >parent
```

- `!1` through `!3` set High, Medium, or Low priority.
- `#tag` creates and sets a lowercase ASCII kebab tag on the fly. Input is
  canonicalized (`#Needs / Review` becomes `needs-review`); `#review` feeds the
  Review lane.
- `@date` accepts the supported date expressions and stores a local calendar
  date.
- `$destination` accepts `inbox`, `backlog`, `today`, `next`, `doing`,
  `waiting`, `paused`, `someday`, `done`, or `canceled`. Today and Next create
  Backlog tasks with an explicit planning membership.
- `%project` overrides scope routing.
- `>parent` resolves inside the selected project after project routing.

`^type`, `~recurrence`, priorities `!4`–`!5`, `$ready`, and `$in_progress` are
rejected with replacement guidance. Other aliases such as `priority:`, `due:`,
`state:`, and `proj:` are literal title text, not task attributes. Root capture
defaults to Inbox. A new child defaults to Backlog under its selected parent.

Exact case-insensitive `bug`, `epic`, `story`, `feature`, and `idea` tags color
task titles in that precedence order. Namespaced values such as `type:bug` and
partial matches remain ordinary tags and add no prefix or marker icon. Both tag
pickers group those five under a `SEMANTIC` section above the rest: `#` Set Tags
always offers all five with a one-line summary each, whether or not any task
carries them, while `ft` Filter by Tags stays usage-derived and lists only the
ones in use.

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
owns its Notes, still accepts `%project` during capture, and is still a valid
Move Project destination. `Enter` on it in Select Project opens it as an
explicit one-project focus, anchored with the `parked` tag, and `Esc` returns
to the Active base with nothing left behind.

Archiving is separate. It preserves a project's tags and restoring returns
them unchanged.

`N` opens All Notes, a workspace-wide manager that scans Global plus every
unarchived project, including the protected Unfiled project and every parked
project. Rows are ordered by most recent
modification and carry an owner label. `v` opens a searchable All, Global, or
single-project scope picker, while `s` applies a persistent title/body search
inside the selected scope.

`n` (or its `a` alias) always asks for a Global or unarchived-project
destination before starting the editor, and `m` moves the selected note to
another such owner as one atomic workspace transaction. `Enter` opens and edits the selected
note in
`TT_EDITOR`, then `VISUAL`, then `EDITOR`; if none is set, `tt` reports how to
configure one. Notes support open/edit, create, rename, move, delete, undo, and
redo. Note tags, task links, pins, bulk actions, previews, archived-project
ownership, and a `Ctrl+N` alias remain excluded.

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

## Schema 6 storage and migration

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

Project task files use strict schema-7 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, project tags, and the
protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice.

Startup migrates supported schema-7 workspaces to schema 8, supported schema-6
workspaces through schema 7 and then schema 8, and supported schema-4/5
workspaces through the whole chain. Each step validates the complete candidate
before mutation and publishes its own immutable rollback archive and audit
first: `pre-simplification-v5-<UTC>.tar.gz` for v4→v5,
`pre-consolidation-v6-<UTC>.tar.gz` for v5→v6, `pre-scope-v7-<UTC>.tar.gz`
for v6→v7, and `pre-tag-v8-<UTC>.tar.gz` for v7→v8. The schema-6 conversion preserves notes and attachment bytes,
converts retained decision text into comments, repairs planning references,
renames the protected project to Unfiled, and deliberately resets the
incompatible pre-v6 undo ledger. The schema-7 conversion folds the retired
per-project grouping fields and the retired workspace scope table into ordinary
project tags, keeping `parked` as the sole behavioral project tag.
Schema 8 canonicalizes all task and project tags to lowercase ASCII kebab form,
audits renames and collisions, upgrades task Markdown to schema 7, and clears
undo/redo history whose retained bytes predate that canonicalization.

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent. To downgrade or retry from the old
format, restore the complete matching archive into a separate workspace and use
a compatible build; migration snapshots themselves are never modified.

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

Global flags are `--data-dir` and `--no-color`. See
[`cmd/cli-reference.md`](cmd/cli-reference.md) for generated syntax.

## Development

```bash
just test
./scripts/check_generated_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario gate replays the committed final-product TUI workflows plus the
snapshot-first migration and interrupted-activation recovery scenarios. See
[docs/tui-testing/README.md](docs/tui-testing/README.md).

For data boundaries, transaction mechanics, migration details, and platform
support, read [docs/architecture.md](docs/architecture.md).
