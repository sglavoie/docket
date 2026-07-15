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
badge/filter. It is not a workflow state. Focus Queue and Daily Plan are also
planning collections, not states, so adding a task to Today does not rewrite its
lifecycle.

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

Project paths are derived from the data directory and safe catalog identifiers;
absolute project paths are not durable workspace configuration. Unknown
schema-v2 header blocks are preserved when task Markdown is rewritten.

Machine-specific state is intentionally separate:

```text
every workspace:  ~/.tt/runtime/<workspace-id>/machine.json
```

`<workspace-id>` is the canonical UUID stored in synced `workspace.json`. The
file contains editors, integration choices, current navigation, histories, and
layout state. A workspace with a different ID never shares this state. On first
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

A workspace-location move copies and validates the portable workspace, binds
and migrates its machine state while the source path is still known, then
switches the machine-local active-workspace pointer. Because the catalog ID is
preserved, navigation, history, and layout remain available at the destination
on the same machine. A true copy that is assigned a new workspace ID receives a
separate default machine-state namespace.

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

AI is optional. `tt` sends task context only after an explicit disclosure and
remembered consent stored in portable `settings.json`. Each request supports:

- full context;
- redacted context, which sends only opaque stable project/task IDs and
  non-text metadata while omitting project names and slugs, qualified task
  references, titles, descriptions, and tags; or
- no context.

Prompts are sent to the configured provider over standard input, not process arguments.
Provider output is treated as untrusted data and must match the
versioned structured action schema. `tt` resolves targets and classifies risk
locally, previews safe actions as one batch, requires a separate confirmation
for destructive actions, and commits accepted actions as one undoable
transaction. Shell commands from provider output are rejected.

Consent controls what `tt` supplies; the provider's own CLI, account, and data
retention policies still apply. Use `--no-context` when no workspace data should
leave the machine.

## Upgrade and migration expectations

Schema-v1 task Markdown is read through a deterministic migration mapping:
canceled; done/completed; review; in progress; blocked/waiting; standby/paused;
maybe/someday; inbox; next; then ready. Nonterminal tasks from the legacy Action
Inbox become `inbox`, and legacy `Active=true` membership is captured in Focus
Queue. The migration audit records original status, section, stage, Active, and
custom-stage configuration before schema-v2 becomes canonical.

Workspace-domain migration and location moves use staged writes and validation.
Keep the pre-migration copy until the audit and acceptance checks pass. Do not
manually convert `state:` metadata or delete the audit merely to silence a
warning; fix the source or restore the backup and rerun the migration.

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

Claude, `fzf`, `glow`, editors, and other external integrations are optional.
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
