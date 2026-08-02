# CLAUDE.md

Contributor guidance for this repository.

## Build and verification

```bash
just build
just test
just fmt
just vet
just tidy
./scripts/check_generated_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

Run a focused Go test by package, for example:

```bash
go test ./internal/tui -run TestFinalDirectGrammar
```

Go accepts packages, not individual test files, as test targets.
`cmd/cli-reference.md` is generated from Cobra help; change the command help and
run `./scripts/check_generated_docs.sh` instead of editing the generated file by
hand.

## Architecture

`tt` is an Outliner-first local task manager. The Bubble Tea TUI is the product
surface; Cobra contains only the TUI launcher and maintenance commands.

- `internal/models` owns pure task, project, note, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, project-qualified target resolution, planning
  reconciliation, and atomic mutation policy.
- `internal/storage` owns strict schema-5 Markdown, attachments, transactions,
  conflicts, recovery, and the workspace undo ledger.
- `internal/config` owns the lean catalog/planning domains, machine-local
  contexts/theme, workspace selection, and v4-to-v5 migration.
- `internal/tui` owns the sole Outliner root, typed subordinate tools and
  overlays, the operation registry/executor, and deterministic scenarios.
- `internal/textgrammar` owns canonical capture/edit sigils.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to narrowly audited maintenance, migration, restore, and transaction
infrastructure.

## Canonical v5 task contract

The live states are:

```text
inbox, ready, next, in_progress, waiting,
paused, someday, done, canceled
```

Review and every other v4-only state or field may appear only in explicitly
named migration DTOs and audit code. Ordinary task Markdown uses schema 5,
generic `Task` headings, a project-local ShortID, and the retained v5 fields.

Planning invariants:

- Next membership is an explicit project-qualified planning collection, never a lifecycle state.
- Today is project-qualified and mutually exclusive with Next.
- Backlog removes Today/Next references.
- Starting sets Doing; completing sets Done and clears planning.
- Dependency blocking is derived.
- Done and Canceled reject a direct nonterminal child.

Parents are project-local with maximum depth six. Cross-project dependencies
must remain project-qualified, resolvable, and acyclic.

## TUI grammar and operations

The AppModel owns exactly one scope-aware Outliner. Every launch starts All
Projects, All, with no restored navigation, search, filters, or selection.

`OperationRegistry` is the descriptor authority and `OperationExecutor` is the
single invocation pipeline. `SimplifiedInputStrategy` drives direct dispatch,
handled keys, footer hints, chord hints, `?` help, and scenario discovery.
Register an operation once, then let those projections consume it. Do not add a
parallel key router, palette, or view-specific copy.

The direct grammar is:

```text
Tab / Shift+Tab                 six modes
j/k, arrows, h/l, S, Home/End  navigation
a/A, e/E                       capture/edit
si sb sn st sg sw sp sm sd sc  state/planning chord
!0-5 @ # z P                   metadata
< > J K                        structure
y Y Ctrl+Y                     clipboard
v V Space                      selection/actions
c N T / ? u Ctrl+R Esc Ctrl+C  global
```

Pressing `s` shows the state/planning chord. Its second keys are `i` Inbox, `b`
Backlog, `n` Next, `t` Today, `g` Doing, `w` Waiting, `p` Paused, `m`
Someday, `d` Done, and `c` Canceled. Next and Today retain their toggle
behavior, and Backlog removes both planning memberships.

Filter chords are `fp0-5`, `fs`, `ft`, `fd`, `fb`, `fz`, and `fx`. `fd` opens
the due-date facet picker for overdue, today, this week, or no due date, and
`fx` clears every facet. The old standalone `b`, `d`, `n`, and `t` actions and
comma state prefix must remain unbound, along with `m`, `:`, `R`, `p`, and
`Ctrl+Esc`.

The `Space` menu is exactly:

1. Move Project
2. Set Parent
3. Checklist
4. Comments
5. Attachments
6. Delete

An operation is direct-only or menu-only, never both. Delete has no direct key
and confirms before mutation.

The Outliner stays full-width at every supported size. `Enter` toggles a
session-only, five-line Peek beneath the cursor for description, tags,
checklist items, and named dependencies. Rows carry
operational metadata directly and omit short IDs. Entering visual selection
closes Peek; while selection exists, structural, edit, Peek, relation, and
clipboard operations must reject it instead of falling back to the cursor.
The bulk `Space` menu contains Delete only.

Scope Hub owns All Projects and exclusive regular project-label contexts;
`personal` is always present and protects its name while keeping membership
editable. `C` opens the search-first
project selector for project focus and lean project administration. `N` always
opens All Notes across Global and every project returned by
`Runtime.ActiveProjects()`. Inside Notes, `v` chooses All, Global, or one active
project, `s` persistently searches titles and bodies, and `m` atomically moves
one note between active owners. Creation with `n` or its `a` alias always asks
for a destination; the remaining mutation grammar is open/edit, rename,
delete, undo, and redo. Keep
note identity location-qualified, keep archived projects and `Ctrl+N` excluded,
and do not restore note tags, task links, pins, bulk actions, or previews. `T`
toggles only dark/light.

## Capture grammar

Canonical capture sigils are:

```text
!1-3  #tag  @date  $destination  %project  >parent
```

`$destination` accepts Inbox, Backlog, Today, Next, Doing, Waiting, Paused,
Someday, Done, or Canceled. `#tag` creates valid tags on the fly. `^type`,
`~recurrence`, priorities 4–5, `$ready`, and `$in_progress` must remain
recognized errors with replacement guidance. Do not reintroduce aliases such as
`priority:`, `due:`, `state:`, or `proj:`. Multiline capture validates every
nonblank line before committing one transaction. Resolve the target project
before the project-local parent; omitted destinations default roots to Inbox and
children to Backlog.

When changing the grammar, update parser definitions, completion, syntax guide,
scenarios, and parity tests together.

## Durable storage and migration

Portable live state consists of:

```text
.tt/workspace.json
.tt/planning.json
.tt/global-notes/
.tt/transactions/
.tt/undo/
.tt/conflicts/
.tt/backups/
projects/<slug>/tasks.md
projects/<slug>/notes/
projects/<slug>/.attachments/
```

Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only custom
contexts and the dark/light theme. Workspace paths are derived below approved
roots; preserve symlink-aware containment, checked revisions, deterministic
locks, and recovery manifests.

The supported v4 migration validates the complete candidate first, publishes an
immutable pre-simplification archive/audit pair, then atomically activates
schema 5 and removes retired storage. v4 decoders are migration-only. Do not let
ordinary runtime code import them or publish partial v5 output before the
snapshot boundary.

Undo/redo is one 50-entry workspace ledger covering task, project, note, and
attachment transactions. A successful forward mutation clears redo; failure
records nothing.

## Documentation and scenarios

User-facing docs must describe only the eight-state, six-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in clearly
identified migration sections.

Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product
contract replayed by `./scripts/run_tui_scenarios.sh`. Prefer public keys for
behavior and harness operations only for setup that has no public shortcut.
Include `no_layout_issues` for important narrow and resized states. The full
scenario gate also runs focused snapshot-first migration and interrupted
activation recovery tests.
