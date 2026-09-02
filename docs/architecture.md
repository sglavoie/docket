# Architecture, data, migration, and recovery

`tt` is an Outliner-first local application. The Bubble Tea TUI is the product
surface; Cobra exposes only maintenance commands. Both use the same application
services and revision-checked storage boundaries.

This page is the index. Each chapter below is one file under
[`architecture/`](architecture/); read them in the listed order for the whole
mechanism, or jump straight to the one that owns your concern.

## Contents

- [Runtime layers](architecture/runtime-and-task-model.md#runtime-layers) and
  [Task model](architecture/runtime-and-task-model.md#task-model) —
  [Workflow and planning](architecture/runtime-and-task-model.md#workflow-and-planning),
  [Hierarchy and ordering](architecture/runtime-and-task-model.md#hierarchy-and-ordering),
  [Rich records](architecture/runtime-and-task-model.md#rich-records)
- [Canonical query](architecture/canonical-query.md#canonical-query) — clause
  vocabulary, the join law, canonical order, equality and fingerprint, the three
  non-clause axes
- [TUI architecture](architecture/tui-architecture.md#tui-architecture) — modes,
  project grouping, `n`/`N` header walk, checklist rows, Recent and `Ctrl+L`
  - [Presentation chrome](architecture/presentation-chrome.md#presentation-chrome)
    — pulse, title fitter, guidance, Pinned and Review
  - [Row indicators](architecture/row-indicators.md#row-indicators) and
    [Status flash](architecture/row-indicators.md#status-flash)
  - [One operation grammar](architecture/operation-grammar.md#one-operation-grammar)
    — registry and executor, value providers, operation chaining, `Space`
    manifest, chord misses
  - [Capture and inline-edit grammar](architecture/capture-and-outline-editor.md#capture-and-inline-edit-grammar)
    — sigil resolution order, pin values, structural references, adoption
  - [Outline editor](architecture/capture-and-outline-editor.md#outline-editor) —
    dump scope, markers, declarative deletion, validation phases
  - [Search, filters, and Peek](architecture/search-filters-and-sorting.md#search-filters-and-peek)
    — facets, `F` populations, `zt`/`zz`/`zb`, Focus and `Esc`, Peek budget
  - [Sorting axis](architecture/search-filters-and-sorting.md#sorting-axis) —
    manual-order exemption, group order, fixed-order modes
  - [Named lenses](architecture/named-lenses.md#named-lenses) — portable domain,
    commit before publish, the single restoration point, lens query editing, the
    built-in view catalog
  - [Selection and subordinate tools](architecture/selection-and-project-tags.md#selection-and-subordinate-tools)
    — visual selection, Select Project, `Ctrl+T` axis
  - [Project tags](architecture/selection-and-project-tags.md#project-tags) — the
    three axes, registries, park confirmation, the two populations, Notes
- [Persistence layout](architecture/persistence-and-transactions.md#persistence-layout)
- [Transactions and recovery](architecture/persistence-and-transactions.md#transactions-and-recovery)
- [Concurrent edits and conflicts](architecture/persistence-and-transactions.md#concurrent-edits-and-conflicts)
- [Snapshot-first schema migration](architecture/migration.md#snapshot-first-schema-migration)
- [Workspace location and cloud consent](architecture/workspace-and-maintenance.md#workspace-location-and-cloud-consent)
- [Backup and restore](architecture/workspace-and-maintenance.md#backup-and-restore)
- [Maintenance CLI and platform support](architecture/workspace-and-maintenance.md#maintenance-cli-and-platform-support)
- [Verification](architecture/workspace-and-maintenance.md#verification)
