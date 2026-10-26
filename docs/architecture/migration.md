[← Architecture index](../architecture.md)

## Snapshot-first schema migration

Workspace recovery first runs the schema-24 layout activation, which moves a
pre-v24 internal `.tt/` to `.docket/` at whatever schema it holds, because every
reader now looks under `.docket/` (steps 40–41). It then reads the catalog
payload schema before ordinary loading.
Schema 24 proceeds directly. A supported schema-4 split-domain workspace first
crosses the schema-5 boundary, a supported schema-5 workspace then enters the
schema-6 consolidation path, a schema-6 workspace crosses the schema-7
scope-folding path, a schema-7 workspace crosses the schema-8 tag path, a
schema-8 workspace crosses the schema-9 state-clock path,
a schema-9 workspace crosses the schema-10 defer-rename path, a schema-10
workspace crosses the schema-11 defer-instant path, a schema-11 workspace
crosses the schema-12 task-context path, a schema-12 workspace crosses the
schema-13 defer-count path, a schema-13 workspace crosses the schema-14
due-time path, a schema-14 workspace crosses the schema-15 soft-dependency
path, a schema-15 workspace crosses the schema-16 portable-lens path, a
schema-16 workspace crosses the schema-17 stored-review path, a schema-17
workspace crosses the schema-18 resume-note path, a schema-18 workspace crosses
the schema-19 project-comment path, a schema-19 workspace crosses the schema-20
checklist-origin path, a schema-20 workspace crosses the schema-21 check-in
path, and a schema-21 workspace, or a working copy enrolled in sync at schema
22, crosses the schema-23 hidden-flag path, and a schema-23 workspace finishes
on the schema-24 layout path:

1. Decode the v4 catalog, planning, settings, automation, machine state, task
   Markdown, completed archives, notes, blockers, and legacy indexes with
   migration-only DTOs.
2. Build and validate the complete schema-5 candidate graph, including notes,
   hierarchy, ShortID repair, dependencies, planning references, and every
   canonical output byte. Corrupt or future-schema input stops here without
   creating a snapshot or changing workspace data.
3. Under the workspace commit lock, publish an immutable rollback pair:
   `.docket/backups/pre-simplification-v5-<UTC>.tar.gz` and the matching
   `.audit.md`. The archive contains portable config, machine state, project
   metadata, task Markdown/archives, and notes. Attachment binaries stay in
   place and are checksum-inventoried in the audit.
4. Revalidate the locked inventory, then atomically publish schema-5 domains,
   notes, and task documents while removing retired settings, automation,
   completed archives/indexes, blockers, trash, Sandbox/Internal content,
   per-project undo, and task backup artifacts.
5. Recover or roll forward the prepared activation journal before any later
   migration inspection.
6. Validate the complete schema-6 graph before changing bytes. This conversion
   canonicalizes the protected project as Unfiled, maps priorities to High,
   Medium, and Low, removes retired settings and relationships, converts
   decision text to comments, repairs Today/Next references, preserves notes and
   attachment bytes, and plans the incompatible undo ledger for removal.
7. Publish a second immutable `pre-consolidation-v6-<UTC>.tar.gz` plus matching
   audit, revalidate the exact locked inventory, and atomically activate schema
   6 across portable and machine-state roots.
8. Validate the complete schema-7 catalog before changing bytes. This conversion
   folds the two retired per-project grouping fields and the retired workspace
   scope table into ordinary project tags, discards the scope definitions, and
   assigns the reserved `parked` tag only where a retired name resolved to it.
   Names a tag cannot carry are sanitized or dropped, collisions gain a
   deterministic numeric suffix, and an unparked project whose retired name reads
   `parked` is renamed rather than silently parked. An undefined scope reference
   stops the conversion.
9. Publish a third immutable `pre-scope-v7-<UTC>.tar.gz` plus matching audit,
   revalidate the exact locked inventory, atomically activate schema 7 in the
   portable catalog, and delete the stale schema-6 catalog sidecar the loader
   would otherwise recover from.
10. Validate every schema-7 project document and catalog tag, canonicalize task
   and project tags to lowercase ASCII kebab form, and audit renames/collisions.
11. Publish `pre-tag-v8-<UTC>.tar.gz` plus its audit, atomically activate catalog
   schema 8 and task Markdown schema 7 across active and archived projects, and
   clear undo/redo history that retains pre-canonical bytes.
