# Architecture, data, privacy, and recovery

This document describes the release-candidate architecture of `tt`. It is the
operator reference for the workflow-state model, on-disk layout, transaction and
conflict behavior, cloud and AI consent, upgrades, backups, and supported
platforms. CLI syntax remains authoritative in `tt --help`; TUI interaction is
documented in the in-app Reference and contextual help.

## Workflow state model

A task has one canonical `WorkflowState`. The persisted `state:` value in
schema-v2 task Markdown is one of:

| State | Meaning |
| --- | --- |
| `inbox` | Captured but not triaged. |
| `ready` | Available work with no stronger scheduling signal. |
| `next` | A deliberately selected next action. |
| `in_progress` | Work currently underway. |
| `waiting` | Waiting on an external event or person. |
| `paused` | Voluntarily suspended work. |
| `review` | Work awaiting review or validation. |
| `someday` | Deferred without a current commitment. |
| `done` | Completed terminal work. |
| `canceled` | Canceled terminal work. |

Any nonterminal state may move to another nonterminal state. `done` and
`canceled` are terminal; `reopen` returns either terminal state to `inbox`.
`CompletedAt` is populated only for terminal tasks.

Dependency blocking is derived from unresolved dependencies and appears as a
badge/filter. It is not a workflow state. The Active section identifies work
currently being worked on. Next (the on-deck shortlist) is not a
separate membership collection: a task is in Next exactly when its workflow
state is `next` (`StateNext`). The persisted Focus Queue list is ordering-only
— it orders the StateNext tasks (the next tasks in the pipeline) and never
confers membership, so unindexed StateNext tasks still belong to Next and
stale indexed non-Next tasks do not.
Daily Plan (Today) is a separate, config-backed planning collection. Next and
Today membership are mutually exclusive. Moving a task from Next to Today
atomically transitions it from `StateNext` to `StateReady` and adds it to Daily
Plan. Entering Next atomically removes any Today membership.

Use `tt task state <ref> <state>` for the complete state surface. Convenience
commands include `ready`, `next`, `start`, `wait`, `pause`, `review`, `someday`,
`complete`, `cancel`, and `reopen`.

## Runtime and application layers

Both the CLI and TUI invoke the presentation-neutral application service. A
mutation resolves targets from one workspace snapshot, validates domain rules,
builds one or more `ChangeSet` values, and crosses the same repository commit
boundary. Views and commands do not write project Markdown directly.

Workspace snapshots retain parsed projects, canonical revisions, task lookup,
qualified references, reverse dependencies, and partial-load diagnostics.
Long-lived global views reuse the snapshot loader: a changed project is parsed
once for a rebuild and unchanged indexed projects are not reparsed. Hierarchy
and Markdown traversal use lookup and child-adjacency indexes rather than
repeated full-project scans.

AI mutations use an opaque prepared action plan from preview through apply. The
prepared value defensively retains the exact snapshot, resolved actions, original
provider indexes, and accepted risk tiers shown to the user; callers cannot rebuild
an executable plan from a detached preview. Apply is single-use and adds read-only
revision preconditions for every project whose state influenced the accepted plan,
including transitive dependency reads and delete cleanup. Any relevant drift expires
the plan atomically and requires a fresh response, preview, and confirmation.
Unrelated project drift does not invalidate the plan.

## Durable data layout

Markdown remains canonical for project tasks. A workspace rooted at `<data>`
uses this layout:

```text
<data>/
├── .tt/
│   ├── workspace.json      # schema version, workspace ID, project catalog, Inbox ID
│   ├── planning.json       # Focus Queue, Daily Plan, reviews, filters, saved views
│   ├── settings.json       # portable behavior, appearance, privacy, and AI consent
│   ├── automation.json     # workflows, templates, and scoring configuration
│   ├── transactions/       # prepared/recovering transaction manifests and staging
│   ├── undo/               # workspace transaction ledger and retained undo artifacts
│   ├── conflicts/          # base/local/remote conflict bundles
│   └── trash/              # versioned project tombstones
└── projects/
    └── <safe-slug>/
        ├── tasks.md        # canonical schema-v2 task Markdown
        ├── completed.md    # archived completed tasks (plus completed.N.md rotations)
        ├── .completed.index.json # rebuildable archive summary index
        ├── notes/
        └── .attachments/
```

