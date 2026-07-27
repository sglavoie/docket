# CLAUDE.md

This file gives Claude Code contributor guidance for this repository.

## Planning

- Keep implementation plans concise.
- End plans with unresolved questions, if any.
- Preserve unrelated work in the working tree.

## Build and development

```bash
just build          # Build ./tt
just install        # Install to ~/.local/bin
just test           # Run all Go tests
just lint           # Run formatting and vet checks
just fmt
just vet
just tidy
just ci             # Full release gate
just tui-scenarios  # Replay committed TUI scenarios
```

Run a package test with `go test ./internal/models` or a focused test with
`go test ./internal/tui -run TestName`. Go accepts packages, not individual test
files, as test targets.

Generated documentation is checked by:

```bash
./scripts/check_generated_docs.sh
```

Do not hand-edit `cmd/cli-reference.md` without changing the Cobra help that
generates it.

## Architecture

`tt` is a local-first task manager with a Cobra CLI and a Bubble Tea TUI. Both
presentation layers call the application layer; neither is a second domain
implementation.

### Packages

- `cmd/` contains CLI adapters, argument resolution, output formatting, and
  Cobra help.
- `internal/models/` contains canonical domain values and pure task/project
  behavior.
- `internal/app/` contains use cases, workflow orchestration, planning
  reconciliation, and mutation policies.
- `internal/storage/` parses and renders task Markdown and implements
  revision-checked workspace transactions, merge, recovery, conflict, archive,
  and undo storage.
- `internal/config/` owns the split workspace domains and machine-local runtime
  state.
- `internal/tui/` contains the Outliner, typed routes and overlays, operation
  registry/executor, shared list surfaces, and secondary tools.
- `internal/workflow/`, `internal/template/`, and `internal/textgrammar/` own
  automation, task templates, and shared capture/search grammar.

### Mutation flow

A user mutation resolves its targets from one workspace snapshot, validates in
`internal/app`, produces changes, and commits through the workspace repository.
The repository checks revisions and paths, acquires locks deterministically,
stages a manifest, atomically publishes all artifacts, and records one undo
entry. Cross-project changes are one transaction; never reproduce this behavior
with independent `SaveProject` calls.

CLI commands should call the existing application service and save/commit seams.
TUI operations should use the appropriate coordinator or executor mutation
binding. Direct storage writes are reserved for maintenance, migration, restore,
or narrowly audited infrastructure.

## Canonical task lifecycle

`models.WorkflowState` is the only runtime task lifecycle value:

```text
inbox, ready, next, in_progress, waiting,
paused, review, someday, done, canceled
```

Use `Task.State()` and the canonical state transition helpers. Do not add
`TaskStatus`, section, stage, or queued lifecycle fields back to runtime models,
application requests, JSON, configuration, templates, search, or workflows.
Schema-v1 task Markdown remains readable only through private migration decoding;
all ordinary writes are canonical schema v2 with one `state` field.

The word `status` remains valid for unrelated concepts such as a project status
icon, a blocker record's resolved status, process status, and HTTP/command
status. Do not conflate those with task lifecycle.

### Planning invariants

- Next membership is exactly `StateNext`. Focus Queue is an optional ordering
  index and never grants membership.
- Today is a project-qualified planning list containing nonterminal, non-Inbox,
  non-Next tasks.
- Today and Next are mutually exclusive.
- Backlog sets Ready and clears Today/Next planning.
- Adding Today to Inbox or Next promotes the task to Ready first.
- Starting sets In Progress; completing sets Done and clears planning.
- Reopening Done or Canceled returns the task to Inbox.
- Dependency blocking is derived from unresolved dependencies.

Use the application planning reconciliation seam whenever state and planning
membership can change together.

## Public lifecycle contract

- Creation and template commands use `--state`.
- List, archive, and sort surfaces use state.
- Task JSON emits only `state`; project JSON does not emit workflow sections or
  section colors.
- Capture, edit, and search use `$ready`-style canonical tokens.
- Saved filters persist `filter_states`; templates persist `state`.
- Workflow rules use state fields, state sets, state transitions, and
  `set_state`.
- Structured AI actions use plan schema v2; `create_task.state` is optional and
  defaults to Inbox.

Retired flags and commands should fail with a concise canonical replacement;
they must not be silently accepted as aliases.

