[← Architecture index](../architecture.md)

## Persistence layout

The portable workspace rooted at `<data>` is:

```text
<data>/
├── .tt/
│   ├── workspace.json       # lean catalog, project tags, Unfiled ID
│   ├── planning.json        # Today and Next order only
│   ├── global-notes/        # minimal Markdown notes
│   ├── transactions/        # prepared/recovering commit journals
│   ├── undo/                # workspace ledger and retained artifact images
│   ├── conflicts/           # merge bundles
│   └── backups/             # immutable migration rollback pairs
└── projects/
    └── <safe-slug>/
        ├── tasks.md         # strict schema-17 generic Task Markdown
        ├── notes/           # minimal project notes
        └── .attachments/    # task-addressed file binaries
```

`tasks.md` holds the whole project, not only its tasks. Schema 17 adds an
optional `PROJECT-COMMENTS` block between the schema marker and the first task
heading, one `<!-- project-comment: UUID RFC3339Nano -->` header per entry with
its body escaped by the same prose escaping the task blocks use, instants
normalized to UTC, entries ordered by creation time and then identity, and the
block omitted entirely when the project holds none. That order and that zone are
what let two machines holding one collection render one byte sequence, which is
what `ProjectDocument.ComputeRevision` hashes. `models.ValidateProjectComments`
is the single gate — canonical lowercase UUID, non-blank text, non-zero instant,
project-local id uniqueness — reached from `models.AuditProject`, from the block
codec, and from the document validator, so no surface can accept a comment the
canonical writer would refuse.

Because the file is the whole project, `storage.WriteProjectMarkdown` renders a
complete `*models.Project` and is the only production rewrite entry point. The
earlier name-plus-task-list renderer is gone rather than deprecated: a rewrite
site that could spell a project's name and tasks without its comments would drop
them silently, and a project rename, a task edit, an outline edit, and a
cross-project move all rewrite documents they did not otherwise change.
`ProjectDocument.Comments` is detached on every hop — parse, `ToProjectDocument`,
snapshot clone, rollback, and conflict reconstruction — so the model's collection
and the merge base never share one comment object.

Project paths are derived from the selected data directory and canonical
lowercase ASCII slugs. Absolute paths are not catalog data. Every mutation path
is checked against approved roots with symlink-aware containment before locks,
staging files, or destination directories are created.

Portable configuration has three domain files:

- `workspace.json`: workspace identity, lean active/archived project catalog,
  automatic color slots, each project's normalized tag set, and the protected
  Unfiled ID.
- `planning.json`: project-qualified Today and Next ordering.
- `lenses.json`: the named-lens definitions. It is a `domainKind` of its own and
  deliberately outside `syncedDomainKinds`, whose membership decides whether a
  workspace reads as bootstrapped: a workspace with no lens home is complete, not
  partial. `WorkspaceLensDefinitions` distinguishes three states rather than two —
  a nil list is "this workspace defines no lenses" and falls back to the machine
  copy, a non-nil empty list is the durable "every lens was deleted", and
  `SaveLensDefinitionsChecked` refuses an undefined collection so the first can
  never be published as the second.

Machine state contains only the dark/light theme selection and the active-lens
pointer:

```text
~/.tt/runtime/<workspace-id>/machine.json
```

The workspace ID prevents unrelated workspaces from sharing machine state.
`MachineState.NamedLenses` survives as the tolerated legacy input the strict
decoder still accepts and as the fallback an unadopted binary reads;
`DomainFileStore.applyPortableLensDefinitions` is the one seam that decides
precedence, and it is applied on both load paths — `LoadMachineChecked` and
`loadDomainBundleWithRevisions` — because the frontends reach the bundle through
the second. `DomainCoordinator.CommitMachine` mirrors it through
`publishOwnedLensDefinitions`, which forwards the lens half to
`CommitLensDefinitions` when `lenses.json` exists or this machine has adopted,
and writes the machine copy beside it either way.

