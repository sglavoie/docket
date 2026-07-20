package tui

import "image/color"

// This file introduces the declarative routing foundation for the TUI shell.
//
// Historically, navigation behavior for each ViewType was scattered across a
// handful of trait switches (isTrackedView, isGlobalNavView, isTaskScopedView,
// isGlobalOverlayView) and a side table (viewAccentMap). Every time a view was
// added, each of those switches had to be updated independently, and there was
// no single place that described what a route *is*.
//
// A RouteDescriptor collapses all of that into one declarative record per view:
// its identity, data scope, history behavior, overlay status, accent, and the
// factory used to construct its View. The legacy trait predicates are now thin
// readers over this table, so navigation behavior is data, not control flow.
//
// Route instances (a descriptor plus concrete parameters such as a project or
// task id) are the parameterized navigation targets that later feature-package
// migrations build on. The NavHistory bridge (NavEntry <-> Route) preserves the
// existing history semantics while expressing them through route instances.

// RouteScope classifies the data a route is bound to.
type RouteScope int

const (
	// ScopeApp routes are workspace-wide and carry no project or task binding
	// (the dashboard and every global overview/manager surface).
	ScopeApp RouteScope = iota
	// ScopeProject routes operate within the currently selected project.
	ScopeProject
	// ScopeTask routes are pinned to one specific task within a project.
	ScopeTask
)

// RouteHistory classifies how a route participates in linear back/forward
// navigation history (Alt+O / Alt+I).
type RouteHistory int

const (
	// HistoryNone routes are never recorded (pickers, editors, creators,
	// modal inputs, and transient overlays).
	HistoryNone RouteHistory = iota
	// HistoryTracked routes are recorded in NavHistory.
	HistoryTracked
)

// AccentCategory names the workflow-stage accent a route paints with. The
// concrete color is resolved from the active theme at read time, so category
// assignment is theme-independent while colors follow the palette.
type AccentCategory int

const (
	AccentCatNone AccentCategory = iota
	AccentCatDashboard
	AccentCatCapture
	AccentCatTriage
	AccentCatPlan
	AccentCatWork
	AccentCatExecute
	AccentCatReview
	AccentCatSupport
	AccentCatSearch
)

// RouteParams are the concrete bindings that turn a descriptor into a navigable
// instance. Empty fields mean "not applicable for this route's scope".
type RouteParams struct {
	ProjectID string
	TaskID    string
}

// RouteFactory constructs the View for a route from shared context and the
// route's concrete parameters. Factories are the seam that lets feature
// packages own their view construction without the shell reaching into their
// internals; they are populated incrementally as feature packages migrate.
type RouteFactory func(ctx ViewContext, params RouteParams) View

// RouteDescriptor is the declarative definition of a navigable route. It is the
// single source of truth that the legacy trait predicates and the accent map
// now read from.
type RouteDescriptor struct {
	ID            ViewType
	Name          string
	Scope         RouteScope
	History       RouteHistory
	GlobalNav     bool // reconstructed by reconstructGlobalView; never carries a ProjectID
	GlobalOverlay bool // launched via the ";" overlay prefix; returns to an anchor view
	Accent        AccentCategory
	Factory       RouteFactory
}

// Tracked reports whether the route is recorded in navigation history.
func (d RouteDescriptor) Tracked() bool { return d.History == HistoryTracked }

// trackedRoutes lists the views recorded in navigation history. Pickers,
// editors, creators, modals, and overlays are intentionally excluded.
var trackedRoutes = []ViewType{
	ViewDashboard,
	ViewDailyReview,
	ViewWeeklyReview,
	ViewProjectFocusView,
	ViewPriorityView,
	ViewRecentActivity,
	ViewGlobalSearch,
	ViewGlobalBlockers,
	ViewGlobalNotes,
	ViewGlobalInboxProcessor,
	ViewGlobalTagManager,
	ViewNotes,
	ViewBlockers,
	ViewInboxProcessor,
	ViewSettings,
	ViewWorkflowRules,
	ViewTaskTemplates,
	ViewTutor,
	ViewScoreProfileManager,
	ViewTaskComments,
	ViewTaskDecisions,
	ViewTaskChecklist,
	ViewTaskDetail,
	ViewTaskAttachments,
	ViewTaskActivity,
	ViewMetricsDashboard,
	ViewAllTasks,
	ViewOutliner,
	ViewCrossProjectOutliner,
	ViewTemporaryOutliner,
	ViewFocusBoard,
	ViewProjectRecentActivity,
	ViewRoadmap,
}

