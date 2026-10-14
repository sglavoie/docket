package chrome

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Plan and Status are the two halves of one presentation vocabulary. A task's
// plan is the collection it was committed into — Today or Next — and its status
// is the lifecycle state it holds, so the eight states are statuses, the two
// collections are plans, and nothing is both. The words live here because the
// workflow chord, the capture route, and Peek all spell them, and a surface
// minting its own synonym is what made one undivided run read as ten states.
const (
	PlanFieldLabel   = "Plan"
	StatusFieldLabel = "Status"
)

// PlanHintGroup and StatusHintGroup are those same two words as a heading over
// the entries they open. The colon is what makes a heading read as one beside
// `key label` entries that carry none.
const (
	PlanHintGroup   = PlanFieldLabel + ":"
	StatusHintGroup = StatusFieldLabel + ":"
)

// The filter chord divides its facets by where the answer comes from: a field is
// a value the task stores and a facet narrows by, and a derived facet is a fact
// worked out from the task and its neighbors that no task stores. Clear all and
// the project facet stand outside both, since neither narrows by a task value.
const (
	FilterFieldHintGroup   = "Field:"
	FilterDerivedHintGroup = "Derived:"
)

// The go-to chord divides by what the jump is aimed at: a mode of the outline,
// a place the cursor's own task names, or the workspace's sync state.
// Cursor-top stands ahead of all three, since it moves within the view it is
// already in.
const (
	GotoModeHintGroup = "Mode:"
	GotoTaskHintGroup = "Task:"
	GotoSyncHintGroup = "Sync:"
)

// PlanStatusValue spells a canonical lowercase token as this vocabulary shows
// it. The grammar stores `today` and `backlog`; a labeled field reads them
// capitalized, and capitalizing once here is what keeps the surfaces that do it
// from disagreeing about how.
func PlanStatusValue(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	first, size := utf8.DecodeRuneInString(token)
	return string(unicode.ToUpper(first)) + token[size:]
}
