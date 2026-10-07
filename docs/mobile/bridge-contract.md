[← Mobile contracts](README.md)

# Bridge contract, version 1

Swift talks to the Go engine through one narrow, versioned bridge. Go owns task
rules, the capture grammar, queries, persistence, migrations, transactions, and
reconciliation. Swift owns presentation and Apple system access, and never
parses `tasks.md` or a domain envelope itself.

## Binding surface

The XCFramework built with `gomobile bind` exports only types gomobile can
marshal. The whole bridge is two functions:

| Go export | Swift sees | Purpose |
| --- | --- | --- |
| `BridgeVersion() int` | `Int` | The highest `bridge_version` this engine speaks. |
| `Call(request string) string` | `String -> String` | One JSON request in, one JSON response out. |

`Call` never panics across the boundary and never returns an empty string.
Engine failures come back as error responses. Typed Swift wrappers encode
requests and decode responses. No Go type crosses the boundary.

## Request

```json tt:bridge-request
{
  "bridge_version": 1,
  "request_id": "5d1c1f8e-2a8b-4f0e-9c55-7f0f3b2d9a10",
  "op": "command.transition",
  "workspace": "8f58ae6c-06ce-4dc0-96a2-cd2f533096c4",
  "expect": {
    "projects": { "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01": "3b1f…" },
    "domains": { "planning": "9a4c…" }
  },
  "payload": {
    "task": { "project_id": "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01", "task_id": "0a7e5d10-2c3b-4f6a-8e9d-000000000101" },
    "state": "done"
  }
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `bridge_version` | yes | Must be no greater than `BridgeVersion()`. Otherwise the request fails with `unsupported_version`. |
| `request_id` | yes | Echoed in the response. A command replayed with the same id and the same payload returns the first result and does not commit twice. |
| `op` | yes | One of the operations listed below. |
| `workspace` | every op except `workspace.open` | The workspace id returned by `workspace.open`. A mismatch fails with `workspace_mismatch`. |
| `expect` | mutating ops | Revision preconditions. `projects` maps a project id to the canonical project revision, and `domains` maps `catalog`, `planning`, or `lenses` to a raw artifact revision (`storage.RawArtifactRevision`). Both kinds are bare lowercase hex SHA-256 strings, passed back exactly as the engine returned them. If any of them is stale, the request fails with `stale_revision` and commits nothing. |
| `payload` | per op | The operation's arguments. |

Decoding is strict: an unknown field at any level fails with `invalid_request`.
The app and the engine always ship together, so a strict decoder costs nothing
and catches drift between them.

### Identity

A task is always addressed by its qualified identity, `{project_id, task_id}`.
The bridge never accepts a bare task id, a short id, or a title as an address.
Short ids are local to one project, and titles are not unique. When a qualified
identity is written as one string, it uses the `depends` locator form,
`<project-id>:<task-id>`.

## Response

```json tt:bridge-response
{
  "bridge_version": 1,
  "request_id": "5d1c1f8e-2a8b-4f0e-9c55-7f0f3b2d9a10",
  "ok": false,
  "error": {
    "code": "stale_revision",
    "message": "project Atlas Launch changed since it was read",
    "retryable": true,
    "details": { "project_id": "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01", "current_revision": "77e0…" }
  }
}
```

A response carries exactly one of `result` (when `ok` is true) or `error` (when
`ok` is false). A successful mutation's `result` always includes `committed`,
which holds the local transaction id and the new revisions of every artifact the
commit touched. The client uses those as its next `expect`.

### Error codes

| Code | Retryable | Meaning |
| --- | --- | --- |
| `invalid_request` | no | Malformed JSON, an unknown field, or a missing required field. |
| `unsupported_version` | no | The request's `bridge_version` is higher than this engine supports. |
| `unknown_op` | no | The op is not part of this bridge version. |
| `workspace_mismatch` | no | The request names a workspace other than the open one. |
| `not_open` | no | No workspace is open. |
| `not_found` | no | A qualified identity, project, note, or comment does not resolve. |
| `validation` | no | A rule in `internal/models` or `internal/app` refused the change. The message says why. |
| `refused` | no | Policy refused the change, for example an archived owner, a terminal parent, or the protected Unfiled project. |
| `stale_revision` | yes | An `expect` precondition did not match. Re-read and retry. |
| `conflict` | no | A three-way merge recorded a conflict. `details.conflict_id` names it. |
| `locked` | yes | Another writer holds the workspace commit lock. |
| `migration_required` | no | The workspace schema is older than this engine, and opening it would migrate. |
| `schema_too_new` | no | The workspace was written by a newer engine. |
| `io` | yes | A filesystem error. The message names the path. |
| `internal` | no | A bug. The engine logs it with `request_id`. |

## Operations

Version 1 defines these operation families. Each one goes through the shared
headless runtime and calls the same `internal/app` use case the desktop calls.
There is no bridge-only mutation path.

| Op | Mutates | Notes |
| --- | --- | --- |
| `workspace.open` | may recover | Payload is `{data_dir, runtime_dir, device_id, create_if_missing, time_zone}`; `create_if_missing` bootstraps an empty directory and `time_zone` names the device zone, both optional. Recovers interrupted journals and refuses to migrate implicitly. |
| `workspace.join` | yes | Builds and opens this device's working copy of an enrolled workspace from exchange history its transport staged. |
| `workspace.status` | no | Reports schemas, revisions, and the sync state summary. |
| `workspace.close` | no | Releases locks and caches. |
| `query.projects` | no | Returns active, parked, and archived projects as separate populations, matching `TaskWorkspaceProjects` and `AvailableProjects`. |
| `query.tasks` | no | Payload is `{mode, scope, query, sort}` in the canonical query grammar. |
| `query.task` | no | Returns one task with its Peek document fields, including computed dependents. |
| `query.lenses` | no | The saved lenses in stored order. |
| `query.capture_preview` | no | Reads capture grammar text without committing: what each line would create, or each refused line. |
| `query.notes`, `query.note` | no | Notes are location-qualified. Archived projects are excluded. |
| `command.capture` | yes | Capture grammar text, or one structured task. Multiline input is one transaction. |
| `command.edit` | yes | Structured field edit of one qualified task, through the inline editor's use case. |
| `command.transition` | yes | Lifecycle state change through the single lifecycle primitive. |
| `command.plan` | yes | Today and Next membership. |
| `command.check_in` | yes | Sets, clears, or acknowledges a check-in (`set`, `clear`, `acknowledge`). |
| `command.checklist` | yes | Adds, edits, toggles, deletes, or promotes a checklist item. |
| `command.comment` | yes | Task or project comment: `add`, `edit`, `delete`. |
| `command.note` | yes | Creates, edits, moves, or deletes a note. |
| `command.attachment` | yes | Adds or removes an attachment. Bytes travel as a file path under `runtime_dir`, never inline. |
| `command.undo`, `command.redo` | yes | Creates a new checked change. It never restores images over remote edits. |
| `sync.status` | no | Reports local-save state, pending outbound transfers, last successful exchange, and conflicts. Each one is reported separately. |
| `sync.export_pending` | no | Lists outbound manifests and blobs not yet published. |
| `sync.import` | yes | Applies incoming manifests whose blobs and parents are all present. |
| `sync.acknowledge` | yes | Records that the transport durably published an outbound manifest. |
| `conflict.list`, `conflict.show` | no | Durable conflicts. |
| `conflict.resolve` | yes | Resolves a conflict as a new transaction. |

## Served operations

`internal/bridge` implements this contract and `mobile/ttbridge` binds it. An op
listed above that a build does not serve yet answers `unknown_op` with
`details.reserved` set to `true`, so a client can tell an op the build lacks
from one the protocol never had. The current build serves these:

| Op | Payload | Result |
| --- | --- | --- |
| `workspace.open`, `workspace.status` | as above; status takes none | `{workspace, data_dir, device_id, schemas, revisions}` |
| `workspace.join` | `{data_dir, runtime_dir, device_id, workspace_id, exchange, inbox_dir, time_zone}`; `data_dir` and `inbox_dir` absolute, `time_zone` optional | the status fields and `import: {results}` |
| `workspace.close` | none | `{closed}` |
| `query.projects` | none | `{active, parked, archived}` |
| `query.tasks` | `{mode, scope, query, sort: {field, ascending}, lens, search}`, each optional; `query` is the strict canonical clause string; `lens` excludes the four before it | `{query, items: [{task, today, next}], diagnostics}` |
| `query.task` | `{task}` | `{task, project, revision, relations: {ancestors, children, dependencies, dependents}, today, next}` |
| `query.lenses` | none | `{lenses: [{id, name, mode, scope, query, sort}]}` |
| `query.capture_preview` | `{text, project_id}`; `project_id` optional | `{project_id, lines: [{line, title, project_id, project_name, destination, plan, state, priority, tags, context, due, defer, pinned, review, parent_id, children, dependencies, attachments, comments}], errors: [{line, column, message}]}` |
| `command.capture` | `{text, project_id}`, or `{title, project_id, description, resume_note, destination, priority, tags, context, due, defer, pinned, review}` with `title` required | `{task, tasks, committed}` |
| `command.edit` | `{task, title, description, resume_note, priority, tags, context, pinned, review, due, defer}`; only `task` is required | `{task, changed, committed}` |
| `command.transition` | `{task, state}` | `{task, committed}` |
| `command.plan` | `{task, action}`; `action` is `add_today`, `remove_today`, `add_next`, `remove_next`, or `backlog` | `{task, committed}` |
| `sync.status` | none | `{device_id, indexed, heads, outbox: [{txn_id, device_seq}], received, conflicts}` |
| `sync.export_pending` | none | `{device_id, pending: [{txn_id, device_seq, manifest, blobs}], external}`; each file is `{path, source, digest, size}` |
| `sync.import` | `{inbox_dir}`, absolute | `{results: [{path, txn_id, device_id, device_seq, outcome, reason, local_txn_id, missing, unavailable}]}` |
| `sync.acknowledge` | `{txn_ids}` | `{acknowledged}` |

`command.capture` with `text` reads every nonblank line through the capture
grammar exactly as the desktop composer does: the same parser
(`internal/capturetext`), the same line-to-request builder, and the same root
default of Inbox, so a line the desktop would refuse is refused here. Every
line commits in one transaction. A refused line commits nothing and answers
`validation` with `details.lines: [{line, column, message}]`, the column
zero-based in runes. `text` takes no structured field beside `project_id`. A
structured capture's `title` is literal, so a sigil in it stays text. `due` and
`defer` are read with the grammar's own date readers (`2026-10-21`,
`2026-10-21 14:30`, `tomorrow`, `+3d`; `defer` also takes `+4h`). A deadline
falling today routes to Today, as it does on the desktop. An absent
`project_id` is the protected Unfiled project. `tasks` lists every created
task in line order, and `task` is the first.

`query.capture_preview` answers a draft rather than failing on it: a refused
draft returns ok with its `errors` and no `lines`. `plan` is `today` or `next`
for a planning destination; `state` is the status the task would hold.

`command.edit` applies every field it names as one transaction through
`EditTaskLine`, the inline editor's use case, so each field passes that
surface's validation. An absent field keeps the stored value. An empty
`description`, `resume_note`, `context`, `due`, or `defer` clears it, and
`tags` is the complete new set. An edit that changes nothing commits nothing
and reports `changed` false. A deferral moves the task to Backlog, as the
desktop's `@@` does.

`command.plan` is the planning boundary the desktop's workflow chord uses.
Today and Next are mutually exclusive, a terminal task joins neither, and
`backlog` leaves both. Ordering is not served yet.

A `validation` refusal from the shared engine carries what it judged in
`details`: `kind` is `invalid_input` with `field` and `reason`,
`invalid_transition` with `from`, `to`, and `reason`, or
`incomplete_children` with `short_id` and `children: [{short_id, title,
state}]`. A refusal inside a capture batch also names its `line`.

`query.tasks` reads `search` the way the desktop's search box does. A value
the grammar recognizes but cannot honor (`$ready`) refuses the query with
`validation`, naming it. Every canonical token becomes its clause, and the
rest is one free-text clause over the task's own prose. `lens` reopens a
saved lens: its mode, scope, sort, and stored query, read the same way, so
`search` can narrow it further. A lens whose scoped project is missing,
archived, or parked answers `not_found`, like any scope outside the
population.

A task view also carries `resume_note`, `next_check_in`, `checklist`
(`[{text, done}]` in stored order), `comment_count`, `attachment_count`, and
`dependency_count`. Each `relations` entry is
`{project_id, task_id, available, title, state, project_name}`: ancestors
root first, direct children in stored order, the task's own dependencies,
and the computed dependents. A reference the workspace cannot resolve, such
as a deleted task or one in an archived project, keeps its stored locator and
arrives with `available` false. It is never dropped.

`workspace.join` is the iPhone's onboarding join. Swift's transport performs
the coordinated reads of the exchange folder and stages its history into
`inbox_dir`. The engine imports the genesis and everything after it into an
empty `data_dir` through the verified import path, records `exchange` in the
device-local enrollment record, and opens the working copy. It never writes
the desktop's active-workspace pointer. A device that already publishes to
the exchange, a non-empty `data_dir`, or an inbox that never delivered the
workspace answers `refused` and leaves no workspace open.

`sync.export_pending` first records any edit made outside a commit (see
[External changes](sync-envelope.md#external-changes)), so the list includes
it. `sync.status` reports the engine's side only. Whether the exchange is
reachable, and when it last succeeded, is the transport's to report (see
[Apple transport](sync-envelope.md#apple-transport)).

`workspace.open` and `workspace.join` take an optional `time_zone`, the
device's IANA zone. The engine adopts it as the process's local time, because
a due date is a local calendar day and Today is a local day, and an iOS
process inherits no zone and would otherwise run in UTC. The binding embeds
the zone database. A zone it cannot resolve answers `invalid_request`.

`committed` is `{transaction_id, projects, domains}`: the project revisions the
commit produced and every domain artifact whose raw revision moved.

## Runtime configuration

The engine reads nothing from the environment on iOS. `workspace.open` supplies
everything:

- `data_dir`: the workspace root in the app container.
- `runtime_dir`: the machine-local root that replaces `~/.tt/runtime/<workspace-id>/`.
- `device_id`: a stable UUID owned by the app and stored in the keychain. It replaces the hostname and `HOME` hash that `transaction.Manifest.Machine` uses on the desktop.

Tests also inject a clock and an id generator through the same configuration
struct on the Go side. The bridge does not expose either one.

## Versioning

`bridge_version` is a single integer. Within a version, adding an optional
request field is allowed because the app and the engine ship together. Adding a
response field is also allowed, but the Swift decoders must tolerate unknown
response fields. Removing or renaming a field, or changing its meaning, requires
a new version, and the engine then answers both versions until the app drops
the old one.
