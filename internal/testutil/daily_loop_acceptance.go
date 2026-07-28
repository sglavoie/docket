package testutil

import "github.com/sglavoie/tt/internal/models"

// LegacyLifecycleFixture is one sanitized schema-v1 lifecycle input and the
// canonical state it must migrate to. These fixtures intentionally describe
// retired status/section/stage/active inputs so storage, config, CLI, and TUI
// migration tests can share one acceptance matrix.
type LegacyLifecycleFixture struct {
	ID      string
	Status  string
	Section string
	Stage   string
	Active  bool
	State   models.WorkflowState
}

// DailyLoopLegacyLifecycleFixtures returns a fresh copy of the accepted
// schema-v1 migration inputs. The IDs match the hand-authored workspace under
// internal/storage/testdata/legacy_workspace.
func DailyLoopLegacyLifecycleFixtures() []LegacyLifecycleFixture {
	return []LegacyLifecycleFixture{
		{ID: "legacy-cancel-1", Status: "canceled", State: models.StateCanceled},
		{ID: "legacy-done-1", Status: "done", State: models.StateDone},
		{ID: "legacy-done-2", Status: "todo", Section: "Completed", State: models.StateDone},
		{ID: "legacy-review-1", Status: "review", State: models.StateReview},
		{ID: "legacy-review-section-1", Status: "todo", Section: "Review", State: models.StateReview},
		{ID: "legacy-inprogress-1", Status: "in_progress", State: models.StateInProgress},
		{ID: "legacy-blocked-1", Status: "blocked", State: models.StateWaiting},
		{ID: "legacy-waiting-1", Status: "todo", Section: "Waiting", State: models.StateWaiting},
		{ID: "legacy-standby-1", Status: "standby", Section: "Standby", State: models.StatePaused},
		{ID: "legacy-maybe-1", Status: "maybe", Section: "Someday", State: models.StateSomeday},
		{ID: "legacy-inbox-1", Status: "todo", Section: "Inbox", State: models.StateInbox},
		{ID: "legacy-next-1", Status: "todo", Section: "Next", State: models.StateNext},
		{ID: "legacy-active-1", Status: "todo", Active: true, State: models.StateReady},
		{ID: "legacy-stage-1", Status: "todo", Stage: "Brainstorm", State: models.StateReady},
	}
}

// WorkflowStateAcceptance is the representative legacy signal for one
// canonical state. The full precedence and conflict matrix remains in
// internal/models/task_workflowstate_test.go; this compact inventory is the
// cross-package acceptance seam.
type WorkflowStateAcceptance struct {
	State         models.WorkflowState
	LegacyStatus  string
	LegacySection string
	LegacyStage   string
	Terminal      bool
}

// DailyLoopWorkflowStateAcceptance returns the ten canonical states in their
// accepted public order.
func DailyLoopWorkflowStateAcceptance() []WorkflowStateAcceptance {
	return []WorkflowStateAcceptance{
		{State: models.StateInbox, LegacyStatus: "todo", LegacySection: "Inbox"},
		{State: models.StateReady, LegacyStatus: "todo"},
		{State: models.StateNext, LegacyStatus: "todo", LegacySection: "Next"},
		{State: models.StateInProgress, LegacyStatus: "in_progress"},
		{State: models.StateWaiting, LegacyStatus: "blocked", LegacySection: "Waiting"},
		{State: models.StatePaused, LegacyStatus: "standby", LegacySection: "Standby"},
		{State: models.StateReview, LegacyStatus: "review", LegacySection: "Review", LegacyStage: "Review"},
		{State: models.StateSomeday, LegacyStatus: "maybe", LegacySection: "Someday"},
		{State: models.StateDone, LegacyStatus: "done", LegacySection: "Completed", Terminal: true},
		{State: models.StateCanceled, LegacyStatus: "canceled", Terminal: true},
	}
}

