// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// Surface identifies a consumer of the shared inline grammar.
type Surface uint8

const (
	SurfaceCapture Surface = 1 << iota
	SurfaceEdit
	SurfaceOutline
)

const allSurfaces = SurfaceCapture | SurfaceEdit | SurfaceOutline

// Field identifies one family of inline metadata tokens.
type Field string

const (
	FieldPriority    Field = "priority"
	FieldTag         Field = "tag"
	FieldRemoveTag   Field = "remove_tag"
	FieldDue         Field = "due"
	FieldRecurrence  Field = "recurrence"
	FieldTaskType    Field = "type"
	FieldParent      Field = "parent"
	FieldChild       Field = "child"
	FieldProject     Field = "project"
	FieldDestination Field = "destination"
	FieldPinned      Field = "pinned"
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
)

type definitionSpec struct {
	Definition
	forms      []surfaceForm
	pattern    string
	validation tokenValidation
}

var priorityNames = []string{"1", "2", "3"}
var retiredPriorityNames = []string{"4", "5"}

const clearValue = "0"

var registry = []definitionSpec{
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
	},
	{
		Definition: Definition{
			Field: FieldTag, Group: "Priority & Tags", Trigger: "#",
			Description: "Add tag", Examples: []string{"#backend", "#bug", "#source:github"},
			Surfaces: allSurfaces, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: "#tag", Surfaces: allSurfaces}},
		pattern:    `(?:^|\s)#([\w][\w.:-]*)`,
		validation: validateTag,
	},
	{
		Definition: Definition{
			Field: FieldRemoveTag, Group: "Priority & Tags", Trigger: "-#",
			Description: "Remove tag", Examples: []string{"-#old", "-#source:github"},
			Surfaces: SurfaceEdit, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: "-#tag", Surfaces: SurfaceEdit}},
		pattern:    `(?:^|\s)-#([\w][\w.:-]*)`,
		validation: validateTag,
	},
	{
		Definition: Definition{
			Field: FieldDue, Group: "Schedule", Trigger: "@",
			Description: "Set due date",
			Values: []string{
				"today", "tomorrow", "+1d", "+3d", "+1w", "+2w", "+1m",
				"monday", "friday", "next monday", "next friday", "in 3 days", "in 1 week", "in 2 weeks",
			},
			Examples: []string{"@tomorrow", "@friday", "@+3d", "@next monday"},
			Surfaces: allSurfaces, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear due date", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "@date", Surfaces: allSurfaces},
			{Text: "@0", Surfaces: SurfaceEdit},
		},
		pattern: `(?i)(?:^|\s)@(next\s+\w+|in\s+\d+\s+\w+|\S+)`,
	},
	{
		Definition: Definition{
			Field: FieldDestination, Group: "Destination", Trigger: "$",
			Description: "Set lifecycle or planning destination", Values: destinationNames,
			Examples: prefixed(destinationNames, "$"), Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
		},
		forms:   []surfaceForm{{Text: "$destination", Surfaces: allSurfaces}},
		pattern: `(?:^|\s)\$(\w+)`,
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
			Clears:      []ClearOperation{{Value: "unpin", Description: "Unpin task", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "*pin", Surfaces: allSurfaces},
			{Text: "*unpin", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)\*(\S+)`, validation: validatePin,
	},
	{
		Definition: Definition{
			Field: FieldProject, Group: "Project Routing", Trigger: "%",
			Description: "Move to project", Examples: []string{"%project-slug"},
			Surfaces: SurfaceCapture | SurfaceEdit, Cardinality: CardinalitySingleton,
		},
		forms:   []surfaceForm{{Text: "%project", Surfaces: SurfaceCapture | SurfaceEdit}},
		pattern: `(?:^|\s)%(\S+)`,
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
	{
		Definition: Definition{
			Field: FieldRecurrence, Group: "Legacy", Trigger: "~", Description: "Retired recurrence",
			Examples: []string{"~weekly"}, Legacy: true, Surfaces: SurfaceCapture,
			Cardinality: CardinalitySingleton,
		},
		forms:   []surfaceForm{{Text: "~recurrence", Surfaces: SurfaceCapture}},
		pattern: `(?:^|\s)~(\S+)`,
	},
}

// Definitions returns defensive copies of the canonical definitions available
// on a surface.
func Definitions(surface Surface) []Definition {
	result := make([]Definition, 0, len(registry))
	for _, spec := range registry {
		if spec.Legacy || spec.Surfaces&surface == 0 {
			continue
		}
		def := spec.Definition
		def.Forms = formsForSurface(spec.forms, surface)
		def.Forms = slices.Clone(def.Forms)
		def.Values = slices.Clone(def.Values)
		def.RetiredValues = slices.Clone(def.RetiredValues)
		def.Examples = slices.Clone(def.Examples)
		def.Clears = clearsForSurface(def.Clears, surface)
		result = append(result, def)
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

func clearsForSurface(clears []ClearOperation, surface Surface) []ClearOperation {
	result := make([]ClearOperation, 0, len(clears))
	for _, clear := range clears {
		if clear.Surfaces&surface != 0 {
			result = append(result, clear)
		}
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
	}
	return validateDestinationDefinition()
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
