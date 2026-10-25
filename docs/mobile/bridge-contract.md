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
| `query.search_suggestions` | no | Completes the final search token using the current list's tags, contexts, projects, dates, and dependency references, plus supported grammar values. |
| `query.task` | no | Returns one task with its Peek document fields, including computed dependents. |
| `query.lenses` | no | The saved lenses in stored order. |
| `query.capture_preview` | no | Reads capture grammar text without committing: what each line would create, or each refused line. |
| `query.capture_suggestions` | no | Completes a capture sigil at the caret using grammar values and workspace vocabulary. |
| `query.notes`, `query.note` | no | Notes are location-qualified. Archived projects are excluded. |
| `command.capture` | yes | Capture grammar text, or one structured task. Multiline input is one transaction. |
| `command.edit` | yes | Structured field edit of one qualified task, through the inline editor's use case. |
| `command.transition` | yes | Lifecycle state change through the single lifecycle primitive. |
| `command.plan` | yes | Today and Next membership. |
| `command.check_in` | yes | Sets, clears, or acknowledges a check-in (`set`, `clear`, `acknowledge`). |
| `command.checklist` | yes | Adds, edits, toggles, deletes, moves, or promotes a checklist item. |
| `command.reorder` | yes | Saves Today, Next, or sibling order in one transaction. |
| `command.hierarchy` | yes | Adds a subtask, sets or clears a parent, indents, outdents, or moves a task among its siblings. |
| `command.bulk` | yes | One planning, status, tag, or project action over a selection, as one transaction. |
| `command.project` | yes | Creates, edits, retags (parking included), archives, restores, or deletes a project. |
| `query.project` | no | One project, archived ones included, with what deleting it would remove. |
| `query.reminders` | no | The local notifications the device should hold, planned under the shared attention rules. |
| `query.widget_snapshot` | no | Today and Inbox as the widgets show them, with the local day they are true for. |
| `capture.import` | yes | Imports the captures a share extension or a shortcut saved to the App Group inbox, each exactly once. |
| `command.comment` | yes | Task or project comment: `add`, `edit`, `delete`. |
| `command.note` | yes | Creates, edits, moves, or deletes a note. |
| `command.attachment` | yes | Adds or removes an attachment. Bytes travel as a file path under `runtime_dir`, never inline. |
| `command.undo`, `command.redo` | yes | Creates a new checked change. It never restores images over remote edits. |
| `sync.status` | no | Reports local-save state, pending outbound transfers, last successful exchange, and conflicts. Each one is reported separately. |
| `sync.export_pending` | no | Lists outbound manifests and blobs not yet published. |
| `sync.import` | yes | Applies incoming manifests whose blobs and parents are all present. |
| `sync.acknowledge` | yes | Records that the transport durably published an outbound manifest. |
| `sync.diagnostics` | no | Status, outbox, conflict structure, and the synced files' digest for a diagnostics export, without file content. |
| `conflict.list`, `conflict.show` | no | Durable conflicts. |
| `conflict.resolve` | yes | Resolves a conflict as a new transaction. |

## Served operations

