package tui

import "strings"

const (
	finderActionCategoryOpen   ListCategoryID = "finder-actions:open"
	finderActionCategoryManage ListCategoryID = "finder-actions:manage"
	finderActionCategoryDanger ListCategoryID = "finder-actions:danger"
)

// finderActionCategories group non-task results by risk. Every result kind owns
// a different operation family, so risk is the only grouping all of them share.
var finderActionCategories = []ListCategory{
	{ID: finderActionCategoryOpen, Label: "Open"},
	{ID: finderActionCategoryManage, Label: "Manage"},
	{ID: finderActionCategoryDanger, Label: "Danger"},
}

func finderActionCategory(descriptor OperationDescriptor) ListCategoryID {
	switch descriptor.Risk {
	case OperationRiskDestructive:
		return finderActionCategoryDanger
	case OperationRiskReadOnly:
		return finderActionCategoryOpen
	default:
		return finderActionCategoryManage
	}
}

// finderContextTarget is one Finder row frozen into everything the shared sheet
// needs: the context its operations resolve against, its heading, and the
// optional host payload an operation family requires.
type finderContextTarget struct {
	context  CommandContext
	title    string
	subtitle string
	value    any
	hasValue bool
}

// finderContextActionTarget freezes the selected row. Selection is always
// cleared so a stale Outliner multi-selection cannot widen a Finder action.
// Rows that name their own entity also drop the inherited one, so only the
// typed row identity decides applicability; tool and operation rows are global
// and keep the context they were ranked in.
func finderContextActionTarget(
	context CommandContext,
	target FinderTarget,
) (finderContextTarget, bool) {
	frozen := cloneCommandContext(context)
	frozen.Selection = SelectionContext{}
	switch target.Kind {
	case FinderItemProject, FinderItemNote, FinderItemTag,
		FinderItemSavedFilter, FinderItemScope, FinderItemMode:
		frozen.Entity = EntityContext{}
		frozen.CurrentEntity = EntityIdentity{}
	case FinderItemTask, FinderItemTool, FinderItemOperation:
		// Tools and operations are global and act on whatever the Finder was
		// opened over; a task row replaces the inherited entity below.
	}

	switch target.Kind {
	case FinderItemTask:
		if target.Task == nil || target.Project == nil {
			return finderContextTarget{}, false
		}
		// A task row names its own target, so the route that happened to launch
		// the Finder must not decide which task actions apply. Rebasing onto the
		// Outliner presentation keeps the frozen scope facts while letting the
		// row's own task be the only thing applicability depends on.
		frozen = rebaseOnOutlinerPresentation(frozen)
		frozen.Entity = taskOperationEntity(target.Task, target.Project)
		frozen.CurrentEntity = commandContextCurrentEntity(frozen.Entity)
		return finderContextTarget{
			context:  frozen,
			title:    "Task Actions",
			subtitle: target.Task.Title,
		}, true

	case FinderItemProject:
		if target.Project == nil {
			return finderContextTarget{}, false
		}
		frozen.CurrentEntity = ProjectEntity(target.Project.ID)
		frozen.FocusedProjectID = target.Project.ID
		return finderContextTarget{
			context:  frozen,
			title:    "Project Actions",
			subtitle: target.Project.Name,
		}, true

	case FinderItemNote:
		if target.Note == nil {
			return finderContextTarget{}, false
		}
		if target.Project != nil {
			frozen.CurrentEntity = ProjectNoteEntity(target.Project.ID, target.Note.Filename)
			frozen.FocusedProjectID = target.Project.ID
		} else {
			frozen.CurrentEntity = GlobalNoteEntity(target.Note.Filename)
		}
		return finderContextTarget{
			context:  frozen,
			title:    "Note Actions",
			subtitle: target.Note.Title,
		}, true

	case FinderItemTag:
		if target.Tag == nil {
			return finderContextTarget{}, false
		}
		// Finder tags are workspace aggregates, never one project's tag.
		frozen.CurrentEntity = WorkspaceTagEntity(target.Tag.Name)
		return finderContextTarget{
			context:  frozen,
			title:    "Tag Actions",
			subtitle: target.Tag.Name,
		}, true

	case FinderItemSavedFilter:
		if target.SavedFilter == nil {
			return finderContextTarget{}, false
		}
		frozen.CurrentEntity = SavedFilterEntity(target.SavedFilter.ID)
		return finderContextTarget{
			context:  frozen,
			title:    "Saved Filter Actions",
			subtitle: target.SavedFilter.Name,
		}, true

	case FinderItemScope:
		if target.Scope == nil {
			return finderContextTarget{}, false
		}
		scope := target.Scope.Scope
		// Scope operations are Scope-Hub capability gated and read their frozen
		// row from Value, exactly as the hub's own sheet supplies it.
		frozen.Capabilities.Add(CapabilityScopeHub)
		frozen.CurrentEntity = ScopeEntity(scope)
		return finderContextTarget{
			context:  frozen,
			title:    "Scope Actions",
			subtitle: contextScopeDisplayName(scope),
			value:    scopeHubActionTarget{Scope: scope},
			hasValue: true,
		}, true

	case FinderItemMode:
		if target.Mode == nil {
			return finderContextTarget{}, false
		}
		frozen.CurrentEntity = ModeEntity(target.Mode.ID)
		return finderContextTarget{
			context:  frozen,
			title:    "Mode Actions",
			subtitle: target.Mode.Label,
		}, true

	case FinderItemTool:
		if target.Tool == nil {
			return finderContextTarget{}, false
		}
		// Tools and operations are global; leaving the context untouched keeps the
		// sheet identical to what Enter on the same row would execute.
		return finderContextTarget{
			context:  frozen,
			title:    "Tool Actions",
			subtitle: target.Tool.Label,
		}, true

	case FinderItemOperation:
		if target.Operation == nil {
			return finderContextTarget{}, false
		}
		return finderContextTarget{
			context:  frozen,
			title:    "Operation Actions",
			subtitle: target.Operation.Label,
		}, true

	default:
		return finderContextTarget{}, false
	}
}

