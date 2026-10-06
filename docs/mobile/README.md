# Mobile bridge and sync contracts

These documents fix the compatibility baseline for the native iPhone app and
offline sync. Code and the committed fixtures decide what is compatible: where a
document here disagrees with `internal/` or with the fixtures under
`internal/compat/testdata/`, the code wins and the document is wrong.

| Contract | Version | Document |
| --- | --- | --- |
| Bridge request/response | `bridge_version` 1 | [bridge-contract.md](bridge-contract.md) |
| Sync transaction manifest | `tt-sync-txn` 1 | [sync-envelope.md](sync-envelope.md) |
| Device capability advertisement | `tt-sync-device` 1 | [sync-envelope.md](sync-envelope.md#capability-negotiation) |
| Compatibility fixtures | `tt-compat-fixture` 1 | [fixture-contract.md](fixture-contract.md) |
| Durable write paths | — | [write-path-inventory.md](write-path-inventory.md) |

## Desktop artifacts this baseline supports

These are the formats the bridge reads and sync exchanges. The numbers below are
what the code uses today. `internal/compat` fails if a fixture declares
anything else.

| Artifact | Path | Format | Synced |
| --- | --- | --- | --- |
| Workspace catalog | `.tt/workspace.json` | domain envelope 1, catalog payload schema 21 | yes |
| Planning | `.tt/planning.json` | domain envelope 1 | yes |
| Named lenses | `.tt/lenses.json` | domain envelope 1 | yes |
| Project document | `projects/<slug>/tasks.md` | task Markdown schema 19 | yes |
| Project notes | `projects/<slug>/notes/*.md` | YAML frontmatter + Markdown | yes |
| Global notes | `.tt/global-notes/*.md` | YAML frontmatter + Markdown | yes |
| Attachment bytes | `projects/<slug>/.attachments/<task-hash>/<file>` | opaque bytes | yes |
| Machine state | `~/.tt/runtime/<workspace-id>/machine.json` | domain envelope 1 | never |
| Commit journals | `.tt/transactions/` | journal manifest | never |
| Undo ledger | `.tt/undo/` | ledger entries and images | never |
| Conflict bundles | `.tt/conflicts/` | bundle directory | not in v1 (see [Open questions](sync-envelope.md#open-questions)) |
| Migration snapshots | `.tt/backups/` | archive/audit pairs | never |
| Domain sidecars and locks | `*.bak`, `*.lock` | — | never |

The catalog payload schema (21) and the task Markdown schema (19) are separate
numbers. Neither one is computed from the other. Both come from the code:
`config.CurrentConfigVersion`, `storage.CurrentTaskSchemaVersion`, and
`config.CurrentDomainSchemaVersion` for the envelope. Some architecture chapters
still describe older numbers; for example, the persistence chapter calls
`tasks.md` schema 17.

### Task fields

The bridge carries every field of the current task record.
`TestCompatFixtureCoverageSurvivesLiveParsing` proves that each one survives
the live reader:

`id`, `short_id`, `title`, `state`, `priority`, `parent`, `checklist-origin`,
`tags`, `context`, `created`, `updated`, `state_changed`, `completed`, `due`,
`due_time`, `deferred`, `defer_count`, `pinned`, `pinned_at`, `review`,
`attachments` (file, URL, and code reference), `checklist`, `depends`,
`check_in`, `check_in_repeat`, `reviewed`, and the `RESUME`, `DESCRIPTION`, and
`COMMENTS` blocks. Each project document also carries its `PROJECT-COMMENTS`
block.

The catalog carries each project's `id`, `slug`, `name`, `description`,
`created_at`, `updated_at`, `archived`, `archived_at`, `pinned`, `pinned_at`,
`color_slot`, and `tags`. The `parked` tag is the only tag that changes
behavior.

## Implementation

`internal/bridge` serves the bridge contract over the shared runtime in
`internal/engine`, and `mobile/ttbridge` is the gomobile binding. The iPhone
app, its Swift package, and the XCFramework build are described in
[`ios/README.md`](../../ios/README.md).

## Verification

```bash
bash scripts/verify_mobile_session.sh 1
bash scripts/verify_mobile_session.sh 3   # macOS with Xcode: builds both slices, runs the simulator suite
```
