package tui

// This file establishes the unified Outliner mode catalog: a single, stable
// vocabulary that wraps the existing perspectives, smart focuses, and
// priority/due-date lenses without changing their persisted meanings. It adds no
// navigation or rendering cutover; it only defines the mode metadata and the
// purpose-built ordering policies that the Next/Backlog row strategies in
// outliner_mode_rows.go realize.

// OutlinerModeID is the stable identifier for a catalog mode. IDs are strings so
// they can appear in the mode picker, aliases, and future persisted state
// without colliding with the integer perspective IDs they wrap.
type OutlinerModeID string

// Stable mode IDs. These strings are the durable identity of each mode; changing
// one is a breaking change to the picker and any future persisted selection.
const (
	ModeNext     OutlinerModeID = "next"
	ModeBacklog  OutlinerModeID = "backlog"
	ModeActive   OutlinerModeID = "active"
	ModeAll      OutlinerModeID = "all"
	ModeInbox    OutlinerModeID = "inbox"
	ModeStandby  OutlinerModeID = "standby"
	ModeWaiting  OutlinerModeID = "waiting"
	ModeSomeday  OutlinerModeID = "someday"
	ModeReview   OutlinerModeID = "review"
	ModeSnoozed  OutlinerModeID = "snoozed"
	ModeComplete OutlinerModeID = "completed"

	ModeDueSoon      OutlinerModeID = "due-soon"
	ModeHighPriority OutlinerModeID = "high-priority"
	ModeBlocked      OutlinerModeID = "blocked"
	ModeParked       OutlinerModeID = "parked"
	ModePinned       OutlinerModeID = "pinned"
	ModeRecent       OutlinerModeID = "recently-visited"

	ModePriorityLens OutlinerModeID = "priority"
	ModeDueDateLens  OutlinerModeID = "due-date"

	ModeDailyReview OutlinerModeID = "daily-review"
	ModeActivity    OutlinerModeID = "activity"
)

// OutlinerModeCategory groups modes for the categorized mode picker.
type OutlinerModeCategory uint8

const (
	ModeCategoryWork OutlinerModeCategory = iota
	ModeCategoryWorkflow
	ModeCategorySmart
	ModeCategoryLens
	ModeCategoryReview
)

// OutlinerOrderPolicy declares how a mode orders its rows. Only OrderPolicyTree
// honors the user's manual sort; every other policy owns a fixed order and marks
// manual sort inactive without destroying the stored tree-mode sort.
type OutlinerOrderPolicy uint8

const (
	OrderPolicyTree       OutlinerOrderPolicy = iota // hierarchical; manual sort allowed
	OrderPolicyFocusQueue                            // Next: focus-queue order
	OrderPolicyRanked                                // Backlog: cross-project ranker order
	OrderPolicyLens                                  // Priority/Due Date: computed flat order
	OrderPolicyReview                                // Daily Review/Activity: timeline order
)

// OutlinerRowStrategy names the row builder a mode uses. Perspective and
// SmartFocus reuse the shared tree renderer with a predicate; Next/Backlog
// are the purpose-built strategies in outliner_mode_rows.go.
type OutlinerRowStrategy uint8

const (
	RowStrategyPerspective OutlinerRowStrategy = iota
	RowStrategySmartFocus
	RowStrategyNext
	RowStrategyBacklog
	RowStrategyLens
	RowStrategyReview
)

// noPerspective marks a mode that is not backed by a persisted perspective.
const noPerspective = -1

// OutlinerMode is one catalog entry. It carries stable identity plus the linkage
// into the existing subsystems it wraps (perspective ID, smart focus, lens) so
// saved-filter and perspective compatibility is preserved rather than replaced.
type OutlinerMode struct {
	ID          OutlinerModeID
	Label       string
	Aliases     []string
	Category    OutlinerModeCategory
	OrderPolicy OutlinerOrderPolicy
	RowStrategy OutlinerRowStrategy

	// Perspective is the stable perspective ID this mode wraps, or noPerspective
	// when the mode is a computed horizon (Backlog) or a non-perspective
	// surface (smart focuses, lenses, review).
	Perspective int
	// SmartFocus is set for smart modes so the existing matchesSmartFocus
	// predicate remains the single implementation.
	SmartFocus *OutlinerFocus
	// Lens is set for the Due Date lens mode; PriorityLens marks the Priority
	// lens (the session-only cross-project priority mode).
	Lens         *OutlinerLens
	PriorityLens bool
}

// SupportsManualSort reports whether the user's manual sort applies. Only
// tree-order modes honor it; fixed-order modes own their ordering.
func (mode OutlinerMode) SupportsManualSort() bool {
	return mode.OrderPolicy == OrderPolicyTree
}

// FixedOrder reports whether the mode owns a fixed order that manual sort must
// not override.
func (mode OutlinerMode) FixedOrder() bool {
	return mode.OrderPolicy != OrderPolicyTree
}

// SupportsCommonFilters reports whether the shared common-filter + search stack
// composes on top of this mode's rows. Every current mode composes them (the
// Planner applied common filters and search to Today/Next/Backlog too), so this
// is universally true; the method exists so future modes can opt out without a
// second convention.
func (mode OutlinerMode) SupportsCommonFilters() bool { return true }

func smartFocusPtr(f OutlinerFocus) *OutlinerFocus { return &f }
func lensPtr(l OutlinerLens) *OutlinerLens         { return &l }

