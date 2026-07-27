package tui

// This file establishes the final ten-mode Outliner catalog.

// OutlinerModeID is the stable identifier for a catalog mode.
type OutlinerModeID string

// Stable canonical mode IDs. These strings are the durable identity of each
// mode; changing one is a breaking persisted-state change.
const (
	ModeAll         OutlinerModeID = "all"
	ModeInbox       OutlinerModeID = "inbox"
	ModeNext        OutlinerModeID = "next"
	ModeBacklog     OutlinerModeID = "backlog"
	ModeDue         OutlinerModeID = "due"
	ModePriority    OutlinerModeID = "priority"
	ModeBlocked     OutlinerModeID = "blocked"
	ModeParked      OutlinerModeID = "parked"
	ModeDailyReview OutlinerModeID = "daily-review"
	ModeActivity    OutlinerModeID = "activity"
)

// OutlinerModeCategory groups modes for the categorized mode picker.
type OutlinerModeCategory uint8

const (
	ModeCategoryWork OutlinerModeCategory = iota
	ModeCategoryComputed
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
	OrderPolicyReview                                // Daily Review/Activity: timeline order
	OrderPolicyComputed                              // Due/Priority/Blocked/Parked: computed flat order
)

// OutlinerReorderPolicy declares whether J/K may mutate the active sequence.
// The four primary modes support it only in a single-project scope; computed
// and review modes never do.
type OutlinerReorderPolicy uint8

const (
	ReorderPolicyNone OutlinerReorderPolicy = iota
	ReorderPolicySingleProjectSequence
)

// OutlinerRowStrategy names the canonical row builder a mode uses.
type OutlinerRowStrategy uint8

const (
	RowStrategyAll OutlinerRowStrategy = iota
	RowStrategyInbox
	RowStrategyNext
	RowStrategyBacklog
	RowStrategyDue
	RowStrategyPriority
	RowStrategyBlocked
	RowStrategyParked
	RowStrategyReview
)

// OutlinerMode is one catalog entry.
type OutlinerMode struct {
	ID            OutlinerModeID
	Label         string
	Aliases       []string
	Category      OutlinerModeCategory
	OrderPolicy   OutlinerOrderPolicy
	ReorderPolicy OutlinerReorderPolicy
	RowStrategy   OutlinerRowStrategy
	// TabIndex is the stable Tab/Shift+Tab position, or noTabIndex for modes
	// outside the four-mode work cycle.
	TabIndex int
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

// SupportsManualReorder reports whether J/K may reorder this mode when the
// active scope resolves to exactly one project.
func (mode OutlinerMode) SupportsManualReorder() bool {
	return mode.ReorderPolicy == ReorderPolicySingleProjectSequence
}

// InTabCycle reports whether the mode participates in Tab/Shift+Tab.
func (mode OutlinerMode) InTabCycle() bool { return mode.TabIndex >= 0 }

// SupportsCommonFilters reports whether the shared common-filter + search stack
// composes on top of this mode's rows. Every current mode composes them (the
// Planner applied common filters and search to Today/Next/Backlog too), so this
// is universally true; the method exists so future modes can opt out without a
// second convention.
func (mode OutlinerMode) SupportsCommonFilters() bool { return true }

// outlinerModes is the ordered catalog. Order is the canonical presentation
// order within each category for the picker.
const noTabIndex = -1

var outlinerModes = []OutlinerMode{
	// Primary modes, in exact Tab order.
	{ID: ModeAll, Label: "All", Aliases: []string{"all", "all tasks", "cross-project tasks", "every task"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyTree, ReorderPolicy: ReorderPolicySingleProjectSequence, RowStrategy: RowStrategyAll, TabIndex: 0},
	{ID: ModeInbox, Label: "Inbox", Aliases: []string{"inbox"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyTree, ReorderPolicy: ReorderPolicySingleProjectSequence, RowStrategy: RowStrategyInbox, TabIndex: 1},
	{ID: ModeNext, Label: "Next", Aliases: []string{"next", "n"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyFocusQueue, ReorderPolicy: ReorderPolicySingleProjectSequence, RowStrategy: RowStrategyNext, TabIndex: 2},
	{ID: ModeBacklog, Label: "Backlog", Aliases: []string{"backlog", "b"}, Category: ModeCategoryWork, OrderPolicy: OrderPolicyRanked, ReorderPolicy: ReorderPolicySingleProjectSequence, RowStrategy: RowStrategyBacklog, TabIndex: 3},

	// Computed modes.
	{ID: ModeDue, Label: "Due", Aliases: []string{"due", "due date", "schedule", "due dates", "by due date"}, Category: ModeCategoryComputed, OrderPolicy: OrderPolicyComputed, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyDue, TabIndex: noTabIndex},
	{ID: ModePriority, Label: "Priority", Aliases: []string{"priority", "high priority"}, Category: ModeCategoryComputed, OrderPolicy: OrderPolicyComputed, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyPriority, TabIndex: noTabIndex},
	{ID: ModeBlocked, Label: "Blocked", Aliases: []string{"blocked"}, Category: ModeCategoryComputed, OrderPolicy: OrderPolicyComputed, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyBlocked, TabIndex: noTabIndex},
	{ID: ModeParked, Label: "Parked", Aliases: []string{"parked"}, Category: ModeCategoryComputed, OrderPolicy: OrderPolicyComputed, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyParked, TabIndex: noTabIndex},

	// Review modes.
	{ID: ModeDailyReview, Label: "Daily Review", Aliases: []string{"daily-review", "DailyReview", "daily", "planning", "morning review", "review daily"}, Category: ModeCategoryReview, OrderPolicy: OrderPolicyReview, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyReview, TabIndex: noTabIndex},
	{ID: ModeActivity, Label: "Activity", Aliases: []string{"activity", "RecentActivity", "latest activity", "recent activity", "review activity", "latest"}, Category: ModeCategoryReview, OrderPolicy: OrderPolicyReview, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyReview, TabIndex: noTabIndex},
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

// OutlinerTabModes returns the four primary descriptors in exact Tab order.
func OutlinerTabModes() []OutlinerMode {
	out := make([]OutlinerMode, 0, 4)
	for tabIndex := 0; tabIndex < 4; tabIndex++ {
		for _, mode := range outlinerModes {
			if mode.TabIndex == tabIndex {
				out = append(out, mode)
				break
			}
		}
	}
	return out
}