Project paths are derived from the data directory and validated catalog slugs;
absolute project paths are not durable workspace configuration. Ordinary slugs must
already be canonical lowercase ASCII components, reserved names are rejected, and
`_sandbox` is accepted only for the catalog's sandbox project. Catalog load and
transactional encode both enforce this invariant. The storage boundary validates
the project directory plus its task, backup, and lock paths against approved roots,
resolving existing symlinks before it creates transaction or lock artifacts.
Project creation is create-only: an uncataloged existing `tasks.md` aborts creation
without changing either the file or catalog. Unknown schema-v2 header blocks are
preserved when task Markdown is rewritten.

Startup classifies the synced domain bundle before recovery or bootstrap. The
precedence is: a complete split bundle loads; a partial bundle fails for repair; a
legacy-only aggregate `.tt.json` returns a typed migration-required error; and only
a truly empty workspace bootstraps defaults. A stale aggregate beside a complete or
partial split bundle is ignored. Legacy-only rejection happens before transaction
directories, locks, domain files, or machine files can be created. This release does
not automatically convert the aggregate format: migration must explicitly choose
the authoritative source of truth.

Machine-specific state is intentionally separate:

```text
every workspace:  ~/.tt/runtime/<workspace-id>/machine.json
```

`<workspace-id>` is the canonical UUID stored in synced `workspace.json`. The
file contains editors, integration choices, current project
selection, custom contexts, and bounded recent task/palette items. Renderer
layout, filters, cursors, folds, and composer expansion are session-owned and
are not serialized. The Outliner's Today mode owns its project-qualified subtree
overrides and global tree-depth limit for the session, and a depth change clears
per-node overrides. A workspace with a
different ID never shares this state. On first
startup after upgrading from the former layout, `tt` checks the machine file
keyed by the current data-directory path, then the historical
`~/.tt/machine.json`; it imports only an empty-ID or catalog-matching envelope,
writes it atomically to the canonical path, and retains the source for rollback.
An identity mismatch is an explicit error. Once canonical state exists, legacy
copies are ignored.

Logs and process locks are also machine-local and never synced, but unlike
`machine.json` they remain keyed by the current data-directory path. This keeps
two separately opened paths isolated even if their durable contents claim the
same workspace identity.

A workspace-location move first acquires the machine-local TUI process locks for
both source and destination in deterministic order, so an active TUI at either path
blocks relocation. It then acquires and recovers both workspace commit locks before
inventory. The locked inventory is hash-bound: source drift, a divergent destination,
or a destination race aborts without switching the active-workspace pointer, and
publication never replaces a competing file. `--force` skips only the confirmation
prompt and never bypasses process locks, commit locks, or snapshot validation.

After copying and validating the portable workspace, the move binds and migrates
machine state while the source path is still known, rechecks source and destination,
then switches the machine-local active-workspace pointer as the final commit. The
source remains as a backup. Because the catalog ID is preserved, supported
machine preferences remain available at the destination on the same machine. A
true copy assigned a new workspace ID receives a separate default machine-state
namespace.

## TUI navigation architecture

The Outliner is the sole primary root and launches globally in Today on every
run. Typed navigation is an Outliner root, Task Detail children, direct
secondary-tool children (Weekly Review, Metrics, Roadmap, Inbox, Notes,
operational Blockers, workflow/scoring tools, Templates, Tags, Notifications,
Settings, and other retained utilities), and reference/help surfaces. Modes and
project scopes belong to the Outliner rather than to routes: `Tab`/`Shift+Tab`
cycle Today → Next → Backlog → Active → All, `h`/`l` rotate workflow
perspectives, `P` opens a categorized searchable mode picker, and `Ctrl+X`
changes project scope. Saved-perspective IDs and saved filters are preserved and
normalized under the mode catalog; the workspace-wide due-date lens is a
Due Date mode rather than a separate route. Esc unwinds local state, task zoom,
project scope, then the active child screen; Ctrl+Esc resets to a clean global
Today.

