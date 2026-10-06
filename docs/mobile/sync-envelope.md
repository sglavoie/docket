[← Mobile contracts](README.md)

# Sync transaction envelope, version 1

Each device commits locally first. Afterwards, sync publishes what the device
committed as immutable, content-addressed transaction manifests and blobs in a
shared exchange folder. Devices never synchronize live workspace files with
each other. The exchange folder is a log, and each device's Markdown workspace
is that device's materialized view of it.

## Exchange folder layout

```text
<exchange>/tt-sync/v1/
├── workspace.json                     # exchange identity: workspace_id, format
├── devices/<device-id>.json           # capability advertisement, rewritten by its owner only
├── txns/<device-id>/<seq>-<hash>.json # immutable transaction manifests
└── blobs/sha256/<aa>/<hash>           # immutable content, named by its digest
```

Each device writes only under its own `devices/` file and its own `txns/`
directory. Blobs are named by their content, so two devices writing the same
blob write identical bytes. A file that is missing or still downloading means
"not yet delivered". It never means "deleted".

## Transaction manifest

```json tt:sync-manifest
{
  "format": "tt-sync-txn",
  "format_version": 1,
  "workspace_id": "8f58ae6c-06ce-4dc0-96a2-cd2f533096c4",
  "txn_id": "sha256:4be0c3…",
  "device_id": "c7f3a9e2-0d41-4b8e-9a6f-2e5d1c0b7a93",
  "device_seq": 42,
  "parents": ["sha256:91d2aa…", "sha256:0c77e4…"],
  "kind": "user",
  "local_txn_id": "1f0e5a3c-6d2b-4c8e-8f1a-9b7d3e2c5a40",
  "created_at": "2026-10-06T18:22:04.512Z",
  "schemas": { "domain_envelope": 1, "catalog": 21, "task_markdown": 19 },
  "requires": ["project-document.v19", "planning.v1"],
  "effects": [
    {
      "kind": "project_document",
      "project_id": "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01",
      "path": "projects/atlas-launch/tasks.md",
      "op": "put",
      "base_blob": "sha256:3b1f…",
      "blob": "sha256:a9c4…"
    },
    {
      "kind": "planning",
      "path": ".tt/planning.json",
      "op": "put",
      "base_blob": "sha256:9a4c…",
      "blob": "sha256:2e81…"
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `format`, `format_version` | Always `tt-sync-txn` and `1`. A reader that does not support the version holds the manifest (see [Capability negotiation](#capability-negotiation)). |
| `workspace_id` | Must equal the catalog's workspace id. Any other value is refused, never imported. |
| `txn_id` | `sha256:` followed by the hex digest of the manifest's canonical JSON with `txn_id` omitted. Canonical JSON has sorted keys, no insignificant whitespace, and UTF-8 encoding. Because the id is the content, a manifest cannot be edited after it is published. |
| `device_id`, `device_seq` | The author, and a gap-free per-device counter starting at 1. A gap tells the reader that delivery is incomplete. It never means the transaction was skipped. |
| `parents` | The heads this device had applied when it committed: the causal ancestry. A manifest is applied only after all of its parents. Several parents mean the author had merged concurrent histories. |
| `kind` | One of `user`, `undo`, `redo`, `merge`, `resolution`, `repair`, `external`. Undo and redo are new forward changes. A reader never treats them as image restores. `external` imports a valid change made outside a commit (see [External changes](#external-changes)). |
| `local_txn_id` | The author's `.tt/transactions` journal id. It is diagnostic only. |
| `created_at` | Informational only. Never use it to order transactions or choose a winner. |
| `schemas` | The author's live artifact schemas when it committed. |
| `requires` | The capabilities a reader needs in order to apply these effects. |
| `effects` | Every durable effect of the transaction, all of them. A local commit that touched N artifacts produces exactly N effects. |

### Effects

| Field | Meaning |
| --- | --- |
| `kind` | `project_document`, `catalog`, `planning`, `lenses`, `note`, `global_note`, `attachment`, `project_tree`. |
| `path` | Workspace-relative, `/`-separated, and checked with the same approved-root containment the commit journal uses. Absolute paths and `..` are refused. |
| `op` | `put`, `delete`, or `move` (`move` also carries `target_path`). |
| `base_blob` | The digest of the bytes the author replaced, or `null` if the path did not exist. This is the merge base. It is required for `delete`, so every deletion is explicit and names what it deleted. |
| `blob` | The digest of the new bytes, or `null` for a `delete`. |
| `project_id` | Required for `project_document`, `note`, `attachment`, and `project_tree`. It ties the effect to a project identity even when the slug changes. |

A manifest becomes **complete** once every `blob` it names and every one of its
`parents` is present locally. Incomplete manifests wait. They are never applied
in part.

### What is never exchanged

`machine.json`, `.tt/transactions/`, `.tt/undo/`, `.tt/backups/`, every `*.bak`
and `*.lock`, logs, and process locks. Each of these appears in the write-path
inventory under `machine-local`, `commit-staging`, or `ledger-restore`. The
fixture test `TestCompatDesktopOpenLeavesFixtureUntouched` shows why the rule
is stated by path rather than by "whatever changed": when a desktop joins a
workspace for the first time, it adopts the workspace lenses and leaves a
`lenses.json.bak` sidecar behind, even though `lenses.json` itself does not
change.

## Atomicity

The local commit and its outbound record are written together. The outbound
manifest is staged inside the same `.tt/transactions` journal entry that
publishes the commit, so after a crash recovery either both exist or neither
does. Importing works the same way: applying a manifest and recording its
receipt happen in one local transaction.

## Local sync history

Each device keeps its own history under `<data>/.tt/sync/`. It is never
exchanged as files; only the manifests and blobs it holds are published.
`internal/synclog` plans every change to it as journal operations, and
`storage.CommitService` stages them in the manifest of the transaction they
describe, so nothing here is ever written on its own.

```text
.tt/sync/
├── blobs/sha256/<aa>/<hash>                       # retained bytes, base and new
├── index/<aa>/<hash>.json                         # txn_id → device, seq, stored record
└── devices/<device-id>/
    ├── device.json                                # identity: device_id, workspace_id
    ├── state.json                                 # heads, next_seq, outbox, received, tracked
    ├── txns/<seq>-<hash>.json                     # immutable outgoing manifests
    └── incoming/<author>/<seq>-<hash>.json        # immutable receipts, each with its manifest
