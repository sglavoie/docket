[← Mobile contracts](README.md)

# Durable write-path inventory

Sync has to see every durable change a device makes. This page names each place
the code writes to disk and the integration point sync uses for it. The
inventory itself is
[`internal/compat/testdata/write_paths.tsv`](../../internal/compat/testdata/write_paths.tsv).
It has one row per production function that references a filesystem mutator
(`os.WriteFile`, `os.Rename`, `os.Remove`, `os.RemoveAll`, `os.Create`,
`os.CreateTemp`, `os.OpenFile`, `os.Mkdir`, `os.MkdirAll`, `os.MkdirTemp`,
`os.Symlink`, `os.Link`, `os.Chmod`, `os.Chtimes`, `os.Truncate`, and
`util.AtomicWriteFile`). A reference counts whether the code calls the mutator
or stores it as a value. A package-level variable bound to a mutator counts as
an alias, and every function that names the alias counts too.
`TestCompatWritePathInventoryCoversEveryDurableWrite` regenerates the scan
from the AST and fails in three cases:

- a write site has no row;
- a row's write site is gone;
- a row's class is not one of the classes below.

## Integration classes

### `commit-journal`

The single publish path for user mutations. Every task, project, note,
attachment, planning, and catalog change that a use case makes goes through
`storage.CommitService`. Undo and redo restores and migrations publish through
the same `transaction.Journal` (`NewJournal` binds its rename, remove, and
atomic-write primitives).

