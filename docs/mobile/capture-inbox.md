# Capture inbox and widget snapshot

The iPhone app shares one App Group container,
`group.io.github.sglavoie.docket`, with its share extension, its widgets, and the
shortcuts it serves. The container holds two things: a durable inbox the
extension and shortcuts save captures into, and a snapshot the app writes for
its widgets. Neither one is the workspace. Only the app's engine changes the
workspace, so an extension never opens it, never locks it, and never races the
app for it. `ios/DocketShared` holds the Swift side of both and
`internal/bridge/capture_inbox_ops.go` and `internal/bridge/widget_ops.go` the
engine side.

```text
<container>/captures/.staging/<id>/   a save in progress; never imported
<container>/captures/<id>/capture.json
<container>/captures/<id>/files/<n>-<name>
<container>/captures/<id>/status.json  the engine's reason it was not imported
<container>/widget/snapshot.json
```

## Saving a capture

A capture is one `tt-capture/v1` envelope. Its `id` is a lowercase UUID and
names its directory.

```json
{
  "format": "tt-capture/v1",
  "id": "6f1d2c3b-4a5e-4f60-8172-839405a6b7c8",
  "workspace": "806a60c1-081b-420d-a4a8-7733b6201f71",
  "created_at": "2026-10-06T08:00:00.000Z",
  "source": "share",
  "title": "Read: release checklist",
  "note": "Shared from Safari",
  "urls": ["https://example.com/release"],
  "files": [{"path": "files/1-scan.pdf", "filename": "scan.pdf", "digest": "sha256:…", "size": 30000}]
}
```

It holds either `text` or a `title`. `text` is capture grammar, read line by
line as the composer's Quick mode reads it, which is what a shortcut sends. A
`title` is literal, with an optional `note`, which is what a share sends.
`urls` and `files` land on the first task the capture creates. `workspace` is
the workspace the extension last saw in the widget snapshot. It may be absent,
and a capture saved for another workspace is left alone.

A writer copies every file into `.staging/<id>/files/`, hashing as it copies,
writes the envelope last, syncs each file, and publishes the entry with one
rename into `captures/`. An extension terminated at any point before that
rename leaves nothing the importer reads. A retry with the same id replaces its
own staging copy. Saving an id that is already published with the same content
changes nothing.

## Importing

`capture.import {captures_dir}` imports every published capture, oldest first,
and answers `{imported: [{capture_id, outcome, tasks}], pending: [{capture_id,
reason, code, message, lines}], transaction_ids}`. The app calls it when it
opens a workspace, when it comes to the foreground, on a pull to refresh, and
after a shortcut saves.

Each capture imports exactly once, however often it is delivered and wherever
a termination cuts an import short:

- Its tasks take identities derived from the capture's id (UUID version 8 over
  `sha256("tt-capture-task/v1", id, index)`), through `CaptureRequest.TaskID`.
  An import that finds them knows the earlier commit landed and answers
  `resumed` rather than creating them again.
- Its links ride the capture's own transaction. Each file is a further
  transaction through the same `ImportAttachment` path the app's own attachment
  imports take, skipped when those bytes are already attached under that name.
  The workspace never records the container path.
- A receipt, `<runtime_dir>/capture-receipts/<id>.json`, is written before the
  entry is removed, so a capture delivered again after its import answers
  `already_imported` and creates nothing. The entry is renamed under `.staging`
  and then deleted, so a removal cut short never reads as a capture still being
  written.

A capture the engine refuses commits nothing and stays in the inbox. Examples
are a grammar refusal, a damaged file, and nothing to create. `status.json`
records the refusal beside it, and the next import retries it. The app lists
every pending capture at the top of the Inbox as waiting to be added, or with
its refusal. The person can correct a refused capture there, which clears its
status, or discard it. The importer sweeps a staging copy no writer has
touched for a day.

## Widget snapshot

`query.widget_snapshot {limit}` answers Today and Inbox as the app's own tabs
evaluate them, at most `limit` rows each (0 is 8, at most 20):

```json
{
  "format": "tt-widget-snapshot/v1",
  "workspace": "806a60c1-081b-420d-a4a8-7733b6201f71",
  "generated_at": "2026-10-06T09:00:00Z",
  "day": "2026-10-06",
  "valid_until": "2026-10-07T04:00:00Z",
  "today": {"count": 5, "tasks": [{"project_id": "…", "task_id": "…", "short_id": 4, "title": "Ship beta build", "project_name": "Atlas Launch", "state": "doing", "priority": 1, "overdue": false, "pinned": false}]},
  "inbox": {"count": 1, "tasks": []}
}
```

The app writes it to `widget/snapshot.json` after every change it observes and
reloads the widgets only when the bytes changed. The snapshot is a cache, so a
widget shows it with its age. It is current for three hours after
`generated_at`. After that it is stale: the rows are still shown, with how old
they are, since another device may have changed them. At `valid_until`, the
next local midnight, it expires. Today and every overdue mark then describe a
day that is over, so the widget hides the rows and asks for tt to be opened.
The widget's timeline carries an entry at each of those two moments, so it
turns on time without the app running. The Inbox widget also counts the
captures still waiting in the inbox.

## Links

Widgets, shortcuts, and notifications open the app through the `tt` scheme:
`docket://task/<workspace>/<project_id>/<task_id>`, `docket://view/today`,
`docket://view/inbox`, and `docket://capture`. A task link carries its workspace. The
app refuses a link for a workspace other than the open one, rather than
looking the task up somewhere else.
