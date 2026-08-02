# Synthetic schema-5 consolidation corpus

This directory is the hand-authored acceptance corpus for the schema-5 to
schema-6 consolidation. All identifiers, paths, timestamps, URLs, and content
are synthetic. Nothing was copied from a user workspace.

The corpus deliberately preserves awkward but schema-5-valid combinations so
the migration must make every repair explicitly:

- `workspace/projects/alpha/tasks.md` and the other project documents cover all
  nine schema-5 workflow states, all six schema-5 priority values, hierarchy,
  rich text, tags, dates, pinning, comments, decisions, recurrence,
  dependencies, attachments, and checklist items.
- `workspace/.tt/planning.json` overlaps Today and Next ordering, references
  terminal and missing tasks, orders tasks that are not in StateNext, and omits
  one StateNext task from the saved order.
- `workspace/.tt/workspace.json` preserves the protected Inbox project ID and
  several ordinary project context labels.
- `machine/` contains custom contexts plus the retained theme selection.
- Project/global notes and a task attachment exercise retained side artifacts.
- `workspace/.tt/undo/` contains one undo entry and one redo entry with valid
  content-addressed blobs. The absolute artifact paths are intentionally rooted
  at `/synthetic/workspace` so validation never targets a real workspace.
- `transformation-matrix.json` is the machine-readable source of truth for
  every task-field action, lifecycle/priority mapping, domain repair, and side
  artifact disposition expected at the schema boundary.
- `retired_concepts_baseline.txt` is generated from production Go syntax by
  `TestRetiredConceptInventoryBaseline`; it inventories recurrence,
  dependency, decision, and custom-context identifiers until live ownership is
  removed in later sessions.

Tests must read the corpus in place. Any test that exercises mutation should
first copy it to a temporary directory.
