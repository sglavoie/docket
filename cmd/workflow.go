package cmd

import (
	"github.com/spf13/cobra"
)

var workflowCmd = &cobra.Command{
	Use:     "workflow",
	Aliases: []string{"wf"},
	Short:   "Manage workflow automation rules",
	Long: `Commands for creating, listing, and managing workflow automation rules.

Workflow rules automatically modify tasks when conditions are met.
Rules are evaluated whenever tasks are saved (status changes, edits, etc).

EXAMPLES

  When task unblocked, add ready tag (event trigger):
    tt workflow add -n "Unblock Ready" -f status_changed -o equals -V "blocked->*" -a add_tag --action-value ready

  When completed, move to Completed (event trigger):
    tt workflow add -n "Route Completed" -f completed -o equals -V true -a set_section --action-value Completed

  Auto-complete epic when all children done (hierarchical):
    tt workflow add -n "Epic Complete" -f type -o equals -V epic -f all_children_done -o equals -V true --logic and -a set_status --action-value done

  Inherit urgent tag from parent (hierarchical):
    tt workflow add -n "Inherit Urgent" -f parent_tags -o contains -V urgent -a inherit_tag --action-value urgent

  Tag overdue tasks:
    tt workflow add -n "Overdue Alert" -f due_days -o less_than -V 0 -a add_tag --action-value overdue

  Test which rules would match tasks (dry-run):
    tt workflow test

  List all rules:
    tt workflow list

STATE TRIGGER FIELDS (match current task state)
  status, priority, section, tags, type
  due_days, created_days, updated_days

BOOLEAN STATE TRIGGER FIELDS (presence/absence checks)
  has_due_date      - Value: "true"/"false"
  has_tags          - Value: "true"/"false"
  description_empty - Value: "true"/"false"

EVENT TRIGGER FIELDS (detect transitions)
  status_changed        - Value: "old->new" (e.g., "blocked->*", "*->done")
  section_changed       - Value: "old->new" (e.g., "Next->Waiting")
  priority_changed      - Value: "old->new" (e.g., "*->1")
  tag_added             - Value: tag name that was added
  tag_removed           - Value: tag name that was removed
  completed             - Value: "true" (fires on done/canceled)
  reopened              - Value: "true" (fires when reopened)
  due_date_changed      - Value: "added"/"removed"/"changed"/"*"
  type_changed          - Value: "old->new" (e.g., "task->bug")
  recurrence_completed  - Value: "true" (fires when recurring task completes)
  streak_broken         - Value: "true" (fires when recurrence streak resets)

HIERARCHICAL TRIGGER FIELDS
  parent_status, parent_type, parent_tags, level
  has_children, children_progress, all_children_done
  is_blocked, blocker_count, dependency_count

ACTION TYPES
  set_status, set_priority, set_section, add_tag, remove_tag
  set_due_relative, clear_due, set_pinned, add_comment, set_type
  complete_parent, block_parent, cascade_tag, inherit_tag, notify_parent
  set_snooze, create_followup, move_to_project, set_recurrence

RULE SCOPING
  Rules can be restricted to specific projects, task types, or sections:
    tt workflow add ... --scope-project "MyProject"
    tt workflow add ... --scope-type task,bug --scope-section Waiting

COMPOUND TRIGGERS
  Multiple triggers can be combined with --logic:
    tt workflow add -n "Rule" -f status -o equals -V blocked -f priority -o equals -V 1 --logic and ...
    tt workflow add -n "Rule" -f tags -o contains -V urgent -f due_days -o less_than -V 3 --logic or ...`,
	RunE: runWorkflowList,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}