// outlinerModes is the ordered catalog. Order is the canonical presentation
// order within each category for the picker.
var outlinerModes = []OutlinerMode{
	// Work.
	{ID: ModeNext, Label: "Next", Aliases: []string{"next", "n"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyFocusQueue, RowStrategy: RowStrategyNext, Perspective: PerspectiveNext},
	{ID: ModeBacklog, Label: "Backlog", Aliases: []string{"backlog", "b"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyRanked, RowStrategy: RowStrategyBacklog, Perspective: noPerspective},
	{ID: ModeActive, Label: "Active", Aliases: []string{"active"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveActive},
	{ID: ModeAll, Label: "All", Aliases: []string{"all", "all tasks", "cross-project tasks", "every task"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveAll},

	// Workflow.
	{ID: ModeInbox, Label: "Inbox", Aliases: []string{"inbox"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveInbox},
	{ID: ModeStandby, Label: "Standby", Aliases: []string{"standby"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveStandby},
	{ID: ModeWaiting, Label: "Waiting", Aliases: []string{"waiting"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveWaiting},
	{ID: ModeSomeday, Label: "Someday", Aliases: []string{"someday"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveSomeday},
	{ID: ModeReview, Label: "Review", Aliases: []string{"review"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveReview},
	{ID: ModeSnoozed, Label: "Snoozed", Aliases: []string{"snoozed"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveSnoozed},
	{ID: ModeComplete, Label: "Completed", Aliases: []string{"completed", "done", "completed tasks"}, Category: ModeCategoryWorkflow, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategyPerspective, Perspective: PerspectiveCompleted},

	// Smart.
	{ID: ModeDueSoon, Label: "Due Soon", Aliases: []string{"due-soon", "due"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerDueSoon)},
	{ID: ModeHighPriority, Label: "High Priority", Aliases: []string{"high-priority", "high", "priority view"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerHighPriority)},
	{ID: ModeBlocked, Label: "Blocked", Aliases: []string{"blocked"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerBlocked)},
	{ID: ModeParked, Label: "Parked", Aliases: []string{"parked"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerParked)},
	{ID: ModePinned, Label: "Pinned", Aliases: []string{"pinned"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerPinned)},
	{ID: ModeRecent, Label: "Recently Visited", Aliases: []string{"recently-visited", "recent", "recent tasks"}, Category: ModeCategorySmart, OrderPolicy: OrderPolicyTree, RowStrategy: RowStrategySmartFocus, Perspective: noPerspective, SmartFocus: smartFocusPtr(OutlinerRecent)},

	// Lenses.
	{ID: ModePriorityLens, Label: "Priority", Aliases: []string{"priority"}, Category: ModeCategoryLens, OrderPolicy: OrderPolicyLens, RowStrategy: RowStrategyLens, Perspective: noPerspective, PriorityLens: true},
	{ID: ModeDueDateLens, Label: "Due Date", Aliases: []string{"due-date", "schedule", "all tasks by due date", "due dates", "by due date", "tasks by due date", "due date overview"}, Category: ModeCategoryLens, OrderPolicy: OrderPolicyLens, RowStrategy: RowStrategyLens, Perspective: noPerspective, Lens: lensPtr(OutlinerLensDueDate)},

	// Review.
	{ID: ModeDailyReview, Label: "Daily Review", Aliases: []string{"daily-review", "daily", "planning", "morning review", "DailyReview", "review daily"}, Category: ModeCategoryReview, OrderPolicy: OrderPolicyReview, RowStrategy: RowStrategyReview, Perspective: noPerspective},
	{ID: ModeActivity, Label: "Activity", Aliases: []string{"activity", "latest activity", "recent activity", "review activity", "RecentActivity", "latest"}, Category: ModeCategoryReview, OrderPolicy: OrderPolicyReview, RowStrategy: RowStrategyReview, Perspective: noPerspective},
}

// outlinerModeIndex maps each mode ID to its catalog entry for O(1) lookup.
var outlinerModeIndex = func() map[OutlinerModeID]OutlinerMode {
	index := make(map[OutlinerModeID]OutlinerMode, len(outlinerModes))
	for _, mode := range outlinerModes {
		index[mode.ID] = mode
	}
	return index
}()

// OutlinerModeCatalog returns the ordered mode catalog.
func OutlinerModeCatalog() []OutlinerMode {
	out := make([]OutlinerMode, len(outlinerModes))
	copy(out, outlinerModes)
	return out
}

// OutlinerModeByID returns the mode for the given stable ID and true, or a zero
// mode and false when the ID is unknown.
func OutlinerModeByID(id OutlinerModeID) (OutlinerMode, bool) {
	mode, ok := outlinerModeIndex[id]
	return mode, ok
}

// OutlinerModesByCategory returns the catalog modes in the given category, in
// catalog order.
func OutlinerModesByCategory(category OutlinerModeCategory) []OutlinerMode {
	var out []OutlinerMode
	for _, mode := range outlinerModes {
		if mode.Category == category {
			out = append(out, mode)
		}
	}
	return out
}

// OutlinerModeForPerspective returns the mode that wraps the given stable
// perspective ID and true, or a zero mode and false. Every visible perspective
// maps to exactly one mode, preserving persisted perspective meanings.
func OutlinerModeForPerspective(perspective int) (OutlinerMode, bool) {
	for _, mode := range outlinerModes {
		if mode.Perspective == perspective {
			return mode, true
		}
	}
	return OutlinerMode{}, false
}

// OutlinerModeForSmartFocus returns the smart mode wrapping the given focus and
// true, or a zero mode and false.
func OutlinerModeForSmartFocus(focus OutlinerFocus) (OutlinerMode, bool) {
	for _, mode := range outlinerModes {
		if mode.SmartFocus != nil && *mode.SmartFocus == focus {
			return mode, true
		}
	}
	return OutlinerMode{}, false
}