12. Validate every schema-7 project document, then lift it to schema 8 by
   backfilling each task's `state_changed` clock from its `updated` timestamp.
13. Publish `pre-state-clock-v9-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 9 and task Markdown schema 8 across active and archived
   projects, and clear undo/redo history whose retained bytes predate the new
   field. The audit records that the backfill is an approximation, because
   `updated` is the last touch of any field; it self-corrects on the task's next
   state transition.
14. Validate every schema-8 project document, then lift it to schema 9 by
   renaming each task's durable deferral metadata key from `snoozed` to
   `deferred`. The key is optional, so the audit counts only the tasks that
   actually carry a date.
15. Publish `pre-defer-rename-v10-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 10 and task Markdown schema 9 across active and
   archived projects, and clear undo/redo history whose retained bytes are raw
   pre-rename Markdown.
16. Validate every schema-9 project document, then lift it to schema 10 by
   rewriting each task's durable deferral value from a `YYYY-MM-DD` calendar day
   to the canonical RFC3339Nano instant that day began at in the migrating
   machine's local timezone, which is the location a bare date was always
   implicitly read in, so no deferral changes the moment it expires.
17. Publish `pre-defer-instant-v11-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 11 and task Markdown schema 10 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   old calendar-day value. The audit states that the migrating machine's offset
   is frozen into the file, so a workspace synced across timezones stops waking
   at each machine's own midnight. No down-migration exists: recovery is manual
   extraction of that archive into a separate workspace.
18. Validate every schema-10 project document, then lift it to schema 11 by
   adding the optional task-level `context` field with an empty value on every
   existing task. This new field is distinct from the retired v4 project-level
   context that schema 7 folded into project tags.
19. Publish `pre-context-v12-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 12 and task Markdown schema 11 across active and archived
   projects, and clear undo/redo history whose retained bytes carry the
   schema-10 task-document marker.
20. Validate every schema-11 project document, then lift it to schema 12 by
   adding the optional `defer_count` field. Existing tasks begin at zero, whose
   canonical representation omits the field.
21. Publish `pre-defer-count-v13-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 13 and task Markdown schema 12 across active and
   archived projects, and clear undo/redo history whose retained bytes predate
   the new task-document marker.
22. Validate every schema-12 project document, then lift it to schema 13 by
   adding the optional `due_time` field. Existing due dates are calendar-only,
   so every migrated task keeps its date and receives no `due_time`.
23. Publish `pre-due-time-v14-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 14 and task Markdown schema 13 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-12 task-document marker.
24. Validate every schema-13 project document, then lift it to schema 14 by
   adding the optional `depends` list of soft task references. No retired field
   carries dependency data across this boundary, so every migrated task begins
   with an empty list, whose canonical representation omits the key.
25. Publish `pre-dependencies-v15-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 15 and task Markdown schema 14 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-13 task-document marker.
26. Validate every schema-14 project document and lift none of them. Schema 16
   moves named lens definitions into the workspace, which changes no task
   grammar, so this is the one boundary whose source scan exists purely to
   refuse a workspace this build cannot read.
27. Publish `pre-portable-lenses-v16-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 16 over unchanged schema-14 task Markdown, and clear
   undo/redo history, whose entries snapshot `.docket/workspace.json` beside task
   Markdown and would otherwise walk the catalog back across the boundary.
28. Validate every schema-14 project document, then lift it to schema 15 by
   moving review off the tag list and onto the stored `review` flag. A task
   carrying the `review` tag arrives flagged and loses that tag as it is lifted,
   because a workspace holding both would let the next tag rewrite disagree with
   the lane; every other tag is preserved in place and an unflagged task
   round-trips byte-identical, the key being omitted when the flag is off.
29. Publish `pre-stored-review-v17-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 17 and task Markdown schema 15 across active and
   archived projects, rewrite the portable `.docket/lenses.json` collection onto the
   canonical query shape, and clear undo/redo history whose retained bytes carry
   the schema-14 task-document marker.
30. Validate every schema-15 project document, then lift it to schema 16 by
   admitting the optional `RESUME` block that carries one resume note. No
   retired field carries a note across this boundary, so every migrated task
   arrives without one, and a task without one writes no block: each document
   comes out byte-identical to its schema-15 source apart from the marker line.
31. Publish `pre-resume-note-v18-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 18 and task Markdown schema 16 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-15 task-document marker. The portable lens collection is deliberately
   not restaged: a resume note is prose on one task rather than a value a query
   narrows by, so no saved lens changes what it selects.