// globalNavRoutes lists the views reconstructed by reconstructGlobalView. Their
// nav entries never carry a ProjectID.
var globalNavRoutes = []ViewType{
	ViewDailyReview,
	ViewWeeklyReview,
	ViewRecentActivity,
	ViewGlobalSearch,
	ViewGlobalBlockers,
	ViewGlobalNotes,
	ViewGlobalInboxProcessor,
	ViewGlobalTagManager,
	ViewSettings,
	ViewWorkflowRules,
	ViewTaskTemplates,
	ViewTutor,
	ViewScoreProfileManager,
	ViewMetricsDashboard,
	ViewCrossProjectOutliner,
	ViewFocusBoard,
	ViewAllTasks,
	ViewRoadmap,
}

// taskScopedRoutes lists the views pinned to a specific task.
var taskScopedRoutes = []ViewType{
	ViewTaskComments,
	ViewTaskDecisions,
	ViewTaskChecklist,
	ViewTaskDetail,
	ViewTaskAttachments,
	ViewTaskActivity,
}

// globalOverlayRoutes lists the views launched via the global ";" overlay
// prefix. These can be opened from any context and return to an anchor view
// when closed.
var globalOverlayRoutes = []ViewType{
	ViewFocusBoard,
	ViewGlobalInboxProcessor,
	ViewGlobalNotes,
	ViewRecentActivity,
	ViewGlobalBlockers,
	ViewMetricsDashboard,
	ViewAllTasks,
	ViewWorkflowRules,
	ViewPipelineHub,
	ViewRoadmap,
}

// routeAccentCategories maps a view to its workflow-stage accent category.
// Unlisted views inherit their accent from a parent via the stack walk.
var routeAccentCategories = map[ViewType]AccentCategory{
	// Dashboard
	ViewDashboard: AccentCatDashboard,

	// Capture stage (coral/salmon)
	ViewQuickCapture: AccentCatCapture,
	ViewBulkCapture:  AccentCatCapture,

	// Triage stage (sky blue)
	ViewAllTasks:             AccentCatTriage,
	ViewGlobalInboxProcessor: AccentCatTriage,
	ViewInboxProcessor:       AccentCatTriage,
	ViewCrossProjectOutliner: AccentCatTriage,
	ViewCrossProjectKanban:   AccentCatTriage,

	// Plan stage (green)
	ViewProjectFocusView: AccentCatPlan,
	ViewDailyReview:      AccentCatPlan,
	ViewPriorityView:     AccentCatPlan,
	ViewRoadmap:          AccentCatPlan,

	// Work stage (orange)
	ViewFocusBoard: AccentCatWork,

	// Legacy Execute accent (teal) for project work views
	ViewOutliner:          AccentCatExecute,
	ViewTemporaryOutliner: AccentCatExecute,
	ViewKanban:            AccentCatExecute,

	// Review stage (yellow/amber)
	ViewWeeklyReview:     AccentCatReview,
	ViewMetricsDashboard: AccentCatReview,
	ViewRecentActivity:   AccentCatReview,

	// Support stage (muted gray)
	ViewGlobalBlockers:     AccentCatSupport,
	ViewGlobalNotes:        AccentCatSupport,
	ViewNotes:              AccentCatSupport,
	ViewWorkflowRules:      AccentCatSupport,
	ViewNotificationCenter: AccentCatSupport,

	// Search / Command (magenta)
	ViewGlobalSearch:   AccentCatSearch,
	ViewCommandPalette: AccentCatSearch,

	// Pipeline Hub (neutral — spans all stages)
	ViewPipelineHub: AccentCatDashboard,
}