A single Outliner model (`AppModel.crossOutliner`) and one presentation identity
(`ViewCrossProjectOutliner`) own both global and project-scoped work. The root
`TaskWorkspace` route is always app-scoped; project selection is represented only
by Outliner scope frames, never by a project-scoped root route. Project scope is
entered through `Ctrl+X` (the scope picker) and the project-header scope actions
(`Enter` on a header or its `Shift+Enter` admin menu), which all share the
Outliner's own `pushProjectScope` and preserve the active mode. The remaining
project entry points — command-palette project navigation, task deep links, and
project-aware commands — route through `AppModel.enterProjectScopeOutliner`,
which pushes or replaces a scope frame on that one instance rather than
constructing a separate `AppModel.outliner`. The retired `Ctrl+P` and `g o`
drill-downs no longer exist. Task creation and every retained project operation
run from the scoped Outliner. Esc pops to the previous scope
frame; Ctrl+Esc clears the scope stack back to global Today.

## Transactions, recovery, and undo

`WorkspaceRepository.Commit(ChangeSet)` is the sole user-mutation persistence
boundary. Each change carries the revision observed in its base snapshot. The
commit service:

1. validates all paths, revisions, artifacts, and attachment effects;
2. acquires locks in deterministic order;
3. writes a `prepared` transaction manifest and staged bytes;
4. advances through `committing` to `committed` while atomically replacing
   project and domain artifacts; and
5. records one workspace-ledger entry for undo.

Startup recovery rolls an interrupted prepared/committing transaction forward.
It does not expose a partially applied multi-project mutation. Do not manually
delete `.tt/transactions/` while recovery is pending.

`tt undo` and `tt redo` operate in the current project by default;
`tt undo --global` and `tt redo --global` select the newest workspace
transaction. A grouped transaction restores every affected project, domain
artifact, archive entry, index, and retained attachment together. A new
mutation clears overlapping redo entries. Undo depth controls retained history,
including attachment bytes.

Undo is convenience history, not a backup: retention is finite and lives in the
same workspace as the primary data.

## Concurrent edits and conflicts

Project revisions are SHA-256 hashes of canonical schema-v2 content. If the
on-disk revision changed after a snapshot was loaded, `tt` performs a field-level
three-way merge using the retained base, the local intent, and current disk
content.

Read-only project preconditions are stricter than ordinary write revisions. They
guard project state consulted by a prepared AI plan under the same deterministic
project-lock set, but never become writes, conflicts, undo images, or journal
operations. A missing or changed guarded project stops the transaction before
three-way merge, conflict recording, or durable staging. Ordinary non-AI mutations
retain the merge behavior below.

Independent task additions, different-field edits, identical edits, and
delete-versus-unchanged changes merge automatically. The transaction stops and
records a conflict for delete-versus-edit, different edits to the same field,
incompatible hierarchy/order edits, or concurrent changes to the same opaque
Markdown block. No project or artifact in the `ChangeSet` is partially
committed.

Use `tt conflict list`, `tt conflict show`, and `tt conflict resolve`. Resolution
rechecks the current remote revision before committing, so an older resolution
cannot overwrite a newer edit.

## Workspace location and cloud consent

New workspaces start in local storage at `~/.tt/`. `--data-dir` and
`TT_DATA_DIR` select another workspace explicitly. iCloud is opt-in through the
workspace-location flow; selecting it stages a copy, validates the destination,
and switches the machine-local active-workspace pointer only after the copy is
complete. Existing installations that are already active in iCloud remain
there during upgrade unless the operator moves them.

Portable workspace files may sync only when the selected data directory is in a
cloud-synced location. `machine.json`, logs, and locks remain local. Review the
cloud provider's retention and encryption policy before opting in; `tt` does
not add end-to-end encryption around provider storage.

## AI privacy and consent

AI is optional. Ask AI supports the Claude, Codex, and GitHub Copilot CLIs. `tt`
sends task context only after an explicit disclosure and remembered consent
stored in portable `settings.json`. Consent is tracked per harness and scope
version. Full-workspace disclosure requires current provider-specific consent,
so a generic, unversioned, or superseded grant is not sufficient and `tt`
requests renewed consent. Older releases retain legacy Claude fields for
compatibility, but no legacy grant authorizes the current disclosure scope. Each
request supports:

- full context, containing every active, non-archived project and its task
  content;
