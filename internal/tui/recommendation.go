package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// RecommendationKind classifies the category of next action the engine
// suggests to the user. The Today cockpit (sessions 4-6) renders one kind at
// a time and chooses copy/affordances based on the kind.
type RecommendationKind string

const (
	// RecommendationKindNone is the zero value: no recommendation is available
	// (no projects, no tasks, or the engine has not produced output yet).
	RecommendationKindNone RecommendationKind = ""

	// RecommendationKindWorkOnTask suggests resuming or starting work on the
	// task carried in RecommendedAction.Task. Most populated cockpit states
	// land here.
	RecommendationKindWorkOnTask RecommendationKind = "work_on_task"

	// RecommendationKindTriageInbox suggests reviewing inbox-section tasks
	// because nothing is planned for today and the inbox has actionable items.
	RecommendationKindTriageInbox RecommendationKind = "triage_inbox"

	// RecommendationKindReviewBlocked suggests revisiting a blocked task to
	// either unblock it or move it elsewhere.
	RecommendationKindReviewBlocked RecommendationKind = "review_blocked"

	// RecommendationKindPlanToday suggests planning the day because tasks
	// exist but none are scheduled for today.
	RecommendationKindPlanToday RecommendationKind = "plan_today"

	// RecommendationKindAllDone signals every planned task for today is
	// complete and nothing else needs immediate attention.
	RecommendationKindAllDone RecommendationKind = "all_done"
)

// IsValid reports whether k is one of the known recommendation kinds.
// RecommendationKindNone is considered valid because callers may legitimately
// hold a zero-value recommendation while loading.
func (k RecommendationKind) IsValid() bool {
	switch k {
	case RecommendationKindNone,
		RecommendationKindWorkOnTask,
		RecommendationKindTriageInbox,
		RecommendationKindReviewBlocked,
		RecommendationKindPlanToday,
		RecommendationKindAllDone:
		return true
	}
	return false
}

// String returns the string form of the kind.
func (k RecommendationKind) String() string { return string(k) }

// AllRecommendationKinds returns every known recommendation kind in the order
// the engine evaluates them. RecommendationKindNone is omitted because it is
// the zero value, not a state the engine actively produces.
func AllRecommendationKinds() []RecommendationKind {
	return []RecommendationKind{
		RecommendationKindWorkOnTask,
		RecommendationKindTriageInbox,
		RecommendationKindReviewBlocked,
		RecommendationKindPlanToday,
		RecommendationKindAllDone,
	}
}

// RecommendationReason is an explicit, user-facing rationale attached to a
// RecommendedAction. The Today cockpit renders these verbatim as the
// "because…" line under the next-action strip, so values must read as
// English copy and stay short enough for a single visual line.
type RecommendationReason string

const (
	// ReasonPlannedToday — task is on the daily plan for today.
	ReasonPlannedToday RecommendationReason = "planned today"

	// ReasonOverdue — task has a due date that has already passed.
	ReasonOverdue RecommendationReason = "overdue"

	// ReasonDueToday — task is due today (same calendar day as the engine clock).
	ReasonDueToday RecommendationReason = "due today"

	// ReasonHighPriority — task carries priority 1 or 2 (critical/high).
	ReasonHighPriority RecommendationReason = "high priority"

	// ReasonInboxNeedsTriage — actionable tasks sit in the Inbox section
	// awaiting GTD-style triage.
	ReasonInboxNeedsTriage RecommendationReason = "inbox needs triage"

	// ReasonBlocked — task status is models.StatusBlocked or at least one
	// dependency is incomplete or missing.
	ReasonBlocked RecommendationReason = "blocked"

	// ReasonStale — task has not been updated for a long stretch (engine
	// chooses the threshold; the reason copy is stable).
	ReasonStale RecommendationReason = "stale"

	// ReasonCarriedOver — task was planned for a previous day and remains
	// open today.
	ReasonCarriedOver RecommendationReason = "carried over"

	// ReasonRecentlyActive — task has had user activity inside the engine's
	// recent-momentum window.
	ReasonRecentlyActive RecommendationReason = "recently active"
)

// IsValid reports whether r is one of the known recommendation reasons.
func (r RecommendationReason) IsValid() bool {
	switch r {
	case ReasonPlannedToday,
		ReasonOverdue,
		ReasonDueToday,
		ReasonHighPriority,
		ReasonInboxNeedsTriage,
		ReasonBlocked,
		ReasonStale,
		ReasonCarriedOver,
		ReasonRecentlyActive:
		return true
	}
	return false
}