// PlanningTransitionFixture records the accepted Today/Next/state result of a
// user planning operation. Next membership is represented explicitly as well as
// by StateNext so tests can assert the mutual-exclusion contract.
type PlanningTransitionFixture struct {
	Name         string
	Operation    string
	InitialState models.WorkflowState
	InitialToday bool
	InitialNext  bool
	WantState    models.WorkflowState
	WantToday    bool
	WantNext     bool
}

// DailyLoopPlanningTransitionFixtures returns the behavior-level planning
// matrix later state-mutation sessions must implement.
func DailyLoopPlanningTransitionFixtures() []PlanningTransitionFixture {
	return []PlanningTransitionFixture{
		{Name: "backlog clears today", Operation: "backlog", InitialState: models.StateReady, InitialToday: true, WantState: models.StateReady},
		{Name: "backlog removes next", Operation: "backlog", InitialState: models.StateNext, InitialNext: true, WantState: models.StateReady},
		{Name: "next clears today", Operation: "next", InitialState: models.StateReady, InitialToday: true, WantState: models.StateNext, WantNext: true},
		{Name: "next toggles to ready", Operation: "next", InitialState: models.StateNext, InitialNext: true, WantState: models.StateReady},
		{Name: "today promotes inbox", Operation: "today-add", InitialState: models.StateInbox, WantState: models.StateReady, WantToday: true},
		{Name: "today promotes next", Operation: "today-add", InitialState: models.StateNext, InitialNext: true, WantState: models.StateReady, WantToday: true},
		{Name: "today keeps ready", Operation: "today-add", InitialState: models.StateReady, WantState: models.StateReady, WantToday: true},
		{Name: "today removal preserves state", Operation: "today-remove", InitialState: models.StatePaused, InitialToday: true, WantState: models.StatePaused},
		{Name: "start preserves today", Operation: "start", InitialState: models.StateReady, InitialToday: true, WantState: models.StateInProgress, WantToday: true},
		{Name: "done clears planning", Operation: "done", InitialState: models.StateInProgress, InitialToday: true, WantState: models.StateDone},
	}
}

// CreationAcceptance records how each creation surface resolves ownership and
// the state used when no explicit state is supplied.
type CreationAcceptance struct {
	Surface       string
	ProjectTarget string
	DefaultState  models.WorkflowState
}

// DailyLoopCreationAcceptance returns the accepted ownership/default matrix.
func DailyLoopCreationAcceptance() []CreationAcceptance {
	return []CreationAcceptance{
		{Surface: "base-scope capture", ProjectTarget: "workspace-owned System Inbox", DefaultState: models.StateInbox},
		{Surface: "project-focus capture", ProjectTarget: "focused project", DefaultState: models.StateInbox},
		{Surface: "task creation", ProjectTarget: "resolved project", DefaultState: models.StateInbox},
		{Surface: "template creation", ProjectTarget: "resolved project", DefaultState: models.StateInbox},
		{Surface: "action-plan create_task", ProjectTarget: "resolved project", DefaultState: models.StateInbox},
	}
}

// KeyAcceptance is one accepted stable TUI binding.
type KeyAcceptance struct {
	Key    string
	Action string
	Area   string
}

// PublicInterfaceAcceptance records the state vocabulary that replaces each
// legacy persistence or public-interface shape.
type PublicInterfaceAcceptance struct {
	CreationFlag            string
	ListArchiveFilter       string
	SortField               string
	TaskJSONLifecycleFields []string
	TaskJSONForbiddenFields []string
	ProjectJSONForbidden    []string
	SearchTokenExample      string
	TemplateField           string
	WorkflowVocabulary      []string
	ActionPlanVersion       int
	CreateTaskStateOptional bool
}

// DailyLoopContract is the accepted end-state UI and public-interface contract.
// It is deliberately test support rather than production configuration: session
// 1 records the destination while later sessions migrate production toward it.
type DailyLoopContract struct {
	Modes          []string
	TabCycle       []string
	GroupOrder     []string
	InitialMode    string
	InitialScope   string
	Keys           []KeyAcceptance
	Public         PublicInterfaceAcceptance
	Interaction    []string
	ScopeRules     []string
	Visibility     []string
	InteractionMax int
}