// routeTable is the built descriptor table, keyed by ViewType. It is
// theme-independent (accent is stored as a category, not a color) so it can be
// constructed once at package init.
var routeTable = buildRouteTable()

// buildRouteTable assembles a RouteDescriptor for every ViewType from the
// declarative route sets above. Every view gets a descriptor so lookups never
// miss and drift is caught by TestRouteTableCoversEveryViewType.
func buildRouteTable() map[ViewType]RouteDescriptor {
	views := allViewTypes()
	table := make(map[ViewType]RouteDescriptor, len(views))
	for _, vt := range views {
		table[vt] = RouteDescriptor{
			ID:     vt,
			Name:   vt.String(),
			Scope:  ScopeProject,
			Accent: routeAccentCategories[vt],
		}
	}

	// Dashboard is workspace-wide.
	if d, ok := table[ViewDashboard]; ok {
		d.Scope = ScopeApp
		table[ViewDashboard] = d
	}
	for _, vt := range globalNavRoutes {
		d := table[vt]
		d.Scope = ScopeApp
		d.GlobalNav = true
		table[vt] = d
	}
	for _, vt := range taskScopedRoutes {
		d := table[vt]
		d.Scope = ScopeTask // overrides ScopeApp/Project
		table[vt] = d
	}
	for _, vt := range trackedRoutes {
		d := table[vt]
		d.History = HistoryTracked
		table[vt] = d
	}
	for _, vt := range globalOverlayRoutes {
		d := table[vt]
		d.GlobalOverlay = true
		table[vt] = d
	}
	// Populate the RouteFactory for the primary feature views that can be built
	// from ViewContext. Views without a registered factory keep Factory nil and
	// are still constructed by their legacy call sites.
	for vt, factory := range primaryFeatureFactories() {
		d := table[vt]
		d.Factory = factory
		table[vt] = d
	}
	return table
}

// routeFor returns the descriptor for a view, synthesizing a default
// project-scoped descriptor for any view missing from the table.
func routeFor(vt ViewType) RouteDescriptor {
	if d, ok := routeTable[vt]; ok {
		return d
	}
	return RouteDescriptor{ID: vt, Name: vt.String(), Scope: ScopeProject}
}

// accentColorForCategory resolves an accent category to the active theme color.
// Returns nil for AccentCatNone so unmapped routes fall through the stack walk.
func accentColorForCategory(c AccentCategory) color.Color {
	switch c {
	case AccentCatDashboard:
		return AccentDashboard
	case AccentCatCapture:
		return AccentCapture
	case AccentCatTriage:
		return AccentTriage
	case AccentCatPlan:
		return AccentPlan
	case AccentCatWork:
		return AccentWork
	case AccentCatExecute:
		return AccentExecute
	case AccentCatReview:
		return AccentReview
	case AccentCatSupport:
		return AccentSupport
	case AccentCatSearch:
		return AccentSearch
	default:
		return nil
	}
}

// --- Route instances -------------------------------------------------------

// Route is a parameterized navigation target: a descriptor plus the concrete
// project/task bindings for one navigation. It is the instance form that later
// feature migrations pass around in place of a bare ViewType.
type Route struct {
	Descriptor RouteDescriptor
	Params     RouteParams
}

// NewRoute builds a route instance for the given view and parameters, clearing
// any params the route's scope does not use so instances stay canonical.
func NewRoute(vt ViewType, params RouteParams) Route {
	d := routeFor(vt)
	// Global-nav routes never carry a ProjectID (they are reconstructed
	// globally); non-task routes never carry a TaskID.
	if d.GlobalNav {
		params.ProjectID = ""
	}
	if d.Scope != ScopeTask {
		params.TaskID = ""
	}
	return Route{Descriptor: d, Params: params}
}

// ID returns the route's view identity.
func (r Route) ID() ViewType { return r.Descriptor.ID }

// Tracked reports whether this route is recorded in navigation history.
func (r Route) Tracked() bool { return r.Descriptor.Tracked() }

// IsOverlay reports whether this route is a global ";" overlay.
func (r Route) IsOverlay() bool { return r.Descriptor.GlobalOverlay }