// String returns the string form of the reason — the same value the cockpit
// renders. Label is provided as an alias for callers that prefer the
// presentation-oriented name without committing future copy changes to the
// String() contract.
func (r RecommendationReason) String() string { return string(r) }

// Label returns the user-facing label for the reason. Identical to String()
// today; kept separate so the rendering layer can swap copy without touching
// every comparison site.
func (r RecommendationReason) Label() string { return string(r) }

// AllRecommendationReasons returns every known recommendation reason in
// engine-evaluation order. The cockpit copy and contract tests depend on the
// full set being enumerable.
func AllRecommendationReasons() []RecommendationReason {
	return []RecommendationReason{
		ReasonPlannedToday,
		ReasonOverdue,
		ReasonDueToday,
		ReasonHighPriority,
		ReasonInboxNeedsTriage,
		ReasonBlocked,
		ReasonStale,
		ReasonCarriedOver,
		ReasonRecentlyActive,
	}
}

// RecommendedAction is a deterministic snapshot produced by the recommendation
// engine. Task and Project are populated for per-task kinds
// (WorkOnTask, ReviewBlocked); they are nil for cockpit-wide kinds
// (TriageInbox, PlanToday, AllDone). GeneratedAt records the engine clock so
// the cockpit can show "as of HH:MM" without recomputing.
//
// Reasons is the ordered list of rationales the cockpit will render. It must
// contain only RecommendationReason values that pass IsValid(); duplicates are
// preserved so the engine can intentionally emphasize a rationale (e.g.,
// "overdue" twice when both due-date and carry-over windows match).
type RecommendedAction struct {
	Kind        RecommendationKind
	Task        *models.Task
	Project     *models.Project
	Reasons     []RecommendationReason
	GeneratedAt time.Time
}

// HasReason reports whether the action carries the given reason. Nil-receiver
// safe so callers can chain on engine output without intermediate guards.
func (a *RecommendedAction) HasReason(r RecommendationReason) bool {
	if a == nil {
		return false
	}
	for _, candidate := range a.Reasons {
		if candidate == r {
			return true
		}
	}
	return false
}

// IsEmpty reports whether the action is the zero value or carries no kind
// and no task. Nil-receiver safe.
func (a *RecommendedAction) IsEmpty() bool {
	if a == nil {
		return true
	}
	return a.Kind == RecommendationKindNone && a.Task == nil && a.Project == nil && len(a.Reasons) == 0
}

// CockpitTargetKind describes what a recommendation points at. Task and
// Project targets carry model pointers; Cockpit targets describe whole-surface
// actions such as inbox triage, planning, or first-run setup.
type CockpitTargetKind string

const (
	CockpitTargetNone    CockpitTargetKind = ""
	CockpitTargetTask    CockpitTargetKind = "task"
	CockpitTargetProject CockpitTargetKind = "project"
	CockpitTargetSurface CockpitTargetKind = "surface"
)

// IsValid reports whether k is one of the known target kinds.
func (k CockpitTargetKind) IsValid() bool {
	switch k {
	case CockpitTargetNone, CockpitTargetTask, CockpitTargetProject, CockpitTargetSurface:
		return true
	}
	return false
}

// CockpitTarget identifies the task/project/surface a cockpit action operates
// on. It lets renderers and key handlers share the same target contract
// instead of re-inferring it from a kind switch.
type CockpitTarget struct {
	Kind    CockpitTargetKind
	Task    *models.Task
	Project *models.Project
}

// IsEmpty reports whether the target is the zero target.
func (t CockpitTarget) IsEmpty() bool {
	return t.Kind == CockpitTargetNone && t.Task == nil && t.Project == nil
}

// CockpitCommand is a stable command identifier used by the Today cockpit
// contract. Keys and labels are presentation details carried by CockpitAction.
type CockpitCommand string

const (
	CockpitCommandNone               CockpitCommand = ""
	CockpitCommandOpenTask           CockpitCommand = "open_task"
	CockpitCommandCompleteTask       CockpitCommand = "complete_task"
	CockpitCommandDismissTask        CockpitCommand = "dismiss_task"
	CockpitCommandPrioritizeTask     CockpitCommand = "prioritize_task"
	CockpitCommandPlanToday          CockpitCommand = "plan_today"
	CockpitCommandPlanMore           CockpitCommand = "plan_more"
	CockpitCommandTriageInbox        CockpitCommand = "triage_inbox"
	CockpitCommandUnblockTask        CockpitCommand = "unblock_task"
	CockpitCommandQuickCapture       CockpitCommand = "quick_capture"
	CockpitCommandSearchFilter       CockpitCommand = "search_filter"
	CockpitCommandNewProject         CockpitCommand = "new_project"
	CockpitCommandSwitchTab          CockpitCommand = "switch_tab"
	CockpitCommandCloseView          CockpitCommand = "close_view"
	CockpitCommandOpenPipelineHub    CockpitCommand = "open_pipeline_hub"
	CockpitCommandOpenCommandPalette CockpitCommand = "open_command_palette"
	CockpitCommandShowHelp           CockpitCommand = "show_help"
)

