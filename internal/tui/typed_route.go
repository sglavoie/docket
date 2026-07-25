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
	TaskWorkspace RouteID = iota
	SecondaryTool
	TaskDetail
	Reference
)

var routeIDNames = [...]string{
	"TaskWorkspace",
	"SecondaryTool",
	"TaskDetail",
	"Reference",
}

func (id RouteID) String() string {
	if int(id) < len(routeIDNames) {
		return routeIDNames[id]
	}
	return fmt.Sprintf("RouteID(%d)", id)
}

// IsDestination reports whether this identity may occupy the single primary
// slot. The Outliner (TaskWorkspace) is the sole canonical root and the only
// destination type; the retired Home and Planner routes have been deleted.
// Secondary tools are hierarchical children launched from the root, and
// Reference is intentionally presentation-only.
func (id RouteID) IsDestination() bool {
	return id == TaskWorkspace
}

// RouteParams is sealed so every route instance can be exhaustively validated
// and canonicalized by NewRoute.
type RouteParams interface {
	routeParams()
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
	Scope EntityScope
}

func (TaskWorkspaceParams) routeParams() {}

// ReviewModule and ManageModule identify a specific retained secondary tool.
// They travel as SecondaryTool route parameters; the ReviewLanding/ManageLanding
// sentinels are only the pre-activation state of the hub renderers and are never
// valid route parameters (secondary tools launch directly, without a landing).
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
	ManageContexts
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
	// ErrTaskWorkspaceProjectScope rejects a project-scoped TaskWorkspace: the
	// Outliner root is app-scoped only, and project selection is an Outliner
	// scope frame rather than a second root identity.
	ErrTaskWorkspaceProjectScope = errors.New("task workspace route is app-scoped; project selection is an Outliner scope frame")
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
	case TaskWorkspace:
		return TaskWorkspaceParams{Scope: AppEntityScope()}
	case Reference:
		return ReferenceParams{}
	default:
		return nil
	}
}

func canonicalRouteParams(id RouteID, params RouteParams) (RouteParams, error) { //nolint:gocyclo
	switch id {
	case TaskWorkspace:
		p, ok := params.(TaskWorkspaceParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		// The Outliner root is the single app-scoped destination; a project scope
		// is an Outliner scope frame, never a second project-rooted TaskWorkspace.
		if p.Scope.Kind == ScopeProject {
			return nil, ErrTaskWorkspaceProjectScope
		}
		scope, err := canonicalEntityScope(id, p.Scope)
		if err != nil {
			return nil, err
		}
		p.Scope = scope
		return p, nil
	case SecondaryTool:
		switch p := params.(type) {
		case ReviewParams:
			if p.Module == ReviewLanding || p.Module > ReviewRoadmap {
				return nil, invalidParams(id, "unknown review tool")
			}
			return p, nil
		case ManageParams:
			if p.Module == ManageLanding || p.Module > ManageContexts {
				return nil, invalidParams(id, "unknown manage tool")
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
		default:
			return nil, paramsTypeError(id, params)
		}
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
	case TaskWorkspace, TaskDetail:
		return accentColorForCategory(AccentCatExecute)
	case SecondaryTool:
		if _, ok := r.params.(ReviewParams); ok {
			return accentColorForCategory(AccentCatReview)
		}
		return accentColorForCategory(AccentCatSupport)
	case Reference:
		return accentColorForCategory(AccentCatSupport)
	default:
		return nil
	}
}