`Manager.AdoptPortableLenses` is the one-time-per-machine step that joins a
machine to that home. It cannot live in the migration chain, which the synced
catalog version guards: the first machine to reach v16 advances that marker for
every machine sharing the data directory, so a second machine's migration is a
no-op and its lenses would be lost unread. Adoption is therefore keyed off
machine-local `MachineState.LensHomeAdopted` and runs from
`Manager.RecoverWorkspace` after the chain, outside it; `LoadOrCreateDomains`
stamps the marker onto a fresh bundle before `createDomainBundle` instead, so a
brand-new workspace publishes its home in the single machine write bootstrap
already makes. `MergeNamedLenses` never deletes, and it is a join over the union
of both sides rather than a fold in arrival order, so it is commutative,
associative and idempotent over the records it yields. Identity is the id a
record was contributed under together with its core — mode, scope, query and
sort — so two contributions agreeing on both are one lens under two labels; when
one id carries several cores the core that sorts first keeps it and every other
one takes a UUIDv5 derived from that id and that core, reading neither the label
nor the arrival side, which is what lets a record an earlier merge split out
re-derive to the identity it is already stored under instead of splitting again.
`lensIdentityCandidates` is that bounded sequence — the contributed id and one
salted derivation for every salt through `lensIdentitySaltAttempts` — and it is
complete on both ends. Every candidate is checked before it is claimed, the
final derivation included: claiming an unchecked one would settle two cores on
one id and `collapseLensRecords` would then fold them into a single record,
which is the silent deletion the union contract forbids and no undo entry could
take back. Equally, no checkable candidate is withheld — trimming the last
derivation out of the sequence would refuse a merge an available identity could
have settled. A contribution whose every candidate is held by another core
returns `ErrLensIdentityExhausted` and the merge yields no collection at all, so both
adoption and the stale-write reconciliation refuse rather than publish one
definition short. `RemapActiveLensID` walks the same sequence, so the pointer is
followed over exactly the identities the merge can mint.
Names are assigned afterwards over the settled identities, records that kept a
contributed id first and canonical id order within that, because
`NamedLensList.Validate` enforces case-folded name uniqueness as well as id
uniqueness. Element order is the one arrival-bound axis: a record takes the slot
of its earliest contribution, so the workspace file's order leads and carries no
meaning beyond presentation. The active-lens pointer is machine-local while the
definitions are portable, so `RemapActiveLensID` follows it through the same
derivation on both merge paths — adoption and the stale-write reconciliation in
`publishOwnedLensDefinitions` — rather than leaving it on an id another
definition now holds. `LoadPersistedMachineLenses` decodes
into a zero `MachineState` rather than a seeded one, so an omitted `named_lenses`
key contributes nothing and a machine joining later cannot reseed starters into a
workspace whose owner deleted every lens; the founding machine, where neither
home holds a record, is the one exception and hands the workspace the set it
already shows.
Logs and process locks are also machine-local. A fixed machine-local active
workspace pointer may select a non-default data directory before the portable
catalog is loaded.

## Transactions and recovery

The workspace repository is the sole user-mutation persistence boundary. A
commit:

1. validates paths, revisions, artifact bytes, and attachment effects;
2. acquires deterministic locks;
3. writes a prepared journal and staged data;
4. publishes every operation atomically and marks the journal committed; and
5. records one workspace undo transaction.

Startup recovers prepared or committing journals before it classifies or loads
the workspace. Recovery authorizes every transaction root recorded by the
operation; migration publication includes the machine-state root as well as the
portable workspace. A quarantined or malformed journal blocks startup rather
than allowing a partial view.

Each mutation also recovers pending journals and checks quarantine under the
workspace commit lock. It leaves unreferenced staging-file cleanup to full
startup recovery: those files cannot change live data, and walking the whole
workspace on every edit would make save latency grow with retained sync history.
Independent writes share durability barriers before the prepared and committed
markers. Moves, deletes, and overlapping writes retain their ordered barriers.

Because `.tt/transactions/` is portable, recovery can meet a journal that is not
an interrupted transaction at all but another machine's live one, delivered
mid-commit by a sync service. A manifest therefore records who wrote it and when
it was last touched: `Manifest.Machine` is the first `machineIDLength` hex
characters of `sha256(hostname + "\x00" + filepath.Clean(home))`, and
`Manifest.Heartbeat` is refreshed as the transaction proceeds. Both are
`omitempty` and neither advances the journal schema, so a manifest written by a
build that predates them reads as unowned and recovers exactly as it always did.
The identity is recomputed per call rather than cached, which is what makes a
`HOME` switch genuinely a different machine and keeps the harness and the
two-process gate honest.

That per-call identity answers which machine is running now, and a working
copy's sync history asks a different question: whose history this is. So the
desktop derives a sync device identity only for a working copy that has recorded
nothing; afterwards `synclog.RecordedDeviceID` reads it back from
`.tt/sync/devices/`, and a renamed host goes on recording as the device that
enrolled. Deriving it afresh would start a second history beside the first, with
its own genesis and nothing received, so every peer transaction would arrive
"before 1". A working copy already holding several identities answers with the
one that has recorded the most.