// IsValid reports whether c is one of the known cockpit commands.
func (c CockpitCommand) IsValid() bool {
	switch c {
	case CockpitCommandNone,
		CockpitCommandOpenTask,
		CockpitCommandCompleteTask,
		CockpitCommandDismissTask,
		CockpitCommandPrioritizeTask,
		CockpitCommandPlanToday,
		CockpitCommandPlanMore,
		CockpitCommandTriageInbox,
		CockpitCommandUnblockTask,
		CockpitCommandQuickCapture,
		CockpitCommandSearchFilter,
		CockpitCommandNewProject,
		CockpitCommandSwitchTab,
		CockpitCommandCloseView,
		CockpitCommandOpenPipelineHub,
		CockpitCommandOpenCommandPalette,
		CockpitCommandShowHelp:
		return true
	}
	return false
}

// AllCockpitCommands returns every executable cockpit command in stable order.
// CockpitCommandNone is omitted because it represents no action.
func AllCockpitCommands() []CockpitCommand {
	return []CockpitCommand{
		CockpitCommandOpenTask,
		CockpitCommandCompleteTask,
		CockpitCommandDismissTask,
		CockpitCommandPrioritizeTask,
		CockpitCommandPlanToday,
		CockpitCommandPlanMore,
		CockpitCommandTriageInbox,
		CockpitCommandUnblockTask,
		CockpitCommandQuickCapture,
		CockpitCommandSearchFilter,
		CockpitCommandNewProject,
		CockpitCommandSwitchTab,
		CockpitCommandCloseView,
		CockpitCommandOpenPipelineHub,
		CockpitCommandOpenCommandPalette,
		CockpitCommandShowHelp,
	}
}

// CockpitAction binds a stable command identifier to the key and label a
// surface can render in the strip, empty state, or footer.
type CockpitAction struct {
	Command CockpitCommand
	Key     string
	Label   string
}

// IsZero reports whether the action has no command, key, or label.
func (a CockpitAction) IsZero() bool {
	return a.Command == CockpitCommandNone && a.Key == "" && a.Label == ""
}

// Hint returns the key-label form used by footer and strip hint renderers.
func (a CockpitAction) Hint() string {
	switch {
	case a.Key == "":
		return a.Label
	case a.Label == "":
		return a.Key
	default:
		return a.Key + " " + a.Label
	}
}

// CockpitFooterActions separates context-specific footer actions from the
// global escape hatches. The order of each slice is render order.
type CockpitFooterActions struct {
	Contextual []CockpitAction
	Escape     []CockpitAction
}

// All returns a copy of every footer action in render order.
func (f CockpitFooterActions) All() []CockpitAction {
	out := make([]CockpitAction, 0, len(f.Contextual)+len(f.Escape))
	out = append(out, f.Contextual...)
	out = append(out, f.Escape...)
	return out
}

// CockpitProgressKind classifies the Today's-plan progress state shown in the
// title row and footer decisions.
type CockpitProgressKind string

const (
	CockpitProgressNone      CockpitProgressKind = ""
	CockpitProgressUnplanned CockpitProgressKind = "unplanned"
	CockpitProgressPlanned   CockpitProgressKind = "planned"
	CockpitProgressComplete  CockpitProgressKind = "complete"
)

// IsValid reports whether k is one of the known progress states.
func (k CockpitProgressKind) IsValid() bool {
	switch k {
	case CockpitProgressNone, CockpitProgressUnplanned, CockpitProgressPlanned, CockpitProgressComplete:
		return true
	}
	return false
}

// CockpitProgressState captures Today's done/total state without forcing
// renderers to inspect OpenLoops directly.
type CockpitProgressState struct {
	Kind  CockpitProgressKind
	Done  int
	Total int
}

// CockpitEmptyState names the empty-body variant paired with a recommendation.
type CockpitEmptyState string

const (
	CockpitEmptyStateNone         CockpitEmptyState = ""
	CockpitEmptyStateFirstRun     CockpitEmptyState = "first_run"
	CockpitEmptyStateInboxOnly    CockpitEmptyState = "inbox_only"
	CockpitEmptyStateBacklogReady CockpitEmptyState = "backlog_ready"
	CockpitEmptyStateTodayCleared CockpitEmptyState = "today_cleared"
)

