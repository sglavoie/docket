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
| `workspace.open` | may recover | Payload is `{data_dir, runtime_dir, device_id}`. Recovers interrupted journals and refuses to migrate implicitly. |
| `workspace.status` | no | Reports schemas, revisions, and the sync state summary. |
| `workspace.close` | no | Releases locks and caches. |
| `query.projects` | no | Returns active, parked, and archived projects as separate populations, matching `TaskWorkspaceProjects` and `AvailableProjects`. |
| `query.tasks` | no | Payload is `{mode, scope, query, sort}` in the canonical query grammar. |
| `query.task` | no | Returns one task with its Peek document fields, including computed dependents. |
| `query.notes`, `query.note` | no | Notes are location-qualified. Archived projects are excluded. |
| `command.capture` | yes | Capture grammar text and a destination. Multiline input is one transaction. |
| `command.edit` | yes | Quick-edit grammar against one qualified task. |
| `command.transition` | yes | Lifecycle state change through the single lifecycle primitive. |
| `command.plan` | yes | Today and Next membership and order. |
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