**Sync integration point:** the outbound manifest is staged in the same journal
entry that publishes the commit, so it is committed or rolled back with the
commit. `CommitService.prepareTransaction` is that one hook for every commit
path, and the ledger's restore stages the same record for undo and redo; see
[Local sync history](sync-envelope.md#local-sync-history). The rows
outside `CommitService`, namely `SaveProject`, `CreateProjectDir`,
`deleteProjectDir`, `writeFileAtomic`, and the `engine.Repository` fallback, are
reached only through the test-double repository. The audited raw-save
allowlist in `internal/storage` keeps it that way.

### `commit-staging`

Lock files, lock directories, attachment directories, and the workspace
skeleton that are created around a commit. They carry no synced content on
their own.

**Sync integration point:** none. These files are never exchanged.

### `domain-store`

`config.DomainFileStore`'s checked save (`saveObserved`) and its `.bak`
sidecars. Catalog and planning commits reach disk here when a caller uses
`DomainCoordinator.CommitCatalog`/`CommitPlanning` instead of a
`CheckedDomainArtifacts` change set. Lens definitions *always* reach disk here,
outside any workspace transaction and outside the undo ledger. Machine state
also reaches disk here, but it stays local.

**Sync integration point:** observation. A checked save is not recorded when
it lands; its bytes then differ from the history's `tracked` digest, and the
external-change observation imports that difference as a change. `.bak`
sidecars and `machine.json` are never exchanged.

### `ledger-restore`

Undo-ledger maintenance (`WorkspaceLedger.gcBlobs`). The ledger is local.

**Sync integration point:** none for the ledger itself. An undo or redo is a
new forward change: `WorkspaceLedger.Undo`/`Redo` publishes through the journal
and syncs as a `kind: undo`/`redo` manifest whose effects carry the current
bytes as `base_blob`. It never restores images over edits made on another
device: the restore preflight refuses the whole entry when any file no longer
holds the image the entry left. Undo history is device-local: each entry is
stamped with the device that committed it, and a device only restores its own
entries (or unstamped entries recorded before sync).

### `workspace-relocation`

`tt workspace move` copies, verifies, and switches the active workspace.

**Sync integration point:** relocation is outside the exchange. The moved
workspace keeps its workspace id, and the copy produces no transactions. Moving
an enrolled working copy keeps its history, since `.tt/sync/` moves with it.

### `sync-enrollment`

`tt sync enroll` snapshots the original workspace under the machine-local
`~/.tt/enrollments/`, stages a working copy beside its final path, advances the
staged catalog to the enrolled schema, writes the device-local enrollment
record under `.tt/sync/`, and publishes the working copy with one rename.
`tt sync join` creates the empty directory a joining device imports into.

**Sync integration point:** the genesis. None of these writes is a
transaction: the working copy's first sync record is the `kind: enroll`
genesis `CommitService.CommitSyncGenesis` records when a runtime first opens
it, which is how every later device rebuilds the same bytes. The snapshot and
the enrollment record are never exchanged.

### `sync-transport`

The desktop's folder exchange (`syncexchange.ExchangeFolder`) writes immutable,
content-addressed manifests and blobs, the exchange identity, and this device's
advertisement into the exchange folder, and keeps its inbox and status in the
device-local `.tt/sync/transport/`.

**Sync integration point:** it *is* the exchange. It never deletes from the
exchange folder, and it applies nothing itself: staged manifests are imported
through `CommitService.CommitIncoming` by `ImportInbox`, and publication is
acknowledged through `CommitService.CommitSyncPublished`.

### `conflict-store`

`ConflictStore.Save`/`Delete` write merge bundles under `.tt/conflicts/`.

**Sync integration point:** local in v1. Resolving a conflict commits through
the journal like any other change, so the resolution syncs. Whether the bundles
themselves sync is an
[open question](sync-envelope.md#open-questions) for session 5.

### `external-editor`

Temporary buffers that tt writes before handing them to `$EDITOR`. These cover
the task content, comments, notes, and the outline dump. The edited result
always comes back in through an `internal/app` use case, and so through
`commit-journal`.

**Sync integration point:** the commit that consumes the buffer. The buffers
themselves are never exchanged.

### `export`

`tt project export` writes outside the workspace.

**Sync integration point:** none. An export is not workspace state.

### `machine-local`

Logs, process locks, the control socket, the active-workspace pointer, and the
phone's capture inbox (`internal/bridge/capture_inbox_ops.go`): the receipts that
keep a saved capture from importing twice, the status beside a refused one,
and the removal of imported entries from the App Group container.

The observation stat cache (`History.saveStatCache`, `.tt/sync/stat-cache.json`)
is here too, although it sits inside the workspace: it remembers the size,
modification time, and digest each synced file was last read at, so an
observation reads only the files that changed. It is derived from those files
alone, accounts for no transaction, and is safe to delete.

**Sync integration point:** none, ever.

## Commit entry points

`TestCompatCommitEntryPointsAreDocumented` requires this table to name every
exported commit, undo, redo, migrate, adopt, recover, and restore method on
`CommitService`, `WorkspaceLedger`, `DomainCoordinator`, and `Manager`.

| Entry point | Class | Sync integration |
| --- | --- | --- |
| `CommitService.Commit` | commit-journal | Outbound manifest in the commit's journal entry. |
| `CommitService.CommitMulti` | commit-journal | One manifest for the whole multi-project transaction. |
| `CommitService.CommitOps` | commit-journal | Raw ops (moves, deletes, project trees). Each op becomes an effect, and deletes become explicit `delete` effects. |
| `CommitService.CommitOpsWithArtifacts` | commit-journal | As `CommitOps`, plus artifact effects. |
| `CommitService.CommitOpsTransaction` | commit-journal | As `CommitOps`, recorded in the ledger. |
| `CommitService.CommitPlannedOpsWithArtifacts` | commit-journal | Planned under the commit lock. Effects are taken from the planned ops. |
| `CommitService.CommitArtifactTransaction` | commit-journal | Artifact-only user mutation: notes, attachments, domains. It is synced even though no `tasks.md` changes. |
| `CommitService.CommitArtifacts` | commit-journal | Config-only artifact commit with no undo entry. It still syncs. |
| `CommitService.CommitPlannedArtifacts` | commit-journal | Maintenance snapshot. It syncs only if it touches synced artifacts. Migration snapshots under `.tt/backups/` never sync. |
| `CommitService.CommitProjectRepair` | commit-journal | `doctor --fix` is published as a `kind: repair` manifest. |
| `CommitService.CommitIncoming` | commit-journal | Applies another device's manifest and records its receipt in one journal entry; with a merger attached it reconciles the manifest with the whole history and stores the derived conflict records in that same entry. Never produces an outgoing record. |
| `CommitService.CommitSyncResolution` | commit-journal | Resolves a derived sync conflict by writing its path as an undoable `resolution` transaction, which records an outgoing record even when the chosen bytes are the current ones. |
| `CommitService.CommitSyncResolutions` | commit-journal | Resolves several sync conflicts as one undoable `resolution` transaction, so a change that left conflicts in several files is resolved whole. Each path must still hold the bytes its conflict reported under the commit's artifact locks, or nothing is written. `engine.Runtime.ResolveSyncConflict` (bridge `conflict.resolve`, `tt sync resolve`) commits through it. |
| `CommitService.CommitSyncPublished` | commit-journal | Drops published transactions from the outbox through the journal. Never produces an outgoing record. |
| `CommitService.CommitExternalChanges` | commit-journal | Observes the synced workspace against the history's `tracked` digests and records every valid difference as one outgoing `kind: external` transaction; writes only the history, never the workspace. Invalid differences are held on disk unpublished. The first observation on a device only indexes the workspace. `CommitIncoming` runs the same observation first, and `engine.Open` runs it at launch. |
| `CommitService.CommitSyncGenesis` | commit-journal | Records an enrolled working copy's first transaction, `kind: enroll` at sequence 1 with no parents, writing every synced file as a creation; writes only the history. Refused whole when the device already has history or a file would be held. `engine.Open` runs it before the first observation of an enrolled working copy. |
| `CommitService.RecoverWorkspace` | commit-journal | Recovery completes or rolls back a journal entry, and its outbound record with it. |
| `WorkspaceLedger.Undo` | commit-journal | A new forward `kind: undo` manifest. |
| `WorkspaceLedger.UndoWithEntry` | commit-journal | As `Undo`. |
| `WorkspaceLedger.UndoTransactionWithEntry` | commit-journal | As `UndoWithEntry`, refusing with `ErrUndoSuperseded` when the named transaction is no longer the newest entry. |
| `WorkspaceLedger.Redo` | commit-journal | A new forward `kind: redo` manifest. |
| `WorkspaceLedger.RedoWithEntry` | commit-journal | As `Redo`. |
| `WorkspaceLedger.UndoCount` | — | Read-only. |
| `WorkspaceLedger.RedoCount` | — | Read-only. |
| `DomainCoordinator.CommitCatalog` | domain-store | Outbound `catalog` effect. |
| `DomainCoordinator.CommitPlanning` | domain-store | Outbound `planning` effect. |
| `DomainCoordinator.CommitLensDefinitions` | domain-store | Outbound `lenses` effect. Today this path is outside every transaction. |
| `DomainCoordinator.CommitMachine` | domain-store | Machine-local. Never exchanged. |
| `DomainCoordinator.AdoptArtifacts` | — | Adopts revisions in memory and writes nothing. |
| `Manager.CommitCatalog` | domain-store | Delegates to `DomainCoordinator.CommitCatalog`. |
| `Manager.CommitPlanning` | domain-store | Delegates to `DomainCoordinator.CommitPlanning`. |
| `Manager.CommitMachine` | domain-store | Machine-local. |
| `Manager.AdoptPortableLenses` | domain-store | One-time adoption per machine. It republishes `lenses.json` and leaves a `.bak` sidecar, so a changed digest is the only evidence that warrants an outbound effect. |
| `Manager.RecoverWorkspace` | commit-journal | Journal recovery, then the local migration chain, then lens adoption. |
| `Manager.RestoreConfigFromBackup` | domain-store | Restores domains from `.bak`. A restored synced domain is published as a domain change. |
| `Manager.MigrateWorkspaceToV5` … `Manager.MigrateWorkspaceToV21` | migration | Local only. Migrations are never exchanged; see [schemas move by upgrade](sync-envelope.md#capability-negotiation). |

The migration entry points named one by one: `Manager.MigrateWorkspaceToV5`,
`Manager.MigrateWorkspaceToV6`, `Manager.MigrateWorkspaceToV7`,
`Manager.MigrateWorkspaceToV8`, `Manager.MigrateWorkspaceToV9`,
`Manager.MigrateWorkspaceToV10`, `Manager.MigrateWorkspaceToV11`,
`Manager.MigrateWorkspaceToV12`, `Manager.MigrateWorkspaceToV13`,
`Manager.MigrateWorkspaceToV14`, `Manager.MigrateWorkspaceToV15`,
`Manager.MigrateWorkspaceToV16`, `Manager.MigrateWorkspaceToV17`,
`Manager.MigrateWorkspaceToV18`, `Manager.MigrateWorkspaceToV19`,
`Manager.MigrateWorkspaceToV20`, `Manager.MigrateWorkspaceToV21`.

## External Markdown editing

Some writes happen outside tt, for example a person editing `tasks.md` or a
note in their own editor. Sync has no hook inside that editor, so its
integration point is **observation**:

- A valid external edit changes the raw bytes, and therefore the raw artifact
  revision. If the edit changes meaning, the canonical revision changes too.
  Whitespace-only edits leave the canonical revision unchanged
  (`ProjectDocument.ComputeRevision` hashes the canonical rendering). Change
  detection for sync must therefore compare the raw digest with the last
  synced `blob`, not the canonical revision.
- The strict reader refuses an invalid external edit. The edit is never read as
  a smaller project, so it cannot be mistaken for a deletion.
- Sync records an observed external edit as a local `kind: external`
  transaction whose `base_blob` is the last indexed digest
  (`CommitService.CommitExternalChanges`). An edit the live readers would
  refuse is held instead: it stays on disk byte for byte, is not published, and
  blocks reconciliation of that path until a repaired version is observed.

`TestCompatExternalMarkdownEditIsObservable` holds both properties against the
representative fixture.