// Accent resolves the route's accent color from the active theme, or nil when
// the route inherits its accent from a parent.
func (r Route) Accent() color.Color { return accentColorForCategory(r.Descriptor.Accent) }

// NavEntry projects the route instance back onto the NavHistory entry form,
// preserving the exact ProjectID/TaskID semantics the shell records.
func (r Route) NavEntry() NavEntry {
	return NavEntry{ViewType: r.Descriptor.ID, ProjectID: r.Params.ProjectID, TaskID: r.Params.TaskID}
}

// routeFromNavEntry reconstructs a route instance from a recorded nav entry.
func routeFromNavEntry(e NavEntry) Route {
	return Route{
		Descriptor: routeFor(e.ViewType),
		Params:     RouteParams{ProjectID: e.ProjectID, TaskID: e.TaskID},
	}
}

// allViewTypes returns every registered ViewType in declaration order. It is
// the single ordered source used by the route table and its parity tests;
// TestAllViewTypesMatchNames guards it against drift from viewTypeNames.
func allViewTypes() []ViewType {
	return []ViewType{
		ViewDashboard,
		ViewProjectList,
		ViewProjectCreator,
		ViewProjectEditor,
		ViewSettings,
		ViewHelp,
		ViewDateInput,
		ViewDependencySelector,
		ViewDependencyViewer,
		ViewFilter,
		ViewTaskJumper,
		ViewProjectPreview,
		ViewTagManager,
		ViewGlobalTagManager,
		ViewTagFilterSelector,
		ViewSectionFilterSelector,
		ViewSectionPicker,
		ViewSectionManager,
		ViewPriorityPicker,
		ViewContextPicker,
		ViewCustomContextEditor,
		ViewGlobalSearch,
		ViewParentSelector,
		ViewPriorityView,
		ViewQuickCapture,
		ViewBulkCapture,
		ViewInboxProcessor,
		ViewTagSelector,
		ViewBulkTagSelector,
		ViewTaskComments,
		ViewCommandPalette,
		ViewBulkEdit,
		ViewSaveSearchInput,
		ViewTrashedProjects,
		ViewProjectSelector,
		ViewDailyReview,
		ViewWorkflowRules,
		ViewWorkflowRuleCreator,
		ViewWorkflowRuleEditor,
		ViewWorkflowWizard,
		ViewRecurrenceEditor,
		ViewTextEditor,
		ViewGlobalInboxProcessor,
		ViewProjectFocusView,
		ViewBlockers,
		ViewTaskAttachments,
		ViewLinkedTaskCreator,
		ViewLinkedTaskSelector,
		ViewCrossProjectLinker,
		ViewNotes,
		ViewNoteTaskLinker,
		ViewNoteTagSelector,
		ViewNotesSearch,
		ViewTaskActivity,
		ViewRecentActivity,
		ViewRecentActivityPreview,
		ViewProjectRecentActivity,
		ViewGlobalNotes,
		ViewWeeklyReview,
		ViewGlobalBlockers,
		ViewSnoozeInput,
		ViewParkInput,
		ViewTutor,
		ViewAsk,
		ViewProjectPicker,
		ViewTaskTemplates,
		ViewTaskTemplateCreator,
		ViewTaskTemplateEditor,
		ViewTaskTemplatePicker,
		ViewTaskChecklist,
		ViewTaskDetail,
		ViewScoreEditor,
		ViewScoreProfileManager,
		ViewSaveFilterInput,
		ViewMetricsDashboard,
		ViewAllTasks,
		ViewScoringProfilePicker,
		ViewFocusBoard,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
		ViewColumnPicker,
		ViewFilterBuilder,
		ViewWorkflowDryRun,
		ViewTemplateVarFiller,
		ViewStatusInput,
		ViewStatusIconPicker,
		ViewNotificationCenter,
		ViewMessageLog,
		ViewKeybindingExplorer,
		ViewProjectFilterSelector,
		ViewTaskForm,
		ViewKanban,
		ViewCrossProjectKanban,
		ViewPipelineHub,
		ViewRoadmap,
		ViewTaskDecisions,
		ViewAttrFilterPicker,
		ViewConflictResolution,
	}
}
