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
| `kind` | One of `user`, `undo`, `redo`, `merge`, `resolution`, `repair`. Undo and redo are new forward changes. A reader never treats them as image restores. |
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
  detection compares against.

Checked domain saves outside the journal (`DomainCoordinator.CommitCatalog`,
`CommitPlanning`, `CommitLensDefinitions`) are not recorded at the commit
boundary. Their bytes differ from `tracked` afterwards, which is exactly what
the external-change observation compares, so they are imported as changes by
that observation rather than by a second hook.

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

Effects carry `base_blob`, so an importer can always reuse the existing
three-way merge. For project documents that merge is `storage.MergeProjectDocuments`.
The base is the bytes behind `base_blob`. "Local" is the importer's current
file, and "remote" is the bytes behind `blob`. Independent edits merge. Edits
that compete become durable conflicts, and timestamps never choose a winner.
Session 5 defines how this extends to the catalog, planning, lenses, notes,
attachments, and multi-ancestor histories.

## Open questions

- Conflict bundles under `.tt/conflicts/` are local today. A conflict must be
  resolvable on either device, so session 5 has to decide between two options:
  sync conflicts as records, or let each device re-derive them from the same
  manifests.
- `lenses.json`, and catalog or planning saves made through the checked domain
  store, are committed outside the journal. They are observed against
  `tracked` rather than hooked (see [Local sync history](#local-sync-history));
  the observation itself belongs to the external-edit work.