`internal/bridge` implements this contract and `mobile/docketbridge` binds it. An op
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
| `query.search_suggestions` | Same axes as `query.tasks`; `search` is the current input | `{suggestions: [{token, search}]}`; each `search` completes the final token and appends a space |
| `query.task` | `{task}` | `{task, project, revision, relations: {ancestors, children, dependencies, dependents}, today, next, comments, attachments, attachment_dir}` |
| `query.lenses` | none | `{lenses: [{id, name, mode, scope, query, sort}]}` |
| `query.capture_preview` | `{text, project_id}`; `project_id` optional | `{project_id, lines: [{line, title, project_id, project_name, destination, plan, state, priority, tags, context, due, defer, pinned, review, parent_id, children, dependencies, attachments, comments}], errors: [{line, column, message}]}` |
| `query.capture_suggestions` | `{text, caret, project_id}`; `project_id` optional; `caret` is a UTF-16 offset | `{suggestions: [{token, label, text, caret, selection_length}]}`; each item supplies the replacement draft and UTF-16 selection |
| `command.capture` | `{text, project_id}`, or `{title, project_id, description, resume_note, destination, priority, tags, context, due, defer, pinned, review}` with `title` required | `{task, tasks, committed}` |
| `command.edit` | `{task, title, description, resume_note, priority, tags, context, pinned, review, due, defer}`; only `task` is required | `{task, changed, committed}` |
| `command.undo` | `{transaction_id}`, required; the exact receipt transaction, still newest in this device’s history | `{transaction_id, action}`; refuses a superseded receipt or changed artifact, and records the restore in sync history |
| `command.transition` | `{task, state}` | `{task, committed}` |
| `command.plan` | `{task, action}`; `action` is `add_today`, `remove_today`, `add_next`, `remove_next`, or `backlog` | `{task, committed}` |
| `command.checklist` | `{task, action, text, index, to, checklist}`; `action` is `add` (`text`), `edit` (`index`, `text`), `toggle`, `remove`, `promote` (`index`), or `move` (`index`, `to`); every action but `add` also sends `checklist`, the complete `[{text, done}]` as read | `{task, child, committed}`; `child` is the subtask a promotion made |
| `command.check_in` | `{tasks, action, date, repeat_days}`; `action` is `set` (`date`, optional `repeat_days`), `clear`, or `acknowledge` (exactly one task) | `{tasks, moves, committed}` |
| `command.reorder` | `{task, kind, order}`; `kind` is `today`, `next`, or `siblings`; `order` is at least two unique locators including `task` | `{tasks, moves, committed}` |
| `command.hierarchy` | `{task, action, title, parent}`; `action` is `add_child` (`title`), `set_parent` (`parent`, same project), `clear_parent`, `indent`, `outdent`, `move_up`, or `move_down` | `{task, created, committed}` |
| `command.bulk` | `{tasks, action, plan, state, add_tags, remove_tags, project_id}`; `action` is `plan` (`plan` as in `command.plan`), `state`, `tags` (`add_tags`, `remove_tags`), or `move` (`project_id`) | `{tasks, moves: [{from, to}], committed}` |
| `query.project` | `{project_id}` | `{project, impact: {tasks, notes, attachments, digest}, comments}` |
| `query.reminders` | `{time_zone, check_in_time, horizon_days, limit}`; `time_zone` (IANA) and `check_in_time` (`HH:MM`) required, `horizon_days` 0-366 (0 is 30), `limit` 0-64 (0 is 60) | `{time_zone, check_in_time, generated_at, truncated, reminders: [{id, kind, task, project_name, short_id, title, fire_at, local_time, can_acknowledge, can_defer, fingerprint}]}`, soonest first |
| `query.widget_snapshot` | `{limit}`, optional, 0-20 (0 is 8) | `{format, workspace, generated_at, day, valid_until, today, inbox}`; each list is `{count, tasks: [{project_id, task_id, short_id, title, project_name, state, priority, due_date, overdue, pinned}]}` |
| `capture.import` | `{captures_dir}`, absolute | `{imported: [{capture_id, outcome, tasks}], pending: [{capture_id, reason, code, message, lines}], transaction_ids}`; `outcome` is `imported`, `resumed`, or `already_imported`; `reason` is `refused`, `other_workspace`, or `incomplete` |
| `command.project` | `{action, project_id, name, description, tags, confirm}`; `action` is `create` (`name`), `edit` (any of `name`, `description`, `tags`), `set_tags` (`tags`, the complete set), `archive`, `unarchive`, or `delete` (`confirm`, the `impact` the confirmation showed) | `{project, committed}` |
| `command.comment` | `{task, project_id, action, text, index, comment_id, original}` with exactly one of `task` and `project_id`; `action` is `add` (`text`), `edit` (`text`), or `delete`; `edit` and `delete` name the comment by `index` on a task or `comment_id` on a project, and send `original`, the text read there | `{task, project_id, comment_id, comments, committed}` |
| `query.notes` | `{location}`, optional; a location is `{scope, project_id}`, `scope` `global` or `project` | `{notes: [{location, project_name, filename, title, created_at, modified_at, revision, parse_error, excerpt}]}` |
| `query.note` | `{location, filename}` | `{note}`, the list's fields and `body` |
| `command.note` | `{action, location, filename, revision, title, body, destination}`; `action` is `create` (`title`, `body`), `update` (`body`, optional `title`), `rename` (`title`), `move` (`destination`), or `delete`; every action but `create` names the note and sends `revision`, the revision read | `{note, committed}`; a delete has no `note` |
| `command.attachment` | `{task, action, path, filename, digest, url, index, original}`; `action` is `add_file` (`path`, `filename`, `digest`), `add_url` (`url`), or `remove` (`index`, `original`, the name read) | `{task, attachment, attachments, outcome, committed}`; `outcome` is `added`, `present`, or `removed` |
| `sync.status` | none | `{device_id, indexed, heads, outbox: [{txn_id, device_seq}], received, conflicts}` |
| `sync.export_pending` | none | `{device_id, pending: [{txn_id, device_seq, manifest, blobs}], external}`; each file is `{path, source, digest, size}` |
| `sync.import` | `{inbox_dir}`, absolute | `{results: [{path, txn_id, device_id, device_seq, outcome, reason, local_txn_id, missing, unavailable}]}` |
| `sync.acknowledge` | `{txn_ids}` | `{acknowledged}` |
| `sync.diagnostics` | `{device_names}`, optional | `{format, generated_at, device_id, workspace_digest, status, outbox, conflicts: [{id, path, kind, base, current, versions: [{digest, available, writers}], details}]}` |
| `conflict.list` | `{device_names}`, optional: device id to the name its advertisement gives | `{conflicts: [{id, path, kind, project_id, label, base, current, versions, details, replaceable}]}`; `base` and `current` are `{digest, deleted, size, text, available}`, `base` null when the file did not exist; each version adds `index`, `current`, `writers: [{txn_id, device_id, device_name, this_device, change, created_at, files}]`, and `also_resolves: [{id, path, label}]` |
| `conflict.show` | `{conflict_id, device_names}` | `{conflict}` as listed; a conflict no longer open is `not_found` |
| `conflict.resolve` | `{conflict_id, choice, version, text}`; `choice` is `version` (`version`, the index), `current`, or `replace` (`text`, tasks or a note only) | `{txn_id, resolved: [{id, path, label}], remaining}`; a conflict that closed or changed since it was read is `stale_revision` with `details.conflicts`, the ids open now; an unknown choice or version, or text the readers refuse, is `validation` with `details.field` `resolution` |

