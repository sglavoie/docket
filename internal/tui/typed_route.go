package tui

import (
	"errors"
	"fmt"
	"image/color"
	"reflect"
	"strings"
)

// RouteID is the stable identity of a primary TUI surface. Modes and entity
// bindings live in sealed RouteParams values rather than multiplying route
// identities.
type RouteID uint8

const (
	TaskWorkspace RouteID = iota
	SecondaryTool
	Reference
)

var routeIDNames = [...]string{
	"TaskWorkspace",
	"SecondaryTool",
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
// destination type; retired primary routes have been deleted.
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

type ManageModule uint8

const (
	ManageNotes ManageModule = iota
	ManageContexts
	ManageProjects
)

// NotesScopeKind identifies the collection subset selected by the workspace-wide
// Notes manager. Unlike EntityScope, it preserves the meaningful distinction
// between All Notes and the workspace-global note collection.
type NotesScopeKind uint8

const (
	NotesScopeAll NotesScopeKind = iota
	NotesScopeGlobal
	NotesScopeProject
)

// NotesScope selects All Notes, Global, or one active project. ProjectID is
// meaningful only for project scope.
type NotesScope struct {
	Kind      NotesScopeKind
	ProjectID string
}

func AllNotesScope() NotesScope { return NotesScope{Kind: NotesScopeAll} }

func GlobalNotesScope() NotesScope { return NotesScope{Kind: NotesScopeGlobal} }

func ProjectNotesScope(projectID string) NotesScope {
	return NotesScope{Kind: NotesScopeProject, ProjectID: projectID}
}

type ManageParams struct {
	Module     ManageModule
	NotesScope NotesScope
}

func (ManageParams) routeParams() {}

type ReferenceKind uint8

const (
	ReferenceHelp ReferenceKind = iota
)

type ReferenceParams struct{ Kind ReferenceKind }

func (ReferenceParams) routeParams() {}

var (
	ErrInvalidRouteID     = errors.New("invalid route ID")
	ErrRouteParamsType    = errors.New("route parameter type does not match route ID")
	ErrInvalidRouteParams = errors.New("invalid route parameters")
	ErrProjectRequired    = errors.New("project-scoped route requires a project ID")
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
		case ManageParams:
			if p.Module > ManageProjects {
				return nil, invalidParams(id, "unknown manage tool")
			}
			if p.Module == ManageNotes {
				scope, err := canonicalNotesScope(id, p.NotesScope)
				if err != nil {
					return nil, err
				}
				p.NotesScope = scope
			} else {
				p.NotesScope = NotesScope{}
			}
			return p, nil
		default:
			return nil, paramsTypeError(id, params)
		}
	case Reference:
		p, ok := params.(ReferenceParams)
		if !ok {
			return nil, paramsTypeError(id, params)
		}
		if p.Kind > ReferenceHelp {
			return nil, invalidParams(id, "unknown reference kind")
		}
		return p, nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrInvalidRouteID, id)
	}
}

func manageParamsEqual(left, right ManageParams) bool {
	return left == right
}

func canonicalNotesScope(id RouteID, scope NotesScope) (NotesScope, error) {
	switch scope.Kind {
	case NotesScopeAll, NotesScopeGlobal:
		scope.ProjectID = ""
		return scope, nil
	case NotesScopeProject:
		if strings.TrimSpace(scope.ProjectID) == "" {
			return NotesScope{}, ErrProjectRequired
		}
		return scope, nil
	default:
		return NotesScope{}, invalidParams(id, "notes scope must be all, global, or project")
	}
}

func (scope NotesScope) commandScope() EntityScope {
	if scope.Kind == NotesScopeProject {
		return ProjectEntityScope(scope.ProjectID)
	}
	return AppEntityScope()
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
	case TaskWorkspace:
		return accentColorForCategory(AccentCatExecute)
	case SecondaryTool:
		return accentColorForCategory(AccentCatSupport)
	case Reference:
		return accentColorForCategory(AccentCatSupport)
	default:
		return nil
	}
}