`Journal.foreignLive` is the resulting refusal: a manifest naming another machine
whose heartbeat is fresher than `ForeignTransactionGrace` is left untouched —
neither rolled forward, nor rolled back, nor quarantined — and reported as
`RecoveryReport.ForeignLive`. It does not block, because the owner is still
working and will finish it; the alternative is one machine rolling back a commit
another machine is mid-way through publishing. Once the heartbeat goes stale the
transaction is ordinary interrupted work again and recovers on the next launch,
so a machine that dies mid-commit strands nothing permanently. The grace window
is the only clock skew this design tolerates, which is why the runbook in
[`guide/storage.md`](../guide/storage.md) says not to run `tt` on two machines at
once rather than treating that as a supported mode.

The ledger retains at most 50 transactions across undo and redo. `u` and
`Ctrl+R` restore complete artifact images, including project moves, planning,
notes, and attachment topology. Every successful forward mutation clears redo.
Failed operations record nothing. Undo is convenience history in the same
workspace, not an independent backup.

An entry keeps its `Seq` on both stacks. Undo takes the highest on the undo
stack, so redo takes the lowest on the redo stack: the last entry undone, and the
only one whose before-image the file currently holds.

A restore preflights every image and refuses on drift before staging a byte. What
happens to the refused entry depends on whether the ledger can explain the drift.
A scoped restore can select an entry older than one outside its scope that wrote
the same file; that entry is blocked behind the newer one, and the refusal leaves
it in place. Drift no pending entry accounts for came from outside the ledger — a
sync import and an external edit record no entry — so the file can never return
to the image the entry expects. `retireDriftedEntry` drops that entry in a
ledger-only journal transaction and returns `RetiredEntryError`, which unwraps to
the `ArtifactConflictError` naming the file. Left selected, it would have been
chosen by every later restore and walled off all older history behind it.

Deleting a project is the widest single transaction the product commits, and it
is built from the same primitive as every other. A delete op is `os.Remove`: one
entry, and never a non-empty directory. `PlanProjectDelete` therefore enumerates
the project tree and emits one op per path in reverse `WalkDir` order, which is
post-order — deterministic, and with every child named before the directory that
holds it. The catalog and planning rewrites ride in the same commit, so a project
can never be absent from disk while still named in either file. Undo restores the
tree from the images capture took beforehand; because those images cover a
directory that is itself leaving, restore holds one advisory lock per outermost
directory rather than one per image, or creating the nested lock's parent would
recreate the directory preflight is about to find absent.

## Concurrent edits and conflicts

Project revisions hash canonical schema-19 content. A stale writer is compared
with its base and current remote document. Independent additions and
non-overlapping field edits can merge; competing edits, delete-versus-edit,
incompatible hierarchy/order changes, or opaque-block divergence produce a
durable conflict bundle without partially committing the transaction.

Merging field by field is not enough to know the result is writable, so
`auditMergedDocument` judges the whole document the merge produced before that
document is allowed to become a commit. The document it judges carries the
merged comment collection as well as the merged tasks, so the project audit
covers both. It runs that audit — duplicate
task ids, hierarchy, cycles, depth, state, comment validity and comment id
uniqueness — the terminal/`CompletedAt`
agreement checks, and project-local ShortID uniqueness, and anything it finds
becomes an ordinary `MergeConflictInvariant` entry in the same retry-only
bundle a competing edit would produce.

The check-in is the one field group that is not merged field by field. `check_in`,
`check_in_repeat`, and `reviewed` answer one question together, so
`mergeCheckInUnit` adopts or refuses all three at once and the reflective loop
skips them; they stay in `mergeableTaskFields` so every other reader still sees a
complete field set. Merged separately they compose a schedule neither machine
wrote: acknowledging a weekly check-in moves the date and stamps the review
instant, rescheduling it moves the date and stamps nothing, and field by field
those two diverge only on the date — so a per-field merge would keep one side's
date beside the other's review instant and claim the task was answered on a day
that produced a different next date. Independent title and description edits
still merge beside a schedule only one side touched. Within the unit the three-way
cases are the familiar ones, compared by instant rather than by representation, so
two machines holding the same date in different locations coalesce instead of
conflicting; a divergence is one `check_in` field conflict naming the whole
schedule on every side, never one conflict per key.