```

- **Recording.** Every `CommitService` entry point, and the ledger's undo and
  redo, records when a history is attached; the engine attaches one to every
  rooted runtime. User commits are `kind: user`, undo and redo are `undo` and
  `redo`, and `CommitProjectRepair` and the planned maintenance commits are
  `repair`. A transaction that changes no synced bytes records nothing.
- **Effects.** Each changed synced file is one effect. A rewrite of identical
  bytes is not a change. Directories are not artifacts: a directory move
  becomes one `move` per file it carries, and removing a directory is
  published through the deletion of each file in it. Device-local paths — the
  journal, the ledger, this history, `.tt/backups/`, `.tt/conflicts/`, `*.bak`,
  `*.lock`, and staging files — are never effects. A commit whose change the
  history cannot classify, or whose project directory no catalog names, is
  refused whole rather than published incomplete.
- **Blobs.** The bytes behind every `base_blob` and `blob` are retained, so a
  peer always has the merge base an effect names.
- **Ancestry.** `heads` are the transactions nothing known descends from. A
  local transaction's parents are the current heads and it becomes the only
  head; an applied incoming transaction replaces its own parents among the
  heads, so concurrent histories leave several heads until the next local
  transaction names them all.
- **Outgoing.** `outbox` lists recorded transactions until the transport
  confirms publication (`CommitService.CommitSyncPublished`); the records
  themselves are retained.
- **Incoming.** `CommitService.CommitIncoming` applies a manifest only as a
  fast-forward: every effect's base must be this device's current bytes. It
  refuses, applying and acknowledging nothing, a manifest that is malformed or
  whose `txn_id` does not match its content, addressed to another workspace,
  authored here, already received, held by capability or schema, missing an
  earlier `device_seq`, a parent, or a blob, or concurrent with a local change
  to the same path. `received` holds each remote device's highest applied
  sequence number.
- **Tracking.** `tracked` holds each synced path's digest as of the last
  recorded or applied transaction: the last indexed revision that external-edit
  detection compares against. `indexed` says it covers every synced file: the
  first time a device records anything — a commit, an apply, or an observation
  — it indexes the workspace as it stands and retains those bytes as blobs, so
  every later difference is a change with a merge base the device can supply.

Checked domain saves outside the journal (`DomainCoordinator.CommitCatalog`,
`CommitPlanning`, `CommitLensDefinitions`) are not recorded at the commit
boundary. Their bytes differ from `tracked` afterwards, which is exactly what
the external-change observation compares, so they are imported as changes by
that observation rather than by a second hook.

### External changes

`CommitService.CommitExternalChanges` observes the workspace: every synced
file whose bytes differ from `tracked` — edited, created, or removed outside a
commit — is judged, and every valid difference becomes one effect of a single
outgoing `kind: external` transaction whose `base_blob` is the indexed digest.
The observation writes only the history, never the workspace. It runs at
`engine.Open` and before every `CommitIncoming`, so an edit made while no
runtime held the workspace, or a domain save, is published before an incoming
manifest is reconciled.

A difference is held, not published, when the live readers would refuse it: a
project document that does not parse, fails the project audit, or does not
render through the canonical writer; a catalog, planning, or lens file the
domain loader refuses (`syncmerge.Merger.ValidateArtifact`); a catalog naming
another workspace; a removed catalog, planning, or lens file; a removed
`tasks.md` whose project the catalog still names; or a file in a project
directory no catalog names. A held file stays on disk byte for byte. Because it
is not indexed, reconciliation refuses to write over it (`ErrUnrecordedChange`)
and the incoming manifest waits; once the file is repaired, the next
observation imports it and the manifest reconciles with it. Files outside the
synced layout are ignored.

### Undo and redo

The undo ledger (`.tt/undo/`) is device-local and never exchanged. Each entry is
stamped with the device that committed it, and a device restores only its own
entries, so a workspace copied with its ledger cannot replay another device's
history; entries recorded before sync carry no device and stay restorable. A
restore is checked against the current bytes of every file it touches and is
refused whole when any of them no longer holds the image the entry left —
another device's applied change, a reconciliation, or an unrecorded edit — so
undo never restores an old image over someone else's work. A successful undo or
redo is published as a new `kind: undo` or `redo` transaction whose parents are
the device's heads.

## Capability negotiation

Each device keeps one advertisement at `devices/<device-id>.json`:

```json tt:device-capabilities
{
  "format": "tt-sync-device",
  "format_version": 1,
  "device_id": "c7f3a9e2-0d41-4b8e-9a6f-2e5d1c0b7a93",
  "device_name": "iPhone",
  "platform": "ios",
  "app_version": "1.0.0 (42)",
  "bridge_versions": [1],
  "sync_format_versions": [1],
  "schemas": { "domain_envelope": 1, "catalog": 21, "task_markdown": 19 },
  "capabilities": [
    "catalog.v21",
    "planning.v1",
    "lenses.v1",
    "project-document.v19",
    "project-comments.v1",
    "check-ins.v1",
    "notes.v1",
    "attachments.v1",
    "project-tree.v1"
  ],
  "updated_at": "2026-10-06T18:00:00Z"
}
```

`platform` is one of `macos`, `ios`, or `linux`.

The rules:

1. **Writers declare their needs.** A manifest's `requires` lists every
   capability its effects need. For example, a `project_document` effect at
   schema 19 requires `project-document.v19`.
2. **Readers hold what they cannot apply.** A reader applies a manifest only if
   all of these hold: it supports `format_version`, its `schemas` equal the
   manifest's, and `requires` is a subset of its own `capabilities`. Otherwise
   the manifest is marked `held: capability` with the missing items named. A
   held manifest is never acknowledged, skipped, or dropped. Every descendant
   of a held manifest waits behind it. The UI reports "update tt on this
   device" and never suggests that the change was lost.
3. **Advertisements are informational.** They let a device warn before it
   publishes something a peer cannot read yet. They never gate a local commit:
   another device never approves a local save.
4. **Schemas move by upgrade, not by exchange.** A schema migration runs locally
   on each device and is never published as a transaction. A device with a
   newer schema keeps publishing at its own schema. Older peers hold those
   manifests until they upgrade.

## Reconciliation

A device's workspace is a pure function of the set of transactions it holds.
Two devices holding the same transactions hold the same bytes and the same
conflicts, whatever order the transactions arrived in. `synclog.History.PlanReconcile`
decides which versions compete; `internal/syncmerge` merges them by what the
file means. With a merger attached (the engine always attaches one),
`CommitService.CommitIncoming` reconciles every incoming manifest. Without
one it applies fast-forwards only, as described above.

- **Logical keys.** A project-scoped path is keyed by its `project_id` and its
  path inside the project directory, and any other path is its own key. A
  `move` is a removal at its source and a write at its target. A rename on one
  device and an edit on another are therefore one file. It lands under the
  slug the resolved catalog gives its project.
- **Causal fast-forward.** The writers of a key that no other writer of it
  descends from are its maximal writers. With one maximal writer, its value
  stands.
- **Concurrent merge.** With several maximal writers, their values are merged
  against the key's value at the intersection of their histories. That base
  is resolved the same way, recursively, so criss-cross histories with several
  common ancestors need no arbitrary choice of one ancestor. Identical values
  coalesce, and a value equal to the base changed nothing. Remaining versions
  are ordered by digest, which makes the first one the same *primary* version
  on every device. Conflicts fall back to it, never to a timestamp.
- **Project documents** (`storage.ReconcileProjectDocuments`) merge task by
  task and field by field, any number of sides at once. A task whose versions
  compete is kept whole as the primary side wrote it, and a change always
  beats a deletion. A deleted parent survives while anything beneath it
  changed. Merged reparenting that cycles or nests deeper than six reverts to
  the base hierarchy. Incompatible sibling reorders keep the base order. Two
  tasks created concurrently under one short id both keep their UUIDs: the
  task that held the number in the base, or else the smaller UUID, keeps it,
  and the others are renumbered past the largest number in use. That
  renumbering is a resolution, not a conflict.
- **Catalog** (`config.ReconcileCatalogArtifacts`) merges project by project.
  A changed project beats its deletion. A merged rename onto a slug another
  project took reverts to its base slug, and two projects created under one
  slug keep it on the smaller id while the other takes a suffix of its own id.
- **Planning** (`config.ReconcilePlanningArtifacts`) merges Today and Next as
  set changes against the base. One reorder, or several compatible ones, is
  adopted. Incompatible reorders keep the base order. A task planned for Today
  on one device and for Next on another stays in Today.
- **Other files** (notes, lenses, attachments, other project files) keep the
  primary version and record the competition.
- **Workspace repair.** A project whose files survived because a change beat
  its deletion is named in the catalog again, using the entry the deleting
  transaction removed.
- **Writes.** Only files whose resolved bytes change are written. Each one must
  still hold what the history says this device last wrote there. Otherwise
  the manifest is refused with `ErrUnrecordedChange` until that change is
  recorded. The applied bytes, retained blobs, conflict records, receipt, and
  tracking state commit in one journal entry. Reconciliation publishes
  nothing: the next local transaction names every head as a parent and carries
  the merged bytes as its base.

### Conflicts

Conflicts are derived, not exchanged. This answers session 1's open question.
`History.OpenConflicts` (and `CommitService.SyncConflicts`) return the
conflicts a history leaves open. Each is a `tt-sync-conflict` record whose `id`
is the digest of its key, kind, base, candidates (each competing digest with
the maximal writers that left it), result, and details. Devices holding the
same transactions therefore derive the same records under the same ids. Every
competing version stays retained as a blob, so nothing a device wrote is lost.
A record is also stored under `.tt/sync/conflicts/` when reconciliation first
derives it, and the receipt names the conflicts open after it.

A conflict closes when a transaction writes its key and descends from every
competing writer. `CommitService.CommitSyncResolution` commits such a
`resolution`, and it records the choice even when the chosen bytes are the
current ones. If two devices resolve one conflict at once, identical
resolutions coalesce and the conflict closes everywhere. Different resolutions
compete again as one new conflict whose candidates are the two resolutions.

## Open questions

- Conflict bundles under `.tt/conflicts/` remain the local commit-time
  merge's. Sync conflicts are derived per device (see [Conflicts](#conflicts));
  surfacing them in the frontends is later work.
- `lenses.json`, and catalog or planning saves made through the checked domain
  store, are committed outside the journal and published by observation (see
  [External changes](#external-changes)), which runs at launch and before every
  incoming manifest. A frontend that wants them published sooner must observe
  after saving; continuous observation belongs to the transport's polling.
- Undo refuses whenever another device changed a file the entry touches, even
  a different task in the same `tasks.md`. A three-way rebase of the inverse
  change would let more undos through.
- Reconciliation reads the whole history on every incoming manifest. Caching
  the resolved workspace, or compacting history, is deferred with history
  retention.
- A project directory emptied by a reconciled rename is left in place,
  because its lock files are device-local.
