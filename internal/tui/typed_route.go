package tui

import (
	"errors"
	"fmt"
	"image/color"
	"reflect"
)

// RouteID is the stable identity of a primary TUI surface. Modes and entity
// bindings live in sealed RouteParams values rather than multiplying route
// identities.
type RouteID uint8

const (
	Home RouteID = iota
	Planner
	TaskWorkspace
	ReviewHub
	ManageHub
	TaskDetail
	Reference
)

var routeIDNames = [...]string{
	"Home",
	"Planner",
	"TaskWorkspace",
	"ReviewHub",
	"ManageHub",
	"TaskDetail",
	"Reference",
}

func (id RouteID) String() string {
	if int(id) < len(routeIDNames) {
		return routeIDNames[id]
	}
	return fmt.Sprintf("RouteID(%d)", id)
}

// IsDestination reports whether this identity may replace the primary stack
// through Ctrl+G. Reference is intentionally presentation-only.
func (id RouteID) IsDestination() bool {
	return id == Home || id == Planner || id == TaskWorkspace || id == ReviewHub || id == ManageHub
}

// RouteParams is sealed so every route instance can be exhaustively validated
// and canonicalized by NewRoute.
type RouteParams interface {
	routeParams()
}

type HomeParams struct{}

func (HomeParams) routeParams() {}

type PlannerTab uint8

const (
	PlannerToday PlannerTab = iota
	PlannerNext
	PlannerBacklog
)

type PlannerParams struct{ Tab PlannerTab }

func (PlannerParams) routeParams() {}

type TaskWorkspaceLayout uint8

const (
	WorkspaceOutline TaskWorkspaceLayout = iota
	WorkspaceList
	WorkspaceBoard
)

type TaskWorkspaceFocus uint8

const (
	WorkspaceActive TaskWorkspaceFocus = iota
	WorkspaceAll
	WorkspaceInbox
	WorkspaceNext
	WorkspaceWaiting
	WorkspaceReview
	WorkspaceSomeday
	WorkspaceCompleted
	WorkspaceDueSoon
	WorkspaceHighPriority
	WorkspaceBlocked
	WorkspaceParked
	WorkspaceSnoozed
	WorkspacePinned
	WorkspaceRecent
	WorkspaceSavedFilter
)

var taskWorkspaceFocusNames = [...]string{
	"Active",
	"All",
	"Inbox",
	"Next",
	"Waiting",
	"Review",
	"Someday",
	"Completed",
	"Due Soon",
	"High Priority",
	"Blocked",
	"Parked",
	"Snoozed",
	"Pinned",
	"Recent",
	"Saved Filter",
}

func (focus TaskWorkspaceFocus) String() string {
	if int(focus) < len(taskWorkspaceFocusNames) {
		return taskWorkspaceFocusNames[focus]
	}
	return fmt.Sprintf("TaskWorkspaceFocus(%d)", focus)
}

// EntityScope identifies app-wide or project-bound route data. App scope is
// canonicalized to an empty ProjectID; project scope requires one.
type EntityScope struct {
	Kind      RouteScope
	ProjectID string
}

func AppEntityScope() EntityScope { return EntityScope{Kind: ScopeApp} }

func ProjectEntityScope(projectID string) EntityScope {
	return EntityScope{Kind: ScopeProject, ProjectID: projectID}
}

type TaskWorkspaceParams struct {
	Scope         EntityScope
	Layout        TaskWorkspaceLayout
	Focus         TaskWorkspaceFocus
	SavedFilterID string
}

func (TaskWorkspaceParams) routeParams() {}

type ReviewModule uint8

const (
	ReviewLanding ReviewModule = iota
	ReviewDaily
	ReviewWeekly
	ReviewActivity
	ReviewMetrics
	ReviewRoadmap
)

type ReviewParams struct{ Module ReviewModule }

func (ReviewParams) routeParams() {}

type ManageModule uint8

const (
	ManageLanding ManageModule = iota
	ManageInbox
	ManageNotes
	ManageBlockers
	ManageRules
	ManageTemplates
	ManageTags
	ManageNotifications
	ManageSettings
)

type NotificationMode uint8

const (
	NotificationAlerts NotificationMode = iota
	NotificationLog
)

type ManageParams struct {
	Module           ManageModule
	Scope            EntityScope
	NotificationMode NotificationMode
}

func (ManageParams) routeParams() {}

type TaskDetailTab uint8

const (
	TaskOverview TaskDetailTab = iota
	TaskChecklist
	TaskComments
	TaskDecisions
	TaskAttachments
	TaskActivity
)

type TaskDetailParams struct {
	ProjectID string
	TaskID    string
	Tab       TaskDetailTab
}

func (TaskDetailParams) routeParams() {}

type ReferenceKind uint8

const (
	ReferenceHelp ReferenceKind = iota
	ReferenceAsk
	ReferenceTutor
)

type ReferenceParams struct{ Kind ReferenceKind }

func (ReferenceParams) routeParams() {}

var (
	ErrInvalidRouteID     = errors.New("invalid route ID")
	ErrRouteParamsType    = errors.New("route parameter type does not match route ID")
	ErrInvalidRouteParams = errors.New("invalid route parameters")
	ErrProjectRequired    = errors.New("project-scoped route requires a project ID")
	ErrTaskRequired       = errors.New("task detail requires a task ID")
)

// Route is a validated primary route instance. Its fields are private so an
// impossible identity/parameter pair cannot be assembled outside NewRoute.
type Route struct {
	id              RouteID
	params          RouteParams
	presentation    PresentationID
	hasPresentation bool
}