- redacted context, which sends only opaque stable project/task IDs and
  non-text metadata while omitting project names and slugs, qualified task
  references, titles, descriptions, and tags; or
- no context.

Only a consented full-context Codex request receives live workspace inspection
capability. The Codex adapter keeps its fresh temporary launcher directory,
uses `codex exec --cd <workspace>` with `TT_DATA_DIR` set to that exact
workspace, and retains its ephemeral read-only sandbox. When fresh data is
needed, its prompt permits only `tt project list --json` and `tt task list
--project <project> --json`. The sandbox is the enforcement boundary: Codex
cannot mutate the workspace directly. Redacted, no-context, and declined
requests receive no workspace path or filesystem inspection capability;
Claude and Copilot retain their existing adapter restrictions.

The machine-local `machine.json` remembers the TUI's selected harness and
per-harness explicit configuration directories. CLI `--harness` and
`--config-dir` values override one request without rewriting those defaults.
Without an explicit directory, the provider environment variable
(`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, or `COPILOT_HOME`) and then its native default
apply. Prompts are sent to the configured provider over standard input, not process arguments.
Provider output is treated as untrusted data and must match the
versioned structured action schema. `tt` resolves targets and classifies risk
locally, prepares one snapshot-bound preview, previews safe actions as one batch,
requires a separate confirmation for destructive actions, and commits accepted
actions as one undoable transaction. If any project consulted by the accepted plan
changes before apply, the plan expires without merge or partial effects and the user
must obtain a fresh response and preview. Shell commands from provider output are
rejected.

Consent controls what `tt` supplies; the provider's own CLI, account, and data
retention policies still apply. Redacted context does not expose the workspace
directory, and `--no-context` sends no workspace data or access capability.

## Upgrade and migration expectations

Schema-v1 task Markdown is read through a deterministic migration mapping:
canceled; done/completed; review; in progress; blocked/waiting; standby/paused;
maybe/someday; inbox; next; then ready. Nonterminal tasks from the legacy Action
Inbox become `inbox`. A legacy `active` metadata key is accepted only to parse
old task files safely, then discarded; it never adds a task to Focus Queue. The
migration audit records original status, section, stage, and custom-stage
configuration before schema-v2 becomes canonical.

Workspace-domain migration and location moves use staged writes and validation.
Keep the pre-migration copy until the audit and acceptance checks pass. Do not
manually convert `state:` metadata or delete the audit merely to silence a
warning; fix the source or restore the backup and rerun the migration.

An aggregate `.tt.json` without split-domain markers is not imported during normal
startup. Startup fails before creating split files or recovery artifacts so an
operator can migrate or archive the legacy source deliberately. Conversely, a stale
aggregate beside a split bundle never overrides that bundle. No permanent aggregate
compatibility or implicit conversion path is part of the runtime architecture.

## Backup and restore

Back up the complete selected data directory, including `.tt/` and `projects/`.
For a simple filesystem copy, stop all `tt` processes first so the copy observes
one transaction boundary. Snapshot-capable backup tools may instead capture the
whole directory atomically.

Keep backups outside the selected workspace and outside its cloud provider when
you need protection from provider deletion or account loss. Test restores into
a separate directory with `tt --data-dir <restore> doctor` before relying on
them. `.completed.index.json` is derived and rebuildable, but task Markdown,
domain JSON, notes, attachments, transaction/undo artifacts, tombstones, and
conflict bundles can contain unique recovery data.

Machine state may be backed up separately if navigation history and local editor
preferences matter; it is not required to recover canonical tasks.

## Platform support

The release candidate is tested on current macOS and Linux runners. Durable
locking and atomic filesystem behavior are implemented for those platforms, and
release builds target `darwin`/`linux` on `amd64`/`arm64`. Windows is not a
supported release target.

Claude, Codex, GitHub Copilot, `fzf`, `glow`, editors, and other external
integrations are optional.
Core CLI, TUI, local storage, transactions, and recovery do not require them.

## Release-candidate checks

Run the same gate as GitHub CI:

```bash
just ci
```

This regenerates and checks documentation, runs race-enabled tests and coverage
floors, replays every committed TUI scenario, vets and lints, checks `go mod
tidy`, rejects dead code, and builds all packages. Use `just tui-scenarios` to
replay only the committed TUI scenarios.
