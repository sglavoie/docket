// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"github.com/sglavoie/tt/internal/models"
)

type Surface uint8

const (
	SurfaceCapture Surface = 1 << iota
	SurfaceEdit
	SurfaceOutline
	SurfaceSearch
)

const allSurfaces = SurfaceCapture | SurfaceEdit | SurfaceOutline | SurfaceSearch

// Field identifies one family of inline metadata tokens.
type Field string

const (
	FieldPriority  Field = "priority"
	FieldTag       Field = "tag"
	FieldRemoveTag Field = "remove_tag"
	FieldContext   Field = "context"
	FieldDue       Field = "due"
	FieldDefer     Field = "defer"
	FieldTaskType  Field = "type"
	FieldParent    Field = "parent"
	FieldChild     Field = "child"
	// FieldDependency and FieldRemoveDependency reclaim the tilde that retired
	// recurrence used to own. A dependency is a soft, informational reference:
	// the grammar records it and nothing about the subject's readiness, ordering,
	// or eligibility to complete follows from it.
	FieldDependency       Field = "dependency"
	FieldRemoveDependency Field = "remove_dependency"
	FieldProject          Field = "project"
	FieldDestination      Field = "destination"
	FieldPinned           Field = "pinned"
	// FieldReview is the stored review flag. Review is not a lifecycle state and
	// never was one, so it takes a sigil of its own rather than a destination:
	// a task carries a state and, independently, whether it is up for review.
	FieldReview     Field = "review"
	FieldAttachment Field = "attachment"
	FieldComment    Field = "comment"
	// The remaining negations are search-only. `-` is the one negation operator
	// in the grammar: on a mutation surface it takes a value away from the task,
	// and on search it takes the rows carrying that value away from the result
	// set. FieldRemoveTag and FieldRemoveDependency already carry both readings;
	// these eight carry only the second, because there is nothing to subtract
	// from a task that does not hold a set.
	FieldRemovePriority    Field = "remove_priority"
	FieldRemoveContext     Field = "remove_context"
	FieldRemoveDue         Field = "remove_due"
	FieldRemoveDefer       Field = "remove_defer"
	FieldRemoveDestination Field = "remove_destination"
	FieldRemovePinned      Field = "remove_pinned"
	FieldRemoveReview      Field = "remove_review"
	FieldRemoveProject     Field = "remove_project"
	// FieldDerived is the computed-predicate family. Its whole meaning is
	// "derived, never assigned", which is why it can only ever reach search: no
	// capture or edit surface may set a value the workspace computes.
	FieldDerived       Field = "derived"
	FieldRemoveDerived Field = "remove_derived"
)

// Cardinality describes whether a field may occur once or many times in one
// task line.
type Cardinality uint8

const (
	CardinalitySingleton Cardinality = iota
	CardinalityRepeatable
)

// ClearOperation describes an explicit clear value available on selected
// surfaces. Omitted fields always preserve their current value.
type ClearOperation struct {
	Value       string
	Description string
	Surfaces    Surface
}

// Definition is the public projection of one canonical registry entry. Help,
// completion, previews, validation, and parsing all consume the same entries.
type Definition struct {
	Field         Field
	Group         string
	Trigger       string
	Forms         []string
	Description   string
	Values        []string
	RetiredValues []string
	Examples      []string
	Legacy        bool
	Surfaces      Surface
	Cardinality   Cardinality
	Clears        []ClearOperation
}

type surfaceForm struct {
	Text     string
	Surfaces Surface
}

type tokenValidation uint8

const (
	validateAny tokenValidation = iota
	validatePriority
	validateTag
	validateClosedSet
	validateAttachment
	validateDerived
)

// definitionSpec adds the search wording beside the mutation wording because
// the same token is a command on capture/edit/outline and a predicate on search.
type definitionSpec struct {
	Definition
	forms   []surfaceForm
	pattern string
	// normalize reduces the matched group to the value consumers receive, for a
	// field whose pattern has to admit more shapes than it wants to hand on. Only
	// the attachment field declares one; a nil normalizer is the identity.
	normalize  func(string) string
	validation tokenValidation
	// negates names the positive field this entry subtracts. A negation owns no
	// vocabulary of its own: its values, its viability test, and its diagnostics
	// all resolve through the twin, which is what keeps "-@friday" and "@friday"
	// from ever disagreeing about what a Friday is. validateNegationPlacement
	// additionally holds the pair adjacent in parse order.
	negates           Field
	searchDescription string
	searchClearText   map[string]string
}

var priorityNames = []string{"1", "2", "3"}
var retiredPriorityNames = []string{"4", "5"}

var dueValueNames = []string{
	"today", "tomorrow", "+1d", "+3d", "+1w", "+2w", "+1m",
	"monday", "friday", "next monday", "next friday", "in 3 days", "in 1 week", "in 2 weeks",
}

var deferValueNames = []string{
	"+2h", "+8h", "tomorrow", "+1d", "+3d", "+1w", "monday", "friday",
}

const clearValue = "0"

// derivedNames is the closed set ?value accepts, spelled out of the canonical
// property list rather than copied: every one of them is a question about a task
// that the task itself does not store, so the answers are invariants and live in
// internal/models. The grammar owns only the spelling. They are search-only for
// the same reason — there is no assignment that could make a task a leaf.
var derivedNames = derivedPropertyNames()

func derivedPropertyNames() []string {
	names := make([]string, 0, len(models.DerivedProperties))
	for _, property := range models.DerivedProperties {
		names = append(names, string(property))
	}
	return names
}

// derivedAgeSuffix marks the day count only ?completed carries. The qualifier is
// optional: bare ?completed asks whether a completion instant exists at all.
const derivedAgeSuffix = "d"