`auditMergedDocument` carries the matching whole-document invariant. Terminal
work owes no reminder, so a completed task retaining a check-in is a
`MergeConflictInvariant` on `check_in`, exactly as a terminal task disagreeing
with `completed_at` is. Neither side wrote that state: it is composed by one
machine finishing the task while the other schedules it, two edits that are
field-wise clean and land on different fields, and without the audit the merge
would publish a reminder that fires forever against work nobody can do. The
`reviewed` instant is deliberately not audited that way, because it records what
happened rather than what is owed, and a finished task keeps the record of having
been reviewed.

Project comments merge like tasks do, keyed by identity rather than by position.
`mergeProjectComments` walks the union of the comment ids across the three sides
and decides each one on its own, which is what makes two machines commenting on
one project a non-event: their comments carry different UUIDs, so neither write
is contended and both land. Position is never consulted, because a comment the
other side added shifts every index below it — an index-keyed merge would
reconcile two unrelated comments, and the stable UUID is the only thing that
survives a concurrent insertion.

Within one identity the cases are the familiar three-way ones. An edit on one
side is adopted; identical concurrent edits coalesce; divergent ones conflict.
Deletion versus an untouched comment resolves as deletion, deletion on both sides
removes it, and deletion versus an edit conflicts, because neither intent
contains the other. Two different comments added under one id conflict rather
than either being adopted: independently minted UUIDs do not collide, so that
shape means the one-identity-one-comment invariant is already gone and adopting a
side would publish a body the other machine never agreed to store there.

`CreatedAt` is treated as identity metadata rather than content. It is half of
the canonical order, it is what the stored header spells beside the id, and no
mutation writes it — editing a comment rewrites the text alone. A side that moved
it is therefore reported, not reconciled, and that holds even when both sides
moved it the same way: two machines agreeing is not permission for an immutable
value to change.

Every one of those is a `MergeConflictProjectMetadata` entry carrying `CommentID`
and one of the fields `comment` (a presence race or a colliding addition),
`comment.text`, or `comment.created_at`. The id lives in a field of its own
rather than in `TaskTitle`: a bundle that spelled a comment id where a reader
expects a task title would send the operator looking for a task that does not
exist. The merge reports one entry per diverging identity instead of stopping at
the first, so a bundle can be resolved once rather than once per comment, and it
orders both the merged collection and the conflict list by the same canonical
creation-time-then-identity key, so which side the caller passed as local cannot
change the result. The merged collection is detached from all three inputs at one
point, whichever branch adopted each value.

The side summaries name the creation instant, a digest of the body, and its
length — never the body. The bundle already stores all three documents, so a
summary that inlined prose would be the only place a comment's text was
duplicated; the digest still answers the one question a summary has to answer,
which is whether two sides hold the same body.

A comment conflict aborts the whole merge exactly as a task conflict does, so a
clean task edit riding in the same transaction is refused with it and nothing
partial reaches disk.

ShortID is in that list because it is the one invariant the canonical writer
enforces that the project audit does not, and because concurrent creation is
the case that breaks it. A short id is minted next-free against the document
the minting machine can see, so two machines creating a task at the same time
both pick the same number; the merge itself is clean, since the two additions
touch nothing in common. Without the audit the whole was rejected later, while
rendering the merged document at the commit boundary — no data was lost, but
the losing machine received a bare render error, `tt conflict` had no bundle to
list, and the retry that would have succeeded was never suggested. Auditing it
here makes that outcome the third ordinary one: nothing partial on disk, a
classified bundle naming `short_id`, and a retry that mints against the merged
document and lands.

The maintenance CLI lists, shows, and resolves those bundles. Resolution
rechecks the current remote revision. A conflict from an aborted multi-artifact
transaction may require adopting remote and retrying the original operation so
all side effects are rebuilt together.

A bundle's three documents are rendered by `WriteCurrent()` at the schema live
when it was recorded, and no migration rewrites `.tt/conflicts/`, so a bundle
that outlives a schema bump holds markdown the current parser refuses. That
makes a stored document undecodable; it must not make the record unresolvable.
`loadDocumentFile` therefore degrades a parse failure the way it already degrades
a missing file — nil document, no error — and reports a reason on
`ConflictBundle.BaseUnreadable` / `LocalUnreadable`, naming the stored schema
marker when the file carries one. Metadata, `list`, `show`, the `gc` browser,
`--use remote`, and `--use file` all read nothing from those documents and keep
working. The two paths that do read one refuse with
`ErrConflictDocumentUnreadable` and name the reason: `--use local`, and the
re-merge against a newer remote, which cannot proceed without a base. Adopting
remote is the only legal resolution for a retry-required bundle, so treating a
parse failure as fatal for the whole bundle stranded exactly the record that
needed clearing.