// rebaseOnOutlinerPresentation rebuilds a context on the Outliner route while
// carrying the session scope forward, so operations gated on Outliner context
// stay available to rows that carry their own target.
func rebaseOnOutlinerPresentation(context CommandContext) CommandContext {
	rebased := PresentationCommandContext(ViewCrossProjectOutliner)
	rebased.Capabilities.Add(CapabilitySimplifiedInput)
	rebased.ScopeProjectIDs = context.ScopeProjectIDs
	rebased.FocusedProjectID = context.FocusedProjectID
	return rebased
}

// NewFinderContextActionSheetModel builds the sheet for one non-task Finder row
// from the ordered manifest that row carries.
func NewFinderContextActionSheetModel(
	registry *OperationRegistry,
	executor *OperationExecutor,
	target finderContextTarget,
	operationIDs []OperationID,
) (*ContextActionSheetModel, error) {
	return NewContextActionSheetModel(
		registry,
		executor,
		target.context,
		operationIDs,
		ContextActionSheetConfig{
			Host:         ContextActionHostFinder,
			Completion:   ContextActionCompleteCloseHost,
			Source:       OperationSourceActionSheet,
			Title:        target.title,
			Subtitle:     strings.TrimSpace(target.subtitle),
			ItemIDPrefix: "finder-action:",
			Categories:   finderActionCategories,
			CategoryFor:  finderActionCategory,
			SecondaryFor: operationActionSecondaryText,
			Value:        target.value,
			HasValue:     target.hasValue,
			Targets:      1,
			EmptyState: ListEmptyState{
				Title:  "No actions available.",
				Detail: "This result cannot be acted on from the Finder.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
}
