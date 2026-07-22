package tui

// PlannerAction identifies one action in the Planner's declarative key and
// command catalog. It is intentionally local to Planner; the Outliner owns its
// mature keymap independently.
type PlannerAction string

const (
	WorkspaceActionCursorDown    PlannerAction = "cursor-down"
	WorkspaceActionCursorUp      PlannerAction = "cursor-up"
	WorkspaceActionJumpFirst     PlannerAction = "jump-first"
	WorkspaceActionJumpLast      PlannerAction = "jump-last"
	WorkspaceActionPageUp        PlannerAction = "page-up"
	WorkspaceActionPageDown      PlannerAction = "page-down"
	WorkspaceActionNextGroup     PlannerAction = "next-group"
	WorkspaceActionPreviousGroup PlannerAction = "previous-group"
	WorkspaceActionOpenDetail    PlannerAction = "open-detail"
	WorkspaceActionOpenPeek      PlannerAction = "open-peek"
	WorkspaceActionSelect        PlannerAction = "toggle-selection"
	WorkspaceActionSearch        PlannerAction = "search"
	WorkspaceActionViewMenu      PlannerAction = "view-menu"
	WorkspaceActionAdd           PlannerAction = "add"
	WorkspaceActionEdit          PlannerAction = "edit"
	WorkspaceActionComplete      PlannerAction = "complete"
	WorkspaceActionDelete        PlannerAction = "delete"
	WorkspaceActionState         PlannerAction = "state"
	WorkspaceActionPin           PlannerAction = "pin"
	WorkspaceActionPriority      PlannerAction = "priority"
	WorkspaceActionDueDate       PlannerAction = "due-date"
	WorkspaceActionTags          PlannerAction = "tags"
	WorkspaceActionSnooze        PlannerAction = "snooze"
	WorkspaceActionMove          PlannerAction = "move"
	WorkspaceActionMoveDown      PlannerAction = "move-down"
	WorkspaceActionMoveUp        PlannerAction = "move-up"
	WorkspaceActionPark          PlannerAction = "park"
	WorkspaceActionCopy          PlannerAction = "copy"
	WorkspaceActionUndo          PlannerAction = "undo"
	WorkspaceActionRedo          PlannerAction = "redo"
)

type PlannerBinding struct {
	Key                  string
	Action               PlannerAction
	Description          string
	Category             string
	RequiredCapabilities CapabilitySet
}

func plannerBinding(key string, action PlannerAction, description, category string, required ...Capability) PlannerBinding {
	return PlannerBinding{
		Key: key, Action: action, Description: description, Category: category,
		RequiredCapabilities: NewCapabilitySet(required...),
	}
}
