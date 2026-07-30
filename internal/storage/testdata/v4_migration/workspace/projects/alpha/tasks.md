# Project: Alpha Fixture
<!-- tt-schema: 2 -->
<!-- sections: Inbox:blue;Next:cyan;Waiting:yellow;Review:purple;Standby:gray;Someday:white;Completed:green -->
<!-- tags: synthetic;catalog-only;type:bug;source:github -->
<!-- pinned_tags: synthetic;catalog-only -->

## Epic: Rich retained and retired fields
<!-- id: synthetic-alpha-rich, state: in_progress, priority: 1, tags: synthetic;Namespaced;type:legacy, created: 2026-01-02 08:00, updated: 2026-02-03 09:15, due: 2026-02-03T14:30, start: 2026-01-15, estimated_days: 3.5, depends: synthetic-beta-root, related: synthetic-alpha-related, pinned: true, pinned_at: 2026-02-03 09:15:30, dimmed: true, parked: Synthetic resume context, parked_at: 2026-02-02 16:20, attachments: specification.txt||/synthetic/source/specification.txt@1769947200;url:https://example.invalid/reference||https://example.invalid/reference@1769947260;code:handler.go||/synthetic/src/handler.go:42@1769947320, checklist: Retained incomplete item[ ];Retained complete item[x], source: GitHub Issues, url: https://example.invalid/legacy-url, epic.scope: evolving, epic.milestone: Synthetic milestone, epic.metrics: Synthetic metric one\\nSynthetic metric two, epic.initial_children: 4, epic.scope_baselined: 2026-01-20, score.impact: 5, score.effort: 2 -->
<!-- DESCRIPTION -->
Synthetic description retained by the v5 task model.
<!-- /DESCRIPTION -->

<!-- ACTIVITY -->
<!-- act: 2026-02-03 09:15, priority: 2 > 1 -->
<!-- act: 2026-01-02 08:00, created -->
<!-- /ACTIVITY -->

<!-- COMMENTS -->
<!-- comment: 2026-02-01 10:00, resolved -->
Synthetic resolved comment text remains while its resolved flag is dropped.
<!-- comment: 2026-02-02 10:00 -->
Synthetic open comment text.
<!-- /COMMENTS -->

<!-- DECISIONS -->
<!-- decision: 2026-02-01 11:00, category:architecture, resolved -->
Keep the synthetic durable boundary.
<!-- rationale -->
The rationale itself remains in v5.
<!-- /DECISIONS -->

### Story: Review task with yearly recurrence
<!-- id: synthetic-alpha-review, state: review, priority: 2, parent: synthetic-alpha-rich, tags: approval, created: 2026-01-03 08:00, due: 2026-02-28, recur: 1y~, recur.skipped: 2, recur.streak: 3, recur.max_streak: 7, recur.original_day: 28, story.points: 8, story.persona: Synthetic reviewer -->

### Bug: Waiting typed task
<!-- id: synthetic-alpha-bug, state: waiting, priority: 2, parent: synthetic-alpha-rich, tags: defect, created: 2026-01-04 08:00, depends: synthetic-alpha-review, bug.reproducible: sometimes, bug.severity: major, bug.workaround: Use the synthetic fallback -->

### Idea: Someday typed task
<!-- id: synthetic-alpha-idea, state: someday, priority: 5, parent: synthetic-alpha-rich, tags: research, created: 2026-01-05 08:00, idea.category: research, idea.effort: large, idea.value: high, idea.validated: true -->

### Task: Next task with weekday recurrence
<!-- id: synthetic-alpha-weekday, state: next, priority: 3, parent: synthetic-alpha-rich, tags: schedule, created: 2026-01-06 08:00, due: 2026-02-06, recur: 2w:mon,wed,fri, recur.skipped: 1, recur.streak: 2, recur.max_streak: 4, task.effort: medium, task.worktype: testing -->

## Action: Paused task with timestamp snooze
<!-- id: synthetic-alpha-paused, state: paused, priority: 4, tags: legacy-action, created: 2026-01-07 08:00, snoozed: 2026-02-04 21:45 -->

## Note: Inbox legacy heading
<!-- id: synthetic-alpha-note, state: inbox, priority: 3, tags: legacy-note, created: 2026-01-08 08:00 -->

## Task: Canceled task
<!-- id: synthetic-alpha-canceled, state: canceled, priority: 3, tags: terminal, created: 2026-01-09 08:00, completed: 2026-01-12 12:00 -->

## Task: Active duplicate wins
<!-- id: synthetic-duplicate-active, state: ready, priority: 3, tags: active-wins, created: 2026-01-10 08:00 -->

## Task: Missing parent repair
<!-- id: synthetic-alpha-orphan, state: ready, priority: 3, parent: synthetic-missing-parent, tags: repair, created: 2026-01-11 08:00, depends: synthetic-duplicate-active;synthetic-beta-root -->

## Task: Related-link target
<!-- id: synthetic-alpha-related, state: ready, priority: 3, tags: relation-target, created: 2026-01-12 08:00 -->
