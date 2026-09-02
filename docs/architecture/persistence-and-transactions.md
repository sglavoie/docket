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
        ├── tasks.md         # strict schema-13 generic Task Markdown
        ├── notes/           # minimal project notes
        └── .attachments/    # task-addressed file binaries
```

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

Project revisions hash canonical schema-13 content. A stale writer is compared
with its base and current remote document. Independent additions and
non-overlapping field edits can merge; competing edits, delete-versus-edit,
incompatible hierarchy/order changes, or opaque-block divergence produce a
durable conflict bundle without partially committing the transaction.

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