32. Validate every schema-16 project document, then lift it to schema 17 by
   admitting the optional `PROJECT-COMMENTS` block that carries a project's own
   comments. No retired field carries a comment across this boundary, so every
   migrated project arrives with none, and a project with none writes no block:
   each document comes out byte-identical to its schema-16 source apart from the
   marker line.
33. Publish `pre-project-comments-v19-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 19 and task Markdown schema 17 across active,
   parked, and archived projects, and clear undo/redo history whose retained
   bytes carry the schema-16 task-document marker. The portable lens collection
   is deliberately not restaged, for the same reason the previous boundary did
   not restage it: a project comment is prose on a project rather than a value a
   task query narrows by.
34. Validate every schema-17 project document, then lift it to schema 18 by
   admitting the optional checklist-origin line that records where a promoted
   task came from. No retired field carries an origin across this boundary, so
   every migrated task arrives without one and each document comes out
   byte-identical to its schema-17 source apart from the marker line.
35. Publish `pre-checklist-origin-v20-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 20 and task Markdown schema 18 across active, parked,
   and archived projects, and clear undo/redo history whose retained bytes carry
   the schema-17 task-document marker. The portable lens collection is not
   restaged: a checklist origin does not change query matching.
36. Validate every schema-18 project document, then lift it to schema 19 by
   admitting the optional `check_in` date, its `check_in_repeat` cadence, and
   the `reviewed` instant, and seed a one-off check-in on every Waiting task
   that carries a due date and holds no check-in yet. The seed is a copy of the
   stored canonical `YYYY-MM-DD` due value, so it is a string copy that cannot
   move a deadline: `due`, `due_time`, both timestamps, the description, the
   comments, and every other field are written back as they were read. No repeat
   and no review instant are invented, because a cadence and a last-review
   timestamp are claims the source workspace never made. `SeedWaitingCheckInsV19`
   is that one mint and it skips a task that already holds a check-in, so the
   lift composed with it is idempotent; `V19ProjectDocumentFromV18` stays a pure
   marker lift and its test asserts it invents nothing.
37. Publish `pre-check-ins-v21-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 21 and task Markdown schema 19 across active, parked, and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-18 task-document marker. The plan iterates the catalog's projects
   unfiltered for that reason: a parked or archived project is catalog-owned, so
   leaving it behind would strand a document the new reader rejects. The audit
   reports how many Waiting tasks were seeded, names the three new keys, and is
   the only place that can explain the emptied history. The portable lens
   collection is not restaged: existing `=review` and Stale lenses keep their
   meanings. The additive Needs Review template uses `?review`; it does not
   rewrite a saved lens or turn its stored flag query into scheduled attention.
38. Validate every schema-19 project document, then lift it to schema 20 by
   admitting the optional `hidden: true` metadata key. No task was hidden before
   this boundary, so `V20ProjectDocumentFromV19` is a pure marker lift and each
   document comes out byte-identical to its schema-19 source apart from the
   marker line. The source catalog is schema 21, or schema 22 for a working copy
   enrolled in sync; both hold schema-19 task Markdown, and the enrolled copy's
   `sync` block is carried across untouched.
39. Publish `pre-hidden-v23-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 23 and task Markdown schema 20 across active, parked, and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-19 task-document marker. The audit names the source catalog schema.
   There is no schema-22 step: 22 was the enrolled marker, never a link in the
   chain, and from schema 23 the `sync` block alone marks enrollment, since
   every older Docket already refuses schema 23. The portable lens collection is
   not restaged: no lens could name the hidden flag before this schema.