// IsValid reports whether s is one of the known empty-state variants.
func (s CockpitEmptyState) IsValid() bool {
	switch s {
	case CockpitEmptyStateNone,
		CockpitEmptyStateFirstRun,
		CockpitEmptyStateInboxOnly,
		CockpitEmptyStateBacklogReady,
		CockpitEmptyStateTodayCleared:
		return true
	}
	return false
}

// CockpitContext supplies surface-level facts that the recommendation engine
// intentionally does not know, such as first-run vs caught-up and footer
// progress. It mirrors the Today/Dashboard/Hub inputs currently passed to the
// strip renderer.
type CockpitContext struct {
	HasProjects bool
	OpenLoops   OpenLoops
}

// CockpitRecommendation is the resolved contract consumed by cockpit
// renderers and key handlers: the engine kind plus target, rationale, actions,
// progress, empty-state, and footer models.
type CockpitRecommendation struct {
	Kind            RecommendationKind
	Target          CockpitTarget
	Rationale       []RecommendationReason
	PrimaryAction   CockpitAction
	SecondaryAction CockpitAction
	Progress        CockpitProgressState
	EmptyState      CockpitEmptyState
	Footer          CockpitFooterActions
}

// BuildCockpitRecommendation resolves the engine output plus surface context
// into the full cockpit contract. It does not mutate tasks or projects.
func BuildCockpitRecommendation(action RecommendedAction, context CockpitContext) CockpitRecommendation {
	rec := CockpitRecommendation{
		Kind:      action.Kind,
		Target:    cockpitTargetFor(action),
		Rationale: append([]RecommendationReason(nil), action.Reasons...),
		Progress:  cockpitProgressFor(context.OpenLoops),
	}
	rec.EmptyState = cockpitEmptyStateFor(action, context)
	rec.PrimaryAction, rec.SecondaryAction = cockpitActionsFor(action, rec.EmptyState)
	rec.Footer = cockpitFooterFor(rec.EmptyState)
	return rec
}

func cockpitTargetFor(action RecommendedAction) CockpitTarget {
	switch action.Kind {
	case RecommendationKindWorkOnTask, RecommendationKindReviewBlocked:
		return CockpitTarget{
			Kind:    CockpitTargetTask,
			Task:    action.Task,
			Project: action.Project,
		}
	case RecommendationKindTriageInbox, RecommendationKindPlanToday, RecommendationKindAllDone:
		return CockpitTarget{Kind: CockpitTargetSurface}
	case RecommendationKindNone:
	}
	if action.Project != nil {
		return CockpitTarget{Kind: CockpitTargetProject, Project: action.Project}
	}
	return CockpitTarget{}
}

func cockpitProgressFor(openLoops OpenLoops) CockpitProgressState {
	if openLoops.TodayTotal <= 0 {
		return CockpitProgressState{Kind: CockpitProgressUnplanned}
	}
	if openLoops.TodayDone >= openLoops.TodayTotal {
		return CockpitProgressState{
			Kind:  CockpitProgressComplete,
			Done:  openLoops.TodayDone,
			Total: openLoops.TodayTotal,
		}
	}
	return CockpitProgressState{
		Kind:  CockpitProgressPlanned,
		Done:  openLoops.TodayDone,
		Total: openLoops.TodayTotal,
	}
}

func cockpitEmptyStateFor(action RecommendedAction, context CockpitContext) CockpitEmptyState {
	if context.OpenLoops.TodayTotal > 0 && context.OpenLoops.TodayDone >= context.OpenLoops.TodayTotal {
		return CockpitEmptyStateTodayCleared
	}
	if context.OpenLoops.Inbox > 0 || action.Kind == RecommendationKindTriageInbox {
		return CockpitEmptyStateInboxOnly
	}
	if action.Kind == RecommendationKindAllDone && !context.HasProjects && !context.OpenLoops.HasTasks {
		return CockpitEmptyStateFirstRun
	}
	if action.Kind == RecommendationKindPlanToday {
		return CockpitEmptyStateBacklogReady
	}
	return CockpitEmptyStateNone
}