// NewRoute validates the parameter type and value domain, and drops fields
// irrelevant to the selected mode so equal routes always have equal meaning.
func NewRoute(id RouteID, params RouteParams) (Route, error) {
	if id > Reference {
		return Route{}, fmt.Errorf("%w: %d", ErrInvalidRouteID, id)
	}
	if params == nil {
		params = defaultRouteParams(id)
	}

	canonical, err := canonicalRouteParams(id, params)
	if err != nil {
		return Route{}, err
	}
	return Route{id: id, params: canonical}, nil
}

func defaultRouteParams(id RouteID) RouteParams {
	switch id {
	case Home:
		return HomeParams{}
	case Planner:
		return PlannerParams{}
	case TaskWorkspace:
		return TaskWorkspaceParams{Scope: AppEntityScope()}
	case ReviewHub:
		return ReviewParams{}
	case ManageHub:
		return ManageParams{Scope: AppEntityScope()}
	case Reference:
		return ReferenceParams{}
	default:
		return nil
	}
}

func canonicalRouteParams(id RouteID, params RouteParams) (RouteParams, error) { //nolint:gocyclo
	switch id {
	case Home:
		if _, ok := params.(HomeParams); !ok {
			return nil, paramsTypeError(id, params)
		}
		return HomeParams{}, nil
	case Planner:
		p, ok := params.(PlannerParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.Tab > PlannerBacklog {
			return nil, invalidParams(id, "unknown planner tab")
		}
		return p, nil
	case TaskWorkspace:
		p, ok := params.(TaskWorkspaceParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		scope, err := canonicalEntityScope(id, p.Scope)
		if err != nil {
			return nil, err
		}
		if p.Layout > WorkspaceBoard {
			return nil, invalidParams(id, "unknown workspace layout")
		}
		if p.Focus > WorkspaceSavedFilter {
			return nil, invalidParams(id, "unknown workspace focus")
		}
		if p.Focus == WorkspaceSavedFilter {
			if p.SavedFilterID == "" {
				return nil, invalidParams(id, "saved-filter focus requires a saved filter ID")
			}
		} else {
			p.SavedFilterID = ""
		}
		p.Scope = scope
		return p, nil
	case ReviewHub:
		p, ok := params.(ReviewParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.Module > ReviewRoadmap {
			return nil, invalidParams(id, "unknown review module")
		}
		return p, nil
	case ManageHub:
		p, ok := params.(ManageParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.Module > ManageSettings {
			return nil, invalidParams(id, "unknown manage module")
		}
		if p.NotificationMode > NotificationLog {
			return nil, invalidParams(id, "unknown notification mode")
		}
		if manageModuleSupportsScope(p.Module) {
			scope, err := canonicalEntityScope(id, p.Scope)
			if err != nil {
				return nil, err
			}
			p.Scope = scope
		} else {
			p.Scope = AppEntityScope()
		}
		if p.Module != ManageNotifications {
			p.NotificationMode = NotificationAlerts
		}
		return p, nil
	case TaskDetail:
		p, ok := params.(TaskDetailParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.ProjectID == "" {
			return nil, ErrProjectRequired
		}
		if p.TaskID == "" {
			return nil, ErrTaskRequired
		}
		if p.Tab > TaskActivity {
			return nil, invalidParams(id, "unknown task detail tab")
		}
		return p, nil
	case Reference:
		p, ok := params.(ReferenceParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.Kind > ReferenceTutor {
			return nil, invalidParams(id, "unknown reference kind")
		}
		return p, nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrInvalidRouteID, id)
	}
}

func canonicalEntityScope(id RouteID, scope EntityScope) (EntityScope, error) {
	switch scope.Kind {
	case ScopeApp:
		scope.ProjectID = ""
		return scope, nil
	case ScopeProject:
		if scope.ProjectID == "" {
			return EntityScope{}, ErrProjectRequired
		}
		return scope, nil
	default:
		return EntityScope{}, invalidParams(id, "scope must be app or project")
	}
}

func manageModuleSupportsScope(module ManageModule) bool {
	return module == ManageInbox || module == ManageNotes || module == ManageBlockers || module == ManageTags
}

func paramsTypeError(id RouteID, params RouteParams) error {
	return fmt.Errorf("%w: %s does not accept %T", ErrRouteParamsType, id, params)
}

func invalidParams(id RouteID, detail string) error {
	return fmt.Errorf("%w for %s: %s", ErrInvalidRouteParams, id, detail)
}

func (r Route) ID() RouteID { return r.id }

func (r Route) Params() RouteParams { return r.params }

func (r Route) IsDestination() bool { return r.id.IsDestination() }

// Presentation returns the implementation renderer selected for this route.
func (r Route) Presentation() (PresentationID, bool) { return r.presentation, r.hasPresentation }

// Equal compares semantic route identity and canonical parameters. Renderer
// details do not change route identity.
func (r Route) Equal(other Route) bool {
	return r.id == other.id && reflect.DeepEqual(r.params, other.params)
}

// Accent returns the renderer accent when present and otherwise the route's
// category color.
func (r Route) Accent() color.Color {
	if r.hasPresentation {
		return accentColorForCategory(presentationAccentCategory(r.presentation))
	}
	switch r.id {
	case Home:
		return accentColorForCategory(AccentCatDashboard)
	case Planner:
		return accentColorForCategory(AccentCatPlan)
	case TaskWorkspace, TaskDetail:
		return accentColorForCategory(AccentCatExecute)
	case ReviewHub:
		return accentColorForCategory(AccentCatReview)
	case ManageHub, Reference:
		return accentColorForCategory(AccentCatSupport)
	default:
		return nil
	}
}
