package chrome

// This file establishes the final seven-mode Outliner catalog.

// OutlinerModeID is the stable identifier for a catalog mode.
type OutlinerModeID string

// Stable canonical mode IDs. These strings are the durable identity of each
// mode; changing one is a breaking persisted-state change.
const (
	ModeAll     OutlinerModeID = "all"
	ModeInbox   OutlinerModeID = "inbox"
	ModeToday   OutlinerModeID = "today"
	ModeNext    OutlinerModeID = "next"
	ModeBacklog OutlinerModeID = "backlog"
	ModeDue     OutlinerModeID = "due"
	ModeRecent  OutlinerModeID = "recent"
)

// OutlinerOrderPolicy declares how a mode orders its rows. Only OrderPolicyTree
// honors the user's manual sort; every other policy owns a fixed order and marks
// manual sort inactive without destroying the stored tree-mode sort.
type OutlinerOrderPolicy uint8

const (
	OrderPolicyTree       OutlinerOrderPolicy = iota // hierarchical; manual sort allowed
	OrderPolicyTodayOrder                            // Today: durable planning order
	OrderPolicyNextOrder                             // Next: focus-queue order
	OrderPolicyDueDate                               // Due: calendar-date order
	OrderPolicyRecency                               // Recent: newest activity first
)

// OutlinerReorderPolicy declares whether J/K may mutate the active sequence.
// Hierarchical and planning modes support it where their concrete reorder
// implementation permits; Due never does.
type OutlinerReorderPolicy uint8

const (
	ReorderPolicyNone OutlinerReorderPolicy = iota
	ReorderPolicyContextual
)

// OutlinerRowStrategy names the canonical row builder a mode uses.
type OutlinerRowStrategy uint8

const (
	RowStrategyAll OutlinerRowStrategy = iota
	RowStrategyInbox
	RowStrategyToday
	RowStrategyNext
	RowStrategyBacklog
	RowStrategyDue
	RowStrategyRecent
)

// OutlinerMode is one catalog entry.
//
// ShowsPinned marks the modes that promote pinned work into a leading Pinned
// section. Only the modes a user reads to decide what to work on carry it;
// Inbox, Next, Backlog, Due, and Recent each answer one narrow question already,
// so a promoted group there only splits the answer the mode was opened for.
//
// No mode declares whether the cursor recovers after a mutation. Continuing the
// sweep from the position a departing task vacated is what every mode does, and
// the question a mode could once answer differently — whether to hand the cursor
// along — is not the question the recovery asks: it never carries the cursor to
// the answer the mutation produced, it keeps it where the user was reading.
type OutlinerMode struct {
	ID            OutlinerModeID
	Label         string
	OrderPolicy   OutlinerOrderPolicy
	ReorderPolicy OutlinerReorderPolicy
	RowStrategy   OutlinerRowStrategy
	ShowsPinned   bool
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

// OutlinerModes is the ordered catalog and the exact Tab/Shift+Tab cycle.
var OutlinerModes = []OutlinerMode{
	{ID: ModeAll, Label: "All", OrderPolicy: OrderPolicyTree, ReorderPolicy: ReorderPolicyContextual, RowStrategy: RowStrategyAll, ShowsPinned: true},
	{ID: ModeToday, Label: "Today", OrderPolicy: OrderPolicyTodayOrder, ReorderPolicy: ReorderPolicyContextual, RowStrategy: RowStrategyToday, ShowsPinned: true},
	{ID: ModeInbox, Label: "Inbox", OrderPolicy: OrderPolicyTree, ReorderPolicy: ReorderPolicyContextual, RowStrategy: RowStrategyInbox},
	{ID: ModeNext, Label: "Next", OrderPolicy: OrderPolicyNextOrder, ReorderPolicy: ReorderPolicyContextual, RowStrategy: RowStrategyNext},
	{ID: ModeBacklog, Label: "Backlog", OrderPolicy: OrderPolicyTree, ReorderPolicy: ReorderPolicyContextual, RowStrategy: RowStrategyBacklog},
	{ID: ModeDue, Label: "Due", OrderPolicy: OrderPolicyDueDate, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyDue},
	{ID: ModeRecent, Label: "Recent", OrderPolicy: OrderPolicyRecency, ReorderPolicy: ReorderPolicyNone, RowStrategy: RowStrategyRecent},
}

// outlinerModeIndex maps each mode ID to its catalog entry for O(1) lookup.
var outlinerModeIndex = func() map[OutlinerModeID]OutlinerMode {
	index := make(map[OutlinerModeID]OutlinerMode, len(OutlinerModes))
	for _, mode := range OutlinerModes {
		index[mode.ID] = mode
	}
	return index
}()

// OutlinerModeCatalog returns the ordered mode catalog.
func OutlinerModeCatalog() []OutlinerMode {
	out := make([]OutlinerMode, len(OutlinerModes))
	copy(out, OutlinerModes)
	return out
}

// OutlinerModeByID returns the mode for the given stable ID and true, or a zero
// mode and false when the ID is unknown.
func OutlinerModeByID(id OutlinerModeID) (OutlinerMode, bool) {
	mode, ok := outlinerModeIndex[id]
	return mode, ok
}

// OutlinerTabModes returns the seven descriptors in exact Tab order.
func OutlinerTabModes() []OutlinerMode {
	return OutlinerModeCatalog()
}
