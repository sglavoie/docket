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

// PlanFieldKey and StatusFieldKey are the same two words in the lowercase
// `label value` idiom Peek's three summary lines are written in. A surface picks
// the spelling its neighbors already use and never a third word.
const (
	PlanFieldKey   = "plan"
	StatusFieldKey = "status"
)

// PlanHintGroup and StatusHintGroup are those same two words as a heading over
// the entries they open. The colon is what makes a heading read as one beside
// `key label` entries that carry none.
const (
	PlanHintGroup   = PlanFieldLabel + ":"
	StatusHintGroup = StatusFieldLabel + ":"
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