// DailyLoopTargetContract returns a fresh copy of the accepted daily-loop
// contract so later migration tests can compare production behavior against it.
func DailyLoopTargetContract() DailyLoopContract {
	return DailyLoopContract{
		Modes:        []string{"All", "Inbox", "Next", "Backlog", "Due", "Priority", "Blocked", "Parked", "Daily Review", "Activity"},
		TabCycle:     []string{"All", "Inbox", "Next", "Backlog"},
		GroupOrder:   []string{"Today", "In Progress", "Inbox", "Next", "Backlog", "Review", "Waiting", "Paused", "Someday"},
		InitialMode:  "All",
		InitialScope: "All Projects",
		Keys: []KeyAcceptance{
			{Key: "a", Action: "capture", Area: "task loop"},
			{Key: "e", Action: "edit", Area: "task loop"},
			{Key: "s", Action: "start", Area: "task loop"},
			{Key: "d", Action: "done", Area: "task loop"},
			{Key: "b", Action: "backlog", Area: "task loop"},
			{Key: "t", Action: "today", Area: "task loop"},
			{Key: "n", Action: "next", Area: "task loop"},
			{Key: "space", Action: "actions", Area: "task loop"},
			{Key: "j/k", Action: "move", Area: "navigation"},
			{Key: "h/l or arrows", Action: "collapse/expand", Area: "navigation"},
			{Key: "J/K", Action: "reorder", Area: "navigation"},
			{Key: "enter", Action: "detail", Area: "navigation"},
			{Key: "/", Action: "visible search/filter", Area: "discovery"},
			{Key: ":", Action: "global finder", Area: "discovery"},
			{Key: "m", Action: "modes", Area: "discovery"},
			{Key: "c", Action: "scope", Area: "discovery"},
			{Key: "v", Action: "multi-select", Area: "discovery"},
			{Key: "N", Action: "notes", Area: "core tools"},
			{Key: "R", Action: "weekly review", Area: "core tools"},
			{Key: "T", Action: "tags", Area: "core tools"},
			{Key: "!", Action: "notifications", Area: "core tools"},
			{Key: "u", Action: "undo", Area: "session"},
			{Key: "ctrl+r", Action: "redo", Area: "session"},
			{Key: "esc", Action: "back", Area: "session"},
			{Key: "ctrl+esc", Action: "reset", Area: "session"},
			{Key: "?", Action: "help", Area: "session"},
			{Key: "ctrl+c", Action: "quit", Area: "session"},
		},
		Public: PublicInterfaceAcceptance{
			CreationFlag:            "--state",
			ListArchiveFilter:       "state",
			SortField:               "state",
			TaskJSONLifecycleFields: []string{"state"},
			TaskJSONForbiddenFields: []string{"status", "section", "stage"},
			ProjectJSONForbidden:    []string{"sections", "section_colors"},
			SearchTokenExample:      "$ready",
			TemplateField:           "state",
			WorkflowVocabulary:      []string{"state", "state-set", "state-transition", "set_state"},
			ActionPlanVersion:       2,
			CreateTaskStateOptional: true,
		},
		Interaction: []string{
			"normal space opens contextual action sheet",
			"multi-select space toggles selection",
			"multi-select enter opens bulk actions",
			"delete exists only in the action sheet and requires confirmation",
			"reorder is limited to one project and manual All/Inbox/Next/Backlog order",
			"ctrl+esc clears children, overlays, transient filters, search, and selection",
		},
		ScopeRules: []string{
			"project focus opens project notes; base scope opens global scratch notes",
			"weekly review and tags use the active scope project set",
			"notifications remain global",
		},
		Visibility: []string{
			"Today is synthetic and deduplicated",
			"Today contains only nonterminal non-Inbox tasks",
			"done and canceled require explicit filters",
			"pinned and recent destinations are finder results",
		},
		InteractionMax: 2,
	}
}
