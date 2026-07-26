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
Rules are evaluated whenever tasks are saved (state changes, edits, etc).

EXAMPLES

  When task becomes ready, add ready tag (event trigger):
    tt workflow add -n "Unblock Ready" -f state_changed -o equals -V "waiting->ready" -a add_tag --action-value ready

  When completed, ensure the task is done (event trigger):
    tt workflow add -n "Route Completed" -f completed -o equals -V true -a set_state --action-value done

  Auto-complete epic when all children done (hierarchical):
    tt workflow add -n "Epic Complete" -f type -o equals -V epic -f all_children_done -o equals -V true --logic and -a set_state --action-value done

  Inherit urgent tag from parent (hierarchical):
    tt workflow add -n "Inherit Urgent" -f parent_tags -o contains -V urgent -a inherit_tag --action-value urgent

  Tag overdue tasks:
    tt workflow add -n "Overdue Alert" -f due_days -o less_than -V 0 -a add_tag --action-value overdue

  Test which rules would match tasks (dry-run):
    tt workflow test

  List all rules:
    tt workflow list

TASK TRIGGER FIELDS (match current task values)
  state, priority, tags, type
  due_days, created_days, updated_days

BOOLEAN STATE TRIGGER FIELDS (presence/absence checks)
  has_due_date      - Value: "true"/"false"
  has_tags          - Value: "true"/"false"
  description_empty - Value: "true"/"false"

EVENT TRIGGER FIELDS (detect transitions)
  state_changed         - Value: "old->new" (e.g., "waiting,paused->ready", "*->done")
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
  parent_state, parent_type, parent_tags, level
  has_children, children_progress, all_children_done
  is_blocked, blocker_count, dependency_count

ACTION TYPES
  set_state, set_priority, add_tag, remove_tag
  set_due_relative, clear_due, set_pinned, add_comment, set_type
  complete_parent, block_parent, cascade_tag, inherit_tag, notify_parent
  set_snooze, create_followup, move_to_project, set_recurrence

RULE SCOPING
  Rules can be restricted to specific projects, task types, or states:
    tt workflow add ... --scope-project "MyProject"
    tt workflow add ... --scope-type task,bug --scope-state waiting,paused

COMPOUND TRIGGERS
  Multiple triggers can be combined with --logic:
    tt workflow add -n "Rule" -f state -o equals -V waiting -f priority -o equals -V 1 --logic and ...
    tt workflow add -n "Rule" -f tags -o contains -V urgent -f due_days -o less_than -V 3 --logic or ...`,
	RunE: runWorkflowList,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}