`command.capture` with `text` reads every nonblank line through the capture
grammar exactly as the desktop composer does: the same parser
(`internal/capturetext`), the same line-to-request builder, and the same root
default of Inbox, so a line the desktop would refuse is refused here. Every
line commits in one transaction. A refused line commits nothing and answers
`validation` with `details.lines: [{line, column, message}]`, the column
zero-based in runes. `text` takes no structured field beside `project_id`. A
structured capture's `title` is literal, so a sigil in it stays text. `due` and
`defer` are read with the grammar's own date readers (`2026-10-21`,
`2026-10-21 14:30`, `tomorrow`, `+3d`; `defer` also takes `+4h` and an RFC 3339
instant, since a deferral is an instant). A deadline
falling today routes to Today, as it does on the desktop. An absent
`project_id` is the protected Unfiled project. `tasks` lists every created
task in line order, and `task` is the first.

`capture.import` and `query.widget_snapshot` serve the App Group container
the share extension, the widgets, and shortcuts share with the app; the
envelope, the exactly-once rules, and the snapshot's freshness are in
[capture-inbox.md](capture-inbox.md). A capture's tasks pass the same
validation `command.capture` applies, and a refused capture commits nothing
and stays in the inbox beside its `status.json`.

`query.reminders` plans and never writes. It rereads the workspace, so an
import or another device's change is planned at the next call, and it covers
the task workspace population only: parked and archived projects raise
nothing. A reminder's `kind` is `check_in`, `deadline`, or `deferral_ends`,
and each is planned only when the engine's own rules say the moment asks for
attention: a check-in on its day at `check_in_time`, exactly when
`Task.CheckInIsDueAt` turns true at that instant, so a check-in already owed,
a deferred one, and finished work raise nothing new; a deadline only when it
carries a time of day and no deferral covers it; a deferral when it lapses,
and a deferral landing on local midnight (a day) at `check_in_time` that
morning. The review flag has no moment and is never a reminder. The request
names its zone and the plan adopts it: check-in days and timed deadlines are
wall-clock values placed in that zone, so a device that changes zone replans
them at the same local time, while a deferral is an instant and keeps it.
`fire_at` is that instant in UTC and `local_time` its wall-clock reading in
`time_zone`. `id` is `kind/project_id/task_id`, stable across reschedules,
and `fingerprint` changes whenever the moment, title, project name, or
offered actions do, so a client replaces a notification rather than keeping
two. `can_acknowledge` is `TaskAllowsExplicitReview` at that moment, which is
when `command.check_in` `acknowledge` would succeed; `can_defer` is set for
open work. A notification's actions are ordinary commands: `command.check_in`
`acknowledge`, and `command.edit` with `defer` `+1h` or `tomorrow`.

