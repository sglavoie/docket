# Project: Alpha Fixture
<!-- tt-schema: 5 -->

## Task: Rich retained and retired fields
<!-- id: task-rich, short_id: 1, state: ready, priority: 0, tags: feature;source:fixture, created: 2026-07-01T09:00:00Z, updated: 2026-07-02T10:00:00Z, due: 2026-08-15, snoozed: 2026-08-05, depends: task-waiting;task-cross-project, pinned: true, pinned_at: 2026-07-01T09:05:00Z, recur: 2w, attachments: file|specification.txt|%2Fsynthetic%2Fsource%2Fspecification.txt|0|2026-07-01T09:10:00Z;url|https%3A%2F%2Fexample.com%2Ffixture|https%3A%2F%2Fexample.com%2Ffixture|0|;code|task.go|internal%2Fmodels%2Ftask.go|42|2026-07-01T09:15:00Z, checklist: 1|Characterize+retained+fields;0|Exercise+schema+6+conversion -->
<!-- DESCRIPTION -->
Multiline synthetic description.
It preserves rich text across the schema boundary.
<!-- /DESCRIPTION -->
<!-- COMMENTS -->
<!-- comment: 2026-07-01T09:30:00Z -->
First ordinary comment.
<!-- comment: 2026-07-01T09:40:00Z -->
Equal-time ordinary comment must sort before decisions.
<!-- /COMMENTS -->
<!-- DECISIONS -->
<!-- decision: 2026-07-01T09:35:00Z -->
Keep the strict conversion boundary
<!-- rationale -->
It makes dropped fields auditable.
<!-- decision: 2026-07-01T09:40:00Z -->
Preserve source order at equal timestamps
<!-- rationale -->
Comments precede decisions when timestamps match.
<!-- /DECISIONS -->

### Task: Inbox child
<!-- id: task-inbox, short_id: 2, state: inbox, priority: 1, parent: task-rich, created: 2026-07-01T10:00:00Z, updated: 2026-07-01T10:00:00Z -->

#### Task: Explicit Next grandchild
<!-- id: task-next, short_id: 3, state: next, priority: 2, parent: task-inbox, created: 2026-07-01T10:10:00Z, updated: 2026-07-01T10:10:00Z -->

## Task: Active Doing source
<!-- id: task-doing, short_id: 4, state: in_progress, priority: 3, created: 2026-07-01T10:20:00Z, updated: 2026-07-01T10:20:00Z -->

## Task: Waiting source
<!-- id: task-waiting, short_id: 5, state: waiting, priority: 4, created: 2026-07-01T10:30:00Z, updated: 2026-07-01T10:30:00Z -->

## Task: Paused source
<!-- id: task-paused, short_id: 6, state: paused, priority: 5, created: 2026-07-01T10:40:00Z, updated: 2026-07-01T10:40:00Z -->

## Task: Someday source
<!-- id: task-someday, short_id: 7, state: someday, priority: 0, created: 2026-07-01T10:50:00Z, updated: 2026-07-01T10:50:00Z -->

## Task: Done terminal source
<!-- id: task-done, short_id: 8, state: done, priority: 1, created: 2026-07-01T11:00:00Z, updated: 2026-07-02T11:00:00Z, completed: 2026-07-02T11:00:00Z -->

## Task: Canceled terminal source
<!-- id: task-canceled, short_id: 9, state: canceled, priority: 2, created: 2026-07-01T11:10:00Z, updated: 2026-07-02T11:10:00Z, completed: 2026-07-02T11:10:00Z -->
