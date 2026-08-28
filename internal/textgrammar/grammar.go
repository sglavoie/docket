// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Surface identifies a consumer of the shared inline grammar.
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
	FieldAttachment       Field = "attachment"
	FieldComment          Field = "comment"
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
	validatePin
	validateAttachment
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
	normalize         func(string) string
	validation        tokenValidation
	searchDescription string
	searchClearText   map[string]string
}

var priorityNames = []string{"1", "2", "3"}
var retiredPriorityNames = []string{"4", "5"}

const clearValue = "0"

var registry = []definitionSpec{
	// Comment is parsed FIRST, ahead of every other field including attachment,
	// and that position is the whole reason a comment can hold sigil characters.
	// RE2 has no lookaround, so no other pattern can say "not inside a ;…; span";
	// peeling the comment before anything else is what keeps
	// "Fix login ;check the #auth flow;" from losing #auth to the tag rule. The
	// closing delimiter is required, so an unterminated ";note" stays prose, and
	// the leading-whitespace boundary keeps "Fix a; then b" literal. The teaching
	// surfaces read guideOrder instead, so this position never leads the guide.
	{
		Definition: Definition{
			Field: FieldComment, Group: "Comments", Trigger: ";",
			Description: "Add a comment to the new task (repeatable)",
			Examples:    []string{";ask about the SSO deadline;", ";blocked on design;"},
			Surfaces:    SurfaceCapture, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: ";comment;", Surfaces: SurfaceCapture}},
		pattern:    `(?:^|\s);(\S[^;]*);`,
		validation: validateAny,
	},
	// Attachment is parsed BEFORE tag, and that position is load-bearing rather
	// than cosmetic. RE2 has no lookaround, so the tag pattern cannot say "not
	// followed by &": #([\w][\w.:-]*) stops at the &, and the rewrite that removes
	// the tag leaves the & whitespace-preceded, which would turn
	// "Task #work&http://x/y" into a spurious attachment. Peeling & first leaves
	// that & glued to the tag, where it stays literal title text. The teaching
	// surfaces read guideOrder instead, so this position never leads the guide.
	//
	// A real path holds spaces — "~/Library/Application Support/…" is where a
	// screenshot tool writes — and a run of non-space characters stops at the
	// first one, which attached half a path and left the rest in the title. The
	// value is therefore a single-quoted span, then a double-quoted one, then a
	// run whose spaces are backslash-escaped, tried in that order. Those
	// alternatives only widen what one token may span; what a token means is
	// settled by util.NormalizePastedPathInput, the same normalizer the
	// Attachments panel runs on a paste, so a quoted path and a typed one
	// classify identically. An unterminated quote matches no quoted span, falls
	// through to the last alternative, and stays the prefix it already was.
	{
		Definition: Definition{
			Field: FieldAttachment, Group: "Attachments", Trigger: "&",
			// The description stays the guide's longest and cannot grow: it sets the
			// aligned description column every entry is padded to, and one word more
			// drops the whole syntax guide into its stacked fallback. The quoted
			// example is what teaches the spaces rule.
			Description: "Attach a URL, file path, or code reference (repeatable)",
			Examples: []string{
				"&https://example.com/spec.pdf",
				"&internal/app/add.go:19",
				"&'~/notes/Q3 plan.pdf'",
			},
			Surfaces: SurfaceCapture, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: "&link", Surfaces: SurfaceCapture}},
		pattern:    `(?:^|\s)&('[^']*'|"(?:\\.|[^"\\])*"|(?:\\ |\S)+)`,
		normalize:  util.NormalizePastedPathInput,
		validation: validateAttachment,
	},
	// Both dependency sigils are parsed BEFORE #tag, for the reason attachment is:
	// RE2 has no lookaround, so #([\w][\w.:-]*) stops at the tilde in
	// "Task #work~foo" and the rewrite that removes the tag would leave the ~
	// whitespace-preceded, minting a dependency out of ordinary prose. Peeling the
	// tilde first leaves it glued to the tag, where it stays literal title text.
	// Removal is parsed before addition so the two-rune trigger is peeled whole;
	// the leading-whitespace boundary already keeps ~ out of "-~12", but the order
	// is what makes that independent of the boundary rule.
	{
		Definition: Definition{
			Field: FieldRemoveDependency, Group: "Dependencies", Trigger: "-~",
			Description: "Remove a dependency (repeatable)",
			Examples:    []string{"-~1", "-~project-slug:1"},
			// Capture carries the field only so it can be refused by name. It
			// declares no capture form, so the guide never advertises it there.
			// Search carries it nowhere at all: a dependency predicate is direct and
			// positive, and ParseSearch refuses -~ by name in placementDiagnostics.
			Surfaces: SurfaceCapture | SurfaceEdit | SurfaceOutline, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: "-~task", Surfaces: SurfaceEdit | SurfaceOutline}},
		pattern:    `(?:^|\s)-~(\S+)`,
		validation: validateAny,
	},
	{
		Definition: Definition{
			Field: FieldDependency, Group: "Dependencies", Trigger: "~",
			Description: "Depend on another task (repeatable)",
			Examples:    []string{"~1", "~project-slug:1"},
			Surfaces:    allSurfaces, Cardinality: CardinalityRepeatable,
			// Clearing belongs to the two surfaces that hold a stored set. Search has
			// nothing to clear, so ~0 is guidance there rather than a form.
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear dependencies", Surfaces: SurfaceEdit | SurfaceOutline}},
		},
		forms: []surfaceForm{
			{Text: "~task", Surfaces: allSurfaces},
			{Text: "~0", Surfaces: SurfaceEdit | SurfaceOutline},
		},
		pattern:           `(?:^|\s)~(\S+)`,
		validation:        validateAny,
		searchDescription: "Match a direct dependency",
	},
	{
		Definition: Definition{
			Field: FieldPriority, Group: "Priority & Tags", Trigger: "!",
			Description: "Set priority", Values: priorityNames, RetiredValues: retiredPriorityNames,
			Examples: []string{"!1", "!2", "!3"}, Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
			Clears:      []ClearOperation{{Value: clearValue, Description: "Clear priority", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "!1-3", Surfaces: allSurfaces},
			{Text: "!0", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)!([^\s]+)`, validation: validatePriority,
		searchDescription: "Match priority exactly",
	},
	{
		Definition: Definition{
			Field: FieldTag, Group: "Priority & Tags", Trigger: "#",
			Description: "Add tag", Examples: []string{"#backend", "#bug", "#source:github"},
			Surfaces: allSurfaces, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "#tag", Surfaces: allSurfaces}},
		pattern:           `(?:^|\s)#([\w][\w.:-]*)`,
		validation:        validateTag,
		searchDescription: "Match a tag prefix",
	},
	{
		Definition: Definition{
			Field: FieldRemoveTag, Group: "Priority & Tags", Trigger: "-#",
			Description: "Remove tag", Examples: []string{"-#old", "-#source:github"},
			Surfaces: SurfaceEdit | SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-#tag", Surfaces: SurfaceEdit | SurfaceSearch}},
		pattern:           `(?:^|\s)-#([\w][\w.:-]*)`,
		validation:        validateTag,
		searchDescription: "Exclude a tag prefix",
	},
	{
		Definition: Definition{
			Field: FieldContext, Group: "Context", Trigger: "##",
			Description: "Set context", Examples: []string{"##frontend", "##client-acme"},
			Surfaces: allSurfaces, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear context", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "##context", Surfaces: allSurfaces},
			{Text: "##0", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)##([\w][\w.:-]*)`, validation: validateTag,
		searchDescription: "Match a context prefix",
	},
	{
		Definition: Definition{
			Field: FieldDue, Group: "Schedule", Trigger: "@",
			Description: "Set due date",
			Values: []string{
				"today", "tomorrow", "+1d", "+3d", "+1w", "+2w", "+1m",
				"monday", "friday", "next monday", "next friday", "in 3 days", "in 1 week", "in 2 weeks",
			},
			Examples: []string{"@tomorrow", "@friday", "@+3d", "@next monday", "@tomorrow 15:30", "@2026-08-25 09:00"},
			Surfaces: allSurfaces, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear due date", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "@date", Surfaces: allSurfaces},
			{Text: "@0", Surfaces: SurfaceEdit},
		},
		// The date token refuses a leading @ (so @@ can never be consumed here,
		// whatever order the registry is walked in — no legal due value starts
		// with @) and may carry an optional trailing hh:mm clock time, which is
		// the only way to set a due time. The hour-offset form (+Nh) is still
		// refused downstream: hours are defer-only.
		pattern:           `(?i)(?:^|\s)@((?:next\s+\w+|in\s+\d+\s+\w+|[^\s@]\S*)(?:\s+\d{1,2}:\d{2})?)`,
		searchDescription: "Match a due bucket, exact due day, day and hh:mm time, or +Nh window",
	},
	{
		Definition: Definition{
			Field: FieldDefer, Group: "Schedule", Trigger: "@@",
			Description: "Defer task until a later instant",
			Values: []string{
				"+2h", "+8h", "tomorrow", "+1d", "+3d", "+1w", "monday", "friday",
			},
			Examples:    []string{"@@+2h", "@@tomorrow", "@@+3d"},
			Surfaces:    SurfaceCapture | SurfaceEdit | SurfaceSearch,
			Cardinality: CardinalitySingleton,
			Clears:      []ClearOperation{{Value: clearValue, Description: "Clear deferral", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "@@defer", Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch},
			{Text: "@@0", Surfaces: SurfaceEdit},
		},
		pattern:           `(?i)(?:^|\s)@@(next\s+\w+|in\s+\d+\s+\w+|\S+)`,
		searchDescription: "Match tasks deferred until that day",
	},
	{
		Definition: Definition{
			Field: FieldDestination, Group: "Destination", Trigger: "$",
			Description: "Set lifecycle or planning destination", Values: destinationNames,
			Examples: prefixed(destinationNames, "$"), Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
		},
		forms:             []surfaceForm{{Text: "$destination", Surfaces: allSurfaces}},
		pattern:           `(?:^|\s)\$(\w+)`,
		searchDescription: "Match a lifecycle state or planning list",
	},
	// Pin advertises only the set value. Definitions does not filter Values by
	// surface, so listing the clear value there would offer it while capturing,
	// exactly as priority keeps its 0 out of Values. An unrecognized value is not
	// consumed, because * occurs in ordinary prose and in code identifiers.
	{
		Definition: Definition{
			Field: FieldPinned, Group: "Pinning", Trigger: "*",
			Description: "Pin task", Values: []string{"pin"},
			Examples: []string{"*pin"}, Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{
				{Value: "unpin", Description: "Unpin task", Surfaces: SurfaceEdit | SurfaceSearch},
			},
		},
		forms: []surfaceForm{
			{Text: "*pin", Surfaces: allSurfaces},
			{Text: "*unpin", Surfaces: SurfaceEdit | SurfaceSearch},
		},
		pattern: `(?:^|\s)\*(\S+)`, validation: validatePin,
		searchDescription: "Match pinned tasks",
		searchClearText:   map[string]string{"unpin": "Match unpinned tasks"},
	},
	{
		Definition: Definition{
			Field: FieldProject, Group: "Project Routing", Trigger: "%",
			Description: "Move to project", Examples: []string{"%project-slug"},
			Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch, Cardinality: CardinalitySingleton,
		},
		forms:             []surfaceForm{{Text: "%project", Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch}},
		pattern:           `(?:^|\s)%(\S+)`,
		searchDescription: "Narrow to matching scope projects",
	},
	{
		Definition: Definition{
			Field: FieldParent, Group: "Placement", Trigger: ">",
			Description: "Set parent task", Examples: []string{">1", ">Login"},
			Surfaces: SurfaceCapture | SurfaceEdit, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Move to project root", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: ">parent", Surfaces: SurfaceCapture | SurfaceEdit},
			{Text: ">0", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)>(\S+)`,
	},
	// Child declares no clear operation, which is what makes <0 an ordinary
	// reference that never resolves rather than a clear. Its pattern requires the
	// same leading whitespace boundary as parent's, so embedded comparison text
	// such as a<b stays literal. The description carries the repeatability hint
	// because child sits beside singleton parent in one group with a near-identical
	// form, so the two would otherwise read as the same cardinality.
	{
		Definition: Definition{
			Field: FieldChild, Group: "Placement", Trigger: "<",
			Description: "Adopt existing task as child (repeatable)", Examples: []string{"<1", "<Login"},
			Surfaces: SurfaceCapture | SurfaceEdit, Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "<child", Surfaces: SurfaceCapture | SurfaceEdit}},
		pattern: `(?:^|\s)<(\S+)`,
	},
	// Retired sigils remain recognized only so capture can provide actionable
	// migration errors. Legacy entries are not advertised by Definitions.
	{
		Definition: Definition{
			Field: FieldTaskType, Group: "Legacy", Trigger: "^", Description: "Retired task type",
			Examples: []string{"^bug"}, Legacy: true, Surfaces: SurfaceCapture,
			Cardinality: CardinalitySingleton,
		},
		forms:   []surfaceForm{{Text: "^type", Surfaces: SurfaceCapture}},
		pattern: `(?:^|\s)\^(\S+)`,
	},
}

// guideOrder is teach order, which is a different concern from the parse order
// `registry` declares. Parse order answers "which pattern peels first", and one
// field can only be disambiguated by sitting early; teach order answers "what
// does a user meet first", where the same field belongs beside its neighbors.
// The two held one list until the attachment sigil needed to be parsed ahead of
// #tag without leading the syntax guide. validateGuideOrder keeps them the same
// set, so a field added to one and forgotten in the other fails at init.
var guideOrder = []Field{
	FieldPriority,
	FieldTag,
	FieldRemoveTag,
	FieldContext,
	FieldDue,
	FieldDefer,
	FieldDestination,
	FieldPinned,
	FieldProject,
	FieldParent,
	FieldChild,
	FieldDependency,
	FieldRemoveDependency,
	FieldAttachment,
	FieldComment,
	FieldTaskType,
}

// Definitions returns defensive copies of the canonical definitions available
// on a surface, in parse order.
//
// A field the parser recognizes on a surface but declares no form for there is
// deliberately absent: recognition without a form is how a token is refused by
// name instead of folding into the title, and advertising a formless entry would
// teach a syntax the surface does not accept. `-~task` is the one such entry
// today — capture tokenizes it only to say removal is not a capture operation.
func Definitions(surface Surface) []Definition {
	result := make([]Definition, 0, len(registry))
	for _, spec := range registry {
		if spec.Legacy || spec.Surfaces&surface == 0 {
			continue
		}
		forms := formsForSurface(spec.forms, surface)
		if len(forms) == 0 {
			continue
		}
		def := spec.Definition
		if surface == SurfaceSearch {
			def.Description = spec.searchDescription
		}
		def.Forms = slices.Clone(forms)
		def.Values = slices.Clone(def.Values)
		def.RetiredValues = slices.Clone(def.RetiredValues)
		def.Examples = slices.Clone(def.Examples)
		def.Clears = clearsForSurface(def.Clears, surface, spec.searchClearText)
		result = append(result, def)
	}
	return result
}

// GuideDefinitions returns the same definitions Definitions returns for a
// surface, in teach order. Every surface that presents the grammar to a
// human — the capture syntax guide today — reads this projection, so parse
// order stays free to move a field for disambiguation alone.
func GuideDefinitions(surface Surface) []Definition {
	available := Definitions(surface)
	byField := make(map[Field]Definition, len(available))
	for _, def := range available {
		byField[def.Field] = def
	}
	result := make([]Definition, 0, len(available))
	for _, field := range guideOrder {
		if def, ok := byField[field]; ok {
			result = append(result, def)
		}
	}
	return result
}

// DefinitionFor returns the definition for a field on a surface.
func DefinitionFor(surface Surface, field Field) (Definition, bool) {
	for _, def := range Definitions(surface) {
		if def.Field == field {
			return def, true
		}
	}
	return Definition{}, false
}

func formsForSurface(forms []surfaceForm, surface Surface) []string {
	result := make([]string, 0, len(forms))
	for _, form := range forms {
		if form.Surfaces&surface != 0 {
			result = append(result, form.Text)
		}
	}
	return result
}

func clearsForSurface(clears []ClearOperation, surface Surface, searchText map[string]string) []ClearOperation {
	result := make([]ClearOperation, 0, len(clears))
	for _, clear := range clears {
		if clear.Surfaces&surface == 0 {
			continue
		}
		if surface == SurfaceSearch {
			clear.Description = searchText[clear.Value]
		}
		result = append(result, clear)
	}
	return result
}

func init() {
	if err := validateRegistry(); err != nil {
		panic(err)
	}
}

// validateRegistry checks the invariants that keep parser and presentation
// projections safe to generate from the canonical entries.
func validateRegistry() error {
	seenFields := make(map[Field]bool, len(registry))
	seenTriggers := make(map[string]Field, len(registry))
	seenForms := make(map[string]Field, len(registry))
	for _, spec := range registry {
		if err := validateDefinitionIdentity(spec, seenFields, seenTriggers); err != nil {
			return err
		}
		if err := validateDefinitionValues(spec); err != nil {
			return err
		}
		if err := validateDefinitionForms(spec, seenForms); err != nil {
			return err
		}
		if err := validateDefinitionClears(spec); err != nil {
			return err
		}
		if err := validateDefinitionDescriptions(spec); err != nil {
			return err
		}
	}
	if err := validateGuideOrder(); err != nil {
		return err
	}
	return validateDestinationDefinition()
}

// validateGuideOrder asserts parse order and teach order hold identical sets.
// Splitting the two lists is what lets a field be parsed early and taught late,
// and this is the fence that makes the split safe: a field added to one list and
// forgotten in the other would otherwise silently vanish from the syntax guide
// or gain a second, unreachable entry.
func validateGuideOrder() error {
	seen := make(map[Field]bool, len(guideOrder))
	for _, field := range guideOrder {
		if seen[field] {
			return fmt.Errorf("grammar guide order lists %q twice", field)
		}
		seen[field] = true
		if _, ok := registrySpecFor(field); !ok {
			return fmt.Errorf("grammar guide order names %q, which the registry does not declare", field)
		}
	}
	for _, spec := range registry {
		if !seen[spec.Field] {
			return fmt.Errorf("grammar field %q is missing from the guide order", spec.Field)
		}
	}
	return nil
}

func validateDefinitionIdentity(spec definitionSpec, seenFields map[Field]bool, seenTriggers map[string]Field) error {
	if spec.Field == "" || spec.Trigger == "" || spec.pattern == "" {
		return fmt.Errorf("grammar definition %q is incomplete", spec.Field)
	}
	if seenFields[spec.Field] {
		return fmt.Errorf("grammar field %q is duplicated", spec.Field)
	}
	seenFields[spec.Field] = true
	if previous, exists := seenTriggers[spec.Trigger]; exists {
		return fmt.Errorf("grammar trigger %q is shared by %s and %s", spec.Trigger, previous, spec.Field)
	}
	seenTriggers[spec.Trigger] = spec.Field
	if spec.Surfaces == 0 || spec.Surfaces&^allSurfaces != 0 {
		return fmt.Errorf("grammar field %s has invalid surfaces %d", spec.Field, spec.Surfaces)
	}
	if spec.Cardinality != CardinalitySingleton && spec.Cardinality != CardinalityRepeatable {
		return fmt.Errorf("grammar field %s has invalid cardinality %d", spec.Field, spec.Cardinality)
	}
	return nil
}

func validateDefinitionValues(spec definitionSpec) error {
	seenValues := make(map[string]bool, len(spec.Values)+len(spec.RetiredValues))
	for _, value := range spec.Values {
		if value == "" || seenValues[value] {
			return fmt.Errorf("grammar field %s has invalid static value %q", spec.Field, value)
		}
		seenValues[value] = true
	}
	for _, retired := range spec.RetiredValues {
		if retired == "" || seenValues[retired] {
			return fmt.Errorf("grammar field %s has invalid retired value %q", spec.Field, retired)
		}
		seenValues[retired] = true
	}
	return nil
}

func validateDefinitionForms(spec definitionSpec, seenForms map[string]Field) error {
	for _, form := range spec.forms {
		if form.Text == "" || !strings.HasPrefix(form.Text, spec.Trigger) {
			return fmt.Errorf("grammar field %s has invalid form %q", spec.Field, form.Text)
		}
		if form.Surfaces == 0 || form.Surfaces&^spec.Surfaces != 0 {
			return fmt.Errorf("grammar form %q has invalid surfaces %d", form.Text, form.Surfaces)
		}
		if previous, exists := seenForms[form.Text]; exists {
			return fmt.Errorf("grammar form %q is shared by %s and %s", form.Text, previous, spec.Field)
		}
		seenForms[form.Text] = spec.Field
	}
	return nil
}

func validateDefinitionClears(spec definitionSpec) error {
	for _, clear := range spec.Clears {
		if clear.Value == "" || clear.Surfaces == 0 || clear.Surfaces&^spec.Surfaces != 0 {
			return fmt.Errorf("grammar field %s has invalid clear semantics", spec.Field)
		}
		clearForm := spec.Trigger + clear.Value
		found := slices.ContainsFunc(spec.forms, func(form surfaceForm) bool {
			return form.Text == clearForm && form.Surfaces&clear.Surfaces == clear.Surfaces
		})
		if !found {
			return fmt.Errorf("grammar field %s clear value %q has no matching form", spec.Field, clear.Value)
		}
	}
	return nil
}

// validateDefinitionDescriptions keeps the search projection from advertising a
// mutation sentence: a field reaching search must carry predicate wording, and
// so must every clear value that reaches it.
func validateDefinitionDescriptions(spec definitionSpec) error {
	if spec.Surfaces&SurfaceSearch != 0 && !spec.Legacy && spec.searchDescription == "" {
		return fmt.Errorf("grammar field %s has no search description", spec.Field)
	}
	if spec.Surfaces&SurfaceSearch == 0 && spec.searchDescription != "" {
		return fmt.Errorf("grammar field %s declares a search description off the search surface", spec.Field)
	}
	for value, text := range spec.searchClearText {
		if text == "" {
			return fmt.Errorf("grammar field %s has empty search text for clear %q", spec.Field, value)
		}
		found := slices.ContainsFunc(spec.Clears, func(clear ClearOperation) bool {
			return clear.Value == value && clear.Surfaces&SurfaceSearch != 0
		})
		if !found {
			return fmt.Errorf("grammar field %s search text %q names no search clear", spec.Field, value)
		}
	}
	for _, clear := range spec.Clears {
		if clear.Surfaces&SurfaceSearch != 0 && spec.searchClearText[clear.Value] == "" {
			return fmt.Errorf("grammar field %s search clear %q has no search text", spec.Field, clear.Value)
		}
	}
	return nil
}

func validateDestinationDefinition() error {
	destination, ok := registrySpecFor(FieldDestination)
	if !ok {
		return fmt.Errorf("destination grammar is missing")
	}
	wantDestinations := []string{"today", "next"}
	for _, state := range models.AllWorkflowStates() {
		wantDestinations = append(wantDestinations, state.String())
	}
	if len(destination.Values) != len(wantDestinations) {
		return fmt.Errorf("destination grammar has %d values; want %d", len(destination.Values), len(wantDestinations))
	}
	for _, want := range wantDestinations {
		if !slices.Contains(destination.Values, want) {
			return fmt.Errorf("destination grammar is missing %q", want)
		}
	}
	return nil
}

func registrySpecFor(field Field) (definitionSpec, bool) {
	for _, spec := range registry {
		if spec.Field == field {
			return spec, true
		}
	}
	return definitionSpec{}, false
}

// StateDescription returns concise presentation text for a canonical state.
func StateDescription(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "unprocessed inbox"
	case models.StateBacklog:
		return "backlog"
	case models.StateDoing:
		return "doing"
	case models.StateWaiting:
		return "waiting externally"
	case models.StatePaused:
		return "paused"
	case models.StateSomeday:
		return "deferred someday"
	case models.StateDone:
		return "completed"
	case models.StateCanceled:
		return "canceled"
	default:
		return ""
	}
}

// ParsePriority resolves a canonical numeric priority. Callers may opt into an
// inclusive N-M range for legacy input inspection.
func ParsePriority(value string, allowRange bool) ([]int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	definition, ok := registrySpecFor(FieldPriority)
	if !ok {
		return nil, false
	}
	if allowRange {
		if parts := strings.Split(value, "-"); len(parts) == 2 {
			start, startErr := strconv.Atoi(parts[0])
			end, endErr := strconv.Atoi(parts[1])
			if startErr == nil && endErr == nil && slices.Contains(definition.Values, parts[0]) && slices.Contains(definition.Values, parts[1]) {
				if start > end {
					start, end = end, start
				}
				result := make([]int, 0, end-start+1)
				for priority := start; priority <= end; priority++ {
					result = append(result, priority)
				}
				return result, true
			}
		}
	}

	priority, err := strconv.Atoi(value)
	if err != nil || !slices.Contains(definition.Values, value) {
		return nil, false
	}
	return []int{priority}, true
}

// InvalidPriorityError formats capture guidance from the canonical values.
func InvalidPriorityError(raw string) error {
	definition, ok := registrySpecFor(FieldPriority)
	if !ok {
		return fmt.Errorf("invalid priority: !%s", raw)
	}
	values := prefixed(definition.Values, definition.Trigger)
	if slices.Contains(definition.RetiredValues, raw) {
		return fmt.Errorf("priority !%s was removed (use %s)", raw, joinOptions(values))
	}
	return fmt.Errorf("invalid priority: !%s (use %s)", raw, joinOptions(values))
}

func joinOptions(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " or " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", or " + values[len(values)-1]
	}
}

func prefixed(values []string, prefix string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = prefix + value
	}
	return result
}
