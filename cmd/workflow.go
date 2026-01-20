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

  Move high-priority tasks to Active section:
    tt workflow add -n "High Priority Active" -f priority -o equals -V 1 -a set_section --action-value ""

  Add 'urgent' tag when a task is blocked:
    tt workflow add -n "Tag Blocked" -f status -o equals -V blocked -a add_tag --action-value urgent

  Auto-start tasks moved to Active (empty section):
    tt workflow add -n "Active Starts" -f section -o equals -V "" -a set_status --action-value in_progress

  Bump priority when 'critical' tag is added:
    tt workflow add -n "Critical Priority" -f tags -o contains -V critical -a set_priority --action-value 1

  Move completed tasks to Archive section:
    tt workflow add -n "Archive Done" -f status -o equals -V done -a set_section --action-value Archive

  Test which rules would match tasks (dry-run):
    tt workflow test

  List all rules:
    tt workflow list

  Toggle a rule on/off:
    tt workflow toggle <rule-id>

  Delete a rule:
    tt workflow delete <rule-id>

TRIGGER FIELDS
  status    - Task status (todo, in_progress, blocked, done, cancelled)
  priority  - Priority level (1-5, where 1 is highest)
  section   - Section name (empty string "" means Active/no section)
  tags      - Task tags
  type      - Task type (task, epic, story, bug, idea, note)

TRIGGER OPERATIONS
  equals       - Exact match (case-insensitive)
  not_equals   - Does not match
  contains     - Contains substring (or has tag)
  greater_than - Numeric comparison
  less_than    - Numeric comparison

ACTION TYPES
  set_status   - Change task status
  set_priority - Change priority (1-5)
  set_section  - Move to section (empty string for Active)
  add_tag      - Add a tag
  remove_tag   - Remove a tag`,
	RunE: runWorkflowList,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}