func cockpitActionsFor(action RecommendedAction, emptyState CockpitEmptyState) (CockpitAction, CockpitAction) {
	switch emptyState {
	case CockpitEmptyStateFirstRun:
		return actionNewProject(), actionQuickCapture()
	case CockpitEmptyStateTodayCleared:
		return actionPlanMore(), actionQuickCapture()
	case CockpitEmptyStateInboxOnly:
		return actionTriageInbox(), actionQuickCapture()
	case CockpitEmptyStateBacklogReady:
		return actionPlanToday(), actionQuickCapture()
	case CockpitEmptyStateNone:
	}

	switch action.Kind {
	case RecommendationKindWorkOnTask:
		return actionOpenTask(), actionCompleteTask()
	case RecommendationKindReviewBlocked:
		if cockpitActionCanUnblock(action) {
			return actionUnblockTask(), actionOpenTask()
		}
		return actionOpenTask(), actionDismissTask()
	case RecommendationKindTriageInbox:
		return actionTriageInbox(), actionQuickCapture()
	case RecommendationKindPlanToday:
		return actionPlanToday(), actionQuickCapture()
	case RecommendationKindAllDone:
		return actionPlanMore(), actionQuickCapture()
	case RecommendationKindNone:
	}
	return CockpitAction{}, CockpitAction{}
}

func cockpitFooterFor(emptyState CockpitEmptyState) CockpitFooterActions {
	footer := CockpitFooterActions{Escape: cockpitEscapeActions()}
	switch emptyState {
	case CockpitEmptyStateTodayCleared:
		footer.Contextual = []CockpitAction{actionPlanMore(), actionQuickCapture(), actionCloseView()}
	case CockpitEmptyStateFirstRun, CockpitEmptyStateInboxOnly, CockpitEmptyStateBacklogReady:
		footer.Contextual = []CockpitAction{actionQuickCapture(), actionSwitchTab(), actionCloseView()}
	default:
		footer.Contextual = []CockpitAction{actionCompleteTask(), actionPrioritizeTask(), actionQuickCapture()}
	}
	return footer
}

func cockpitActionCanUnblock(action RecommendedAction) bool {
	return action.Task != nil &&
		action.Project != nil &&
		action.Task.Status == models.StatusBlocked &&
		!action.Project.HasIncompleteDependencies(action.Task)
}

func cockpitEscapeActions() []CockpitAction {
	return []CockpitAction{
		actionOpenPipelineHub(),
		actionOpenCommandPalette(),
		actionShowHelp(),
	}
}

func actionOpenTask() CockpitAction {
	return CockpitAction{Command: CockpitCommandOpenTask, Key: "enter", Label: "open"}
}

func actionCompleteTask() CockpitAction {
	return CockpitAction{Command: CockpitCommandCompleteTask, Key: "x", Label: "done"}
}

func actionDismissTask() CockpitAction {
	return CockpitAction{Command: CockpitCommandDismissTask, Key: "x", Label: "dismiss"}
}

func actionPrioritizeTask() CockpitAction {
	return CockpitAction{Command: CockpitCommandPrioritizeTask, Key: "p", Label: "priority"}
}

func actionPlanToday() CockpitAction {
	return CockpitAction{Command: CockpitCommandPlanToday, Key: "p", Label: "plan"}
}

func actionPlanMore() CockpitAction {
	return CockpitAction{Command: CockpitCommandPlanMore, Key: "p", Label: "plan more"}
}

func actionTriageInbox() CockpitAction {
	return CockpitAction{Command: CockpitCommandTriageInbox, Key: "ctrl+i", Label: "triage"}
}

func actionUnblockTask() CockpitAction {
	return CockpitAction{Command: CockpitCommandUnblockTask, Key: "u", Label: "unblock"}
}

func actionQuickCapture() CockpitAction {
	return CockpitAction{Command: CockpitCommandQuickCapture, Key: "a", Label: "add"}
}

func actionSearchFilter() CockpitAction {
	return CockpitAction{Command: CockpitCommandSearchFilter, Key: "/", Label: "search"}
}

func actionNewProject() CockpitAction {
	return CockpitAction{Command: CockpitCommandNewProject, Key: "p", Label: "new project"}
}

func actionSwitchTab() CockpitAction {
	return CockpitAction{Command: CockpitCommandSwitchTab, Key: "tab", Label: "switch"}
}

func actionCloseView() CockpitAction {
	return CockpitAction{Command: CockpitCommandCloseView, Key: "Esc", Label: "close"}
}

func actionOpenPipelineHub() CockpitAction {
	return CockpitAction{Command: CockpitCommandOpenPipelineHub, Key: ";;", Label: "hub"}
}

func actionOpenCommandPalette() CockpitAction {
	return CockpitAction{Command: CockpitCommandOpenCommandPalette, Key: ":", Label: "cmd"}
}

func actionShowHelp() CockpitAction {
	return CockpitAction{Command: CockpitCommandShowHelp, Key: "?", Label: "help"}
}
