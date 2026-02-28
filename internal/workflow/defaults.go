package workflow

// GetDefaultRules returns the built-in default workflow rules.
// Most default rules are created disabled so users can review and enable selectively.
// Some rules (like Active section sync) are enabled by default for expected UX.
func GetDefaultRules() []WorkflowRule {
	return []WorkflowRule{
		{
			ID:      "def-overdue",
			Name:    "Tag Overdue Tasks",
			Enabled: false,
			Source:  "default",
			Trigger: RuleTrigger{
				Field:     FieldDueDays,
				Operation: OpLessThan,
				Value:     "0",
			},
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "overdue"},
			},
		},
		{
			ID:      "def-stale",
			Name:    "Mark Stale In-Progress",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
				{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "14"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "stale"},
			},
		},
		{
			ID:      "def-epic-done",
			Name:    "Auto-complete Epic",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: "epic"},
				{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetStatus, Value: "done"},
			},
		},
		{
			ID:      "def-archive",
			Name:    "Archive Completed",
			Enabled: false,
			Source:  "default",
			Trigger: RuleTrigger{
				Field:     FieldCompleted,
				Operation: OpEquals,
				Value:     "true",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetSection, Value: "Archive"},
			},
		},
		// Activate on Active Section: sync active flag with Active section
		{
			ID:      "def-activate-on-active",
			Name:    "Activate on Active Section",
			Enabled: true,
			Source:  "default",
			Trigger: RuleTrigger{
				Field:     FieldSectionChanged,
				Operation: OpEquals,
				Value:     "*->",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetActive, Value: "true"},
			},
		},
		// Deactivate on Leave Active: clear active flag when leaving Active section
		{
			ID:      "def-deactivate-on-leave",
			Name:    "Deactivate on Leave Active",
			Enabled: true,
			Source:  "default",
			Trigger: RuleTrigger{
				Field:     FieldSectionChanged,
				Operation: OpEquals,
				Value:     "->*",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetActive, Value: "false"},
			},
		},
		// Auto-complete Story: same logic as epic completion
		{
			ID:      "def-story-done",
			Name:    "Auto-complete Story",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: "story"},
				{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetStatus, Value: "done"},
			},
		},
		// Clear Overdue Tag: cleanup when due date extended
		{
			ID:      "def-clear-overdue",
			Name:    "Clear Overdue Tag",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldDueDays, Operation: OpGreaterThan, Value: "-1"},
				{Field: FieldTags, Operation: OpContains, Value: "overdue"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionRemoveTag, Value: "overdue"},
			},
		},
		// Bug Aging Alert: bugs deserve faster attention
		{
			ID:      "def-bug-aging",
			Name:    "Bug Aging Alert",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: "bug"},
				{Field: FieldStatus, Operation: OpNotEquals, Value: "done"},
				{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "aging"},
			},
		},
		// Auto-block on Blockers: sync status with dependency state
		{
			ID:      "def-auto-block",
			Name:    "Auto-block on Blockers",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldIsBlocked, Operation: OpEquals, Value: "true"},
				{Field: FieldStatus, Operation: OpNotEquals, Value: "blocked"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetStatus, Value: "blocked"},
			},
		},
		// Unblock When Ready: complement to auto-block
		{
			ID:      "def-auto-unblock",
			Name:    "Unblock When Ready",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldIsBlocked, Operation: OpEquals, Value: "false"},
				{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetStatus, Value: "todo"},
			},
		},
		// High Priority Due Soon: bump priority when deadline approaches
		{
			ID:      "def-priority-due",
			Name:    "High Priority Due Soon",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
				{Field: FieldPriority, Operation: OpGreaterThan, Value: "2"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetPriority, Value: "2"},
			},
		},
		// Inbox Triage Reminder: remind user to process inbox items
		{
			ID:      "def-inbox-stale",
			Name:    "Inbox Triage Reminder",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
				{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "3"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "needs-triage"},
			},
		},
		// WIP Warning: highlight work-in-progress that may be stuck
		{
			ID:      "def-wip-warning",
			Name:    "WIP Warning",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
				{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "wip-old"},
			},
		},
		// Cascade Urgent Tag: propagate urgency down the hierarchy
		{
			ID:      "def-cascade-urgent",
			Name:    "Cascade Urgent Tag",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldTags, Operation: OpContains, Value: "urgent"},
				{Field: FieldHasChildren, Operation: OpEquals, Value: "true"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionCascadeTag, Value: "urgent"},
			},
		},
		// Epic Started: auto-start epic when any child begins
		{
			ID:      "def-epic-started",
			Name:    "Epic Started",
			Enabled: false,
			Source:  "default",
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: "epic"},
				{Field: FieldChildrenProgress, Operation: OpGreaterThan, Value: "0"},
				{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetStatus, Value: "in_progress"},
			},
		},

		// --- Project-level rules ---

		// Project Bugfix Mode: majority of active tasks are bugs
		{
			ID:       "def-proj-bugfix",
			Name:     "Project Bugfix Mode",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 30,
			Source:   "default",
			Trigger: RuleTrigger{
				Field:     FieldMajorityTaskType,
				Operation: OpEquals,
				Value:     "bug",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetProjectStatus, Value: "bugfix"},
			},
		},
		// Project In Review: majority of active tasks are in review
		{
			ID:       "def-proj-review",
			Name:     "Project In Review",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 30,
			Source:   "default",
			Trigger: RuleTrigger{
				Field:     FieldMajorityTaskStatus,
				Operation: OpEquals,
				Value:     "review",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetProjectStatus, Value: "review"},
			},
		},
		// Project Paused: no updates in 14+ days with active tasks
		{
			ID:       "def-proj-paused",
			Name:     "Project Paused",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 40,
			Source:   "default",
			Triggers: []RuleTrigger{
				{Field: FieldDaysSinceUpdate, Operation: OpGreaterThan, Value: "14"},
				{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetProjectStatus, Value: "paused"},
			},
		},
		// Project Active: has tasks currently in progress
		{
			ID:       "def-proj-active",
			Name:     "Project Active",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 50,
			Source:   "default",
			Trigger: RuleTrigger{
				Field:     FieldHasInProgressTasks,
				Operation: OpEquals,
				Value:     "true",
			},
			Actions: []RuleAction{
				{ActionType: ActionSetProjectStatus, Value: "active"},
			},
		},
		// Project Planning: has active tasks but none in progress
		{
			ID:       "def-proj-planning",
			Name:     "Project Planning",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 60,
			Source:   "default",
			Triggers: []RuleTrigger{
				{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"},
				{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "false"},
			},
			TriggerLogic: LogicAnd,
			Actions: []RuleAction{
				{ActionType: ActionSetProjectStatus, Value: "planning"},
			},
		},
	}
}