40. `ActivateV24Layout` runs ahead of everything else in `RecoverWorkspace`.
   It recovers the `.tt/transactions/` journal and refuses while any manifest
   or quarantine remains there, since manifests name `.tt/` paths; it refuses a
   symlink or other non-regular file under `.tt/`; and it publishes
   `pre-docket-layout-v24-<UTC>.tar.gz` plus its audit into `.tt/backups/`,
   holding every byte of `.tt/` but the journal and the backups. Under the
   legacy commit lock it rechecks that `.tt/` still matches the archive,
   assembles `.docket-v24-stage/` from it byte for byte — undo excepted, whose
   entries name absolute `.tt/` paths — with `backups/` copied as it is, syncs
   and re-verifies every file, and renames the stage to `.docket/`. That rename
   is the commit point. It then renames `.tt/` to `.tt-v24-retired/`, puts the
   tombstone in its place through `.tt-v24-tombstone-stage/`, and removes the
   retired directory. The tombstone is a catalog declaring schema 24 under the
   workspace's identity, a `planning.json` note, and an empty
   `transactions/commit.lock`: an older build opens the lock, finds a complete
   domain set, and refuses the schema as newer than it supports, writing
   nothing; a lone catalog would read as a partial workspace instead. The
   catalog keeps its schema through the move, so the chain above then runs
   inside `.docket/`. One process moves a workspace: the move runs under
   `.tt/v24-activation.lock`, so a second opener waits and then settles what it
   finds, and the mover locks the stage's `transactions/commit.lock` before the
   rename, which makes it `.docket/`'s commit lock — the one every settle and
   tombstone write takes — for the rest of the move.
41. `MigrateWorkspaceToV24` closes the chain: a schema-23 catalog advances to 24
   in one transaction with its expected revision and the undo ledger's cleanup.
   Only that number changes; no task grammar, lens, or conflict bundle moves.
   Bootstrap, enrollment, and join give a workspace born at schema 24 its own
   tombstone, and recovery writes a missing one.

Interrupted activation recovery reads the two directories. A stage of either
kind is never authoritative and is deleted on sight. No `.docket/` and a live
`.tt/` is a crash before the commit point: start over, publishing a fresh
archive. `.docket/` with a live `.tt/` is a crash after it: when `.docket/`
still holds every byte the move carries, backups included, `.tt/` is retired
and deleted, and otherwise — an older build kept writing it while the move
synced — it is kept under `.docket/backups/kept-pre-v24-internal-<UTC>/`
before the tombstone replaces it. `.docket/` with no `.tt/` gets the tombstone
and loses any retired directory. A tombstone with no `.docket/` is a move
another machine made that has not arrived: `WorkspaceMovedElsewhereError`
refuses it, and `WorkspaceCatalogSchema` and `LoadDomains` do the same without
writing.
`internal/synclog` reads a pre-v24 record's `.tt/` paths through
`CanonicalPath`, so sealed history, tracked state, and index records keep
resolving, while `Classify` calls the tombstone device-local.

`planV17LensDefinitions` is why the lens rewrite is part of that boundary rather
than cleanup after it. Schema 15 removes the review tag from every task as it
sets the stored flag, so a lens that narrowed by that tag would silently stop
selecting the rows it was saved for. The collection is decoded as strictly as the
runtime store decodes it, lifted through `NamedLens.UnmarshalJSON` — which is the
one translation both lens homes share — and staged with the revision its source
bytes carried, so it publishes or fails together with the catalog and every task
document. `rewriteV16ReviewTagQuery` covers the second spelling: a saved
free-text `#review` term becomes `=review`, and `-#review` becomes `-=review`,
while every other token passes through untouched. A carrier value this build
cannot translate refuses the whole migration instead of being dropped, because
dropping a facet would widen a lens to select rows it never selected. A workspace
with no `lenses.json` stages nothing: the machine-local collection it falls back
to is lifted on every load and rewritten on that machine's next commit, which is
the only upgrade path a file this transaction does not own can have.

Each boundary is idempotent: repeated schema-24 startup neither reconverts data
nor creates another rollback pair. Corrupt or future input and an existing
`unfiled` project/directory fail before mutation with concrete rename/move guidance.
No boundary rewrites `.docket/conflicts/` (the v24 move copies it byte for byte), so a bundle recorded before one keeps the
documents it was recorded with and the recovery limits
[`persistence-and-transactions.md`](persistence-and-transactions.md) states
apply to it unchanged; nothing removes such a bundle on the workspace's behalf,
and each migration audit says so in its own report.
Snapshot artifacts are application-owned and immutable to Docket. Downgrade or a
clean migration retry requires restoring the complete matching snapshot into a
separate workspace with a compatible build; the schema-6 and schema-7 activation
tests also exercise this restore-and-reactivate path.

An older aggregate `.tt.json` without the supported split-domain workspace is
not implicitly imported. Startup reports that unsupported legacy source before
creating a new workspace beside it.