## TUI architecture

The TUI has one AppModel-owned, scope-aware Outliner. It always starts in All
over All Projects. A project focus is a scope inside that Outliner, not another
root model.

Typed navigation is authoritative:

- Outliner root
- at most one Task Detail or Secondary Tool child
- typed overlays, with operation sheet → value picker as the maximum depth

`Ctrl+Esc` closes children and overlays, clears transient filters, search, and
selection, and restores All over All Projects.

### Operations

`OperationRegistry` is the descriptor authority for direct keys, action sheets,
Finder actions, bulk actions, help, and harness discovery.
`OperationExecutor` is the single invocation pipeline. Add an operation once,
give it stable identity, applicability, target cardinality, risk, and optional
value-provider metadata, then bind execution separately. Do not add parallel
command catalogs, prefix-key routers, or view-specific action copies.

Direct Outliner keys are:

```text
a capture   e edit   s start   d done
b Backlog   t Today  n Next    Space actions
/ search    : Finder m modes   c Scope Hub  v multi-select
N Notes     R Weekly Review    T Tags        ! Notifications
u undo      Ctrl+R redo        Esc back      Ctrl+Esc reset
? help      Ctrl+C quit
```

In multi-select, `Space` toggles selection and `Enter` opens bulk actions.
Delete exists only in an action sheet and requires confirmation.

### List surfaces and routes

Use `ListSurface` for generic fuzzy lists, categories, viewport behavior,
preview, and single/multi-selection. Stable item IDs must be non-empty and
project-qualified where task IDs can collide. Keep specialized date, calendar,
form, and domain controls only where their behavior is not a generic picker.

Typed overlays must prevent hidden Outliner input and reload dispatch.
Scenario-only deep links require `CapabilityHarness`; they must not leak into
production help or Finder.

### Scope and promoted tools

`c` opens the Scope Hub for All Projects, regular contexts, custom contexts, and
single projects. Scope is session-owned. Notes uses project notes in project
focus and global scratch notes otherwise. Weekly Review and Tags receive the
active scope's project set. Notifications remains global.

## Capture and search grammar

`internal/textgrammar` is the source for canonical capture and search tokens.
Task Composer and visible search help must derive lifecycle names from it.
Authoring accepts `$state`/`state:<value>`, not retired task status or section
forms. Capture defaults to Inbox and routes to the System Inbox at base scope or
the focused project in project scope.

When changing token definitions, update parser, completion, live preview, syntax
guide, tutor, and parity tests together.

## Durable storage

The portable data directory contains:

```text
.tt/workspace.json
.tt/planning.json
.tt/settings.json
.tt/automation.json
.tt/transactions/
.tt/undo/
.tt/conflicts/
.tt/trash/
projects/<slug>/tasks.md
```

Machine-local state is keyed by workspace ID below
`~/.tt/runtime/<workspace-id>/machine.json`; logs and process locks are also
machine-local. New workspaces start at local `~/.tt/`. iCloud and other data
directories require explicit selection.

Project paths must remain derived below the approved data root. Preserve opaque
schema-v2 Markdown blocks. Do not bypass path validation, transaction locks,
revision checks, or recovery manifests.

## Undo, conflicts, and AI

Undo/redo is a linear workspace ledger. A new overlapping mutation clears redo.
Grouped restores preflight every participant and abort instead of applying a
partial restore.

Concurrent project changes use canonical revisions and field-level three-way
merge. Conflicting edits produce durable conflict bundles; resolution rechecks
the remote revision.

AI provider output is untrusted. Prompts travel over standard input. Structured
plans are parsed as v2, resolved against one snapshot, risk-classified, previewed,
confirmed, and applied once with read preconditions. Never turn provider text
into shell execution or rebuild an executable plan from a detached preview.

## Documentation and scenario contracts

User-facing documentation must describe only the final ten-mode daily loop.
Migration documentation may name retired fields only when it clearly describes
decode-only historical input.

Committed TUI scenarios should cover final user behavior: daily loop, exact mode
catalog, action sheets, bulk selection, Scope Hub, Inbox auto-advance, Finder
shortlist/search, promoted tools, and narrow layouts. Do not preserve obsolete
surfaces solely for scenario setup; use harness-only operations when a deep link
is necessary.