`query.capture_preview` answers a draft rather than failing on it: a refused
draft returns ok with its `errors` and no `lines`. `plan` is `today` or `next`
for a planning destination; `state` is the status the task would hold.

`query.capture_suggestions` replaces the active token, preserving surrounding
text and other lines. Parent and child suggestions use the current line's
`%project` or the selected source project; dependencies can cross projects.
Comments and quoted or escaped attachment paths protect their embedded sigils.
All capture sigils are supported: links offer existing URLs and URL prefixes;
comments offer a selected placeholder or a closing delimiter. Clients discard
replies after the text, selection, or source project changes and apply a
suggestion only while its originating draft and selection still match.

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

The organizing ops call the use cases the desktop's keys call, so the shared
engine judges both. A checklist item has no stored identity, so an action
naming one by `index` carries the whole checklist as the client read it and
is refused (`validation`, `field` `checklist`) when the stored checklist is
no longer exactly that: an edit, tick, reorder, or import in the meantime
never lands on a different item. `promote` turns the item into a subtask of
its owner in one transaction. `command.check_in` writes one schedule across
its selection; `acknowledge` is the review key, which moves a repeating
check-in on from today, clears a one-off, and changes no state or deadline.
`command.reorder` permutes the named tasks in one transaction. Siblings must
share their project and parent; unnamed siblings keep their stored slots.
Today and Next require existing membership and preserve unnamed members in
place, including parked or hidden tasks. A reorder cannot add planning membership
or change a task's project or parent. `query.tasks` accepts sort field `manual`
to read planning or stored project order without a priority, date, or title key;
its direction is ignored. Other default query orders remain unchanged.

`command.hierarchy` holds the depth limit and refuses a cycle. A parent with
open subtasks refuses completion, alone or in a selection, with
`incomplete_children`. `command.bulk` commits the whole selection or none of
it; `move` takes the single task-line edit for one task and the batch for
more, as the desktop does, and rewrites every reference to a moved task. A
mutation's `committed.projects` names every project whose revision moved,
including those a reference rewrite reached.

`command.project` is the engine's one project lifecycle, the same functions
the desktop's project hub commits through. The system Unfiled project is
never renamed, parked, archived, or deleted (`refused`). `delete` is guarded:
`confirm` must equal the project's current `impact`, whose `digest` covers
the stored task Markdown, so a delete confirmed before more work arrived (from
this device or an import) answers `stale_revision` with the current `impact`
and deletes nothing.

A `validation` refusal from the shared engine carries what it judged in
`details`: `kind` is `invalid_input` with `field` and `reason`,
`invalid_transition` with `from`, `to`, and `reason`, or
`incomplete_children` with `short_id` and `children: [{short_id, title,
state}]`. A refusal inside a capture batch also names its `line`.

`query.tasks` reads `search` the way the desktop's search box does. A value
the grammar recognizes but cannot honor (`$ready`) refuses the query with
`validation`, naming it. Every canonical token becomes its clause. Tag and
context spellings are normalized as on capture, so `#Epic` matches `epic`,
including in negated and explicit prefix searches. The remaining text is one
free-text clause over the task's own prose. `lens` reopens a
saved lens: its mode, scope, sort, and stored query, read the same way, so
`search` can narrow it further. A lens whose scoped project is missing,
archived, or parked answers `not_found`, like any scope outside the
population.

Search suggestions use the same mode, scope, filters, lens, and settled query
prefix as the list, excluding the unfinished final token. Tags and contexts
come from matching tasks; static values come from the search grammar and are
offered only when the bridge can evaluate them. Exact matches, already-used
values, and a trailing space produce no redundant completion. Selecting a
suggestion preserves the preceding query text.

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

Local queries and commands take precedence over queued sync requests. An import
yields between complete incoming transactions so a backlog cannot monopolize
the bridge; each transaction still commits atomically. Closing or reopening the
workspace invalidates the yielding import. The mobile transport attaches to the
already-open bridge by workspace identity rather than opening it again.

`sync.export_pending` first records any edit made outside a commit (see
[External changes](sync-envelope.md#external-changes)), so the list includes
it. `sync.status` reports the engine's side only. Whether the exchange is
reachable, and when it last succeeded, is the transport's to report (see
[Apple transport](sync-envelope.md#apple-transport)).

The rich-content ops call the use cases the desktop's comment panel, Notes
surface, and attachment panel call. A task comment has no stored identity, so
`edit` and `delete` name it by `index` and carry the text read there; a project
comment is named by its `comment_id` and that text. Either is refused
(`validation`, `field` `comment`) when the stored comment no longer holds the
text, so an edit or an import in the meantime never redirects the action. Each
comment arrives as `{index, id, text, created_at}`, `id` on project comments
only, in stored order; a task read and a project read carry them whole.

A note is location-qualified: the global collection, or one available
project's. `query.notes` without a location lists every collection a phone can
reach, newest first; an archived project's notes are not among them, and
naming one answers `not_found`. `revision` is the raw revision of the note
bytes read. An `update`, `rename`, `move`, or `delete` whose `revision` is no
longer the note's answers `stale_revision` with `current_revision` and writes
nothing, so another device's edit is never overwritten unseen. A body is stored
without leading blank lines or trailing whitespace, which is how the note file
reads back, so the body a client saved is the body it reads.

An attachment arrives as `{index, kind, name, url, code_path, line, added_at,
available, size, local_path, workspace_path}`. `kind` is `file`, `url`, or
`code`. Only a file has bytes in the workspace: `available` says whether this
device holds them, `local_path` is where (read-only), and `workspace_path` is
the workspace-relative path the exchange names them by. A task read's
`attachment_dir` is the workspace-relative directory its attachment bytes live
under, so a client can tell which paths its transport reports unavailable
belong to the task. A URL and a code reference are kept whole even though a
phone cannot open a code reference's target.

`add_file` imports bytes the client staged under its `runtime_dir`: a copy it
made of a Files or Photos item while the provider's grant was valid. The path
must resolve, after symlinks, inside `runtime_dir` (`invalid_request`
otherwise), and `digest` is that copy's `sha256:`-prefixed content address; a
copy whose bytes no longer match it, as one cut short by a termination would
not, is refused (`validation`, `field` `attachment`) and nothing is written.
The engine reads the copy into one transaction with the task's metadata, so a
failure commits neither, and it records no original path, so no temporary
provider or staging location is ever kept as the attachment's storage. A name
is made storable (one path component, no attachment delimiter or control
character, never hidden), and a name the task already uses for other bytes
takes a number before its extension. Repeating an import whose bytes are
already attached under that name answers `present` and commits nothing, which
is how a client resumes an import a termination interrupted without knowing
whether its commit landed. `remove` names the attachment by `index` and the
name read, deletes a file's bytes in the same transaction, and works on a file
whose bytes this device does not hold.

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
