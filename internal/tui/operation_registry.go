package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// OperationCategory is the stable product grouping used by action sheets,
// help, direct grammar, and harness discovery.
type OperationCategory string

const (
	OperationCategoryTask       OperationCategory = "task"
	OperationCategoryNavigation OperationCategory = "navigation"
	OperationCategoryMode       OperationCategory = "mode"
	OperationCategoryScope      OperationCategory = "scope"
	OperationCategoryFilter     OperationCategory = "filter"
	OperationCategoryProject    OperationCategory = "project"
	OperationCategoryNote       OperationCategory = "note"
	OperationCategoryTag        OperationCategory = "tag"
	OperationCategorySystem     OperationCategory = "system"
)

// TargetCardinality declares how many resolved targets an operation accepts.
// Target identity and resolution remain execution concerns; the registry only
// describes whether a prospective target set has a valid size.
type TargetCardinality string

const (
	TargetCardinalityNone      TargetCardinality = "none"
	TargetCardinalityOne       TargetCardinality = "one"
	TargetCardinalityOneOrMore TargetCardinality = "one-or-more"
)

// Accepts reports whether count satisfies the declared target cardinality.
func (cardinality TargetCardinality) Accepts(count int) bool {
	switch cardinality {
	case TargetCardinalityNone:
		return count == 0
	case TargetCardinalityOne:
		return count == 1
	case TargetCardinalityOneOrMore:
		return count >= 1
	default:
		return false
	}
}

// OperationRisk is declarative execution policy. It deliberately carries no
// confirmation or handler behavior; the execution layer consumes it.
type OperationRisk string

const (
	OperationRiskReadOnly    OperationRisk = "read-only"
	OperationRiskMutation    OperationRisk = "mutation"
	OperationRiskDestructive OperationRisk = "destructive"
)

// ValueProviderID identifies the picker or other value source an operation
// needs before execution. The empty value means no provider is required.
type ValueProviderID string

const (
	ValueProviderState     ValueProviderID = "state"
	ValueProviderPriority  ValueProviderID = "priority"
	ValueProviderDueDate   ValueProviderID = "due-date"
	ValueProviderSnooze    ValueProviderID = "snooze"
	ValueProviderTags      ValueProviderID = "tags"
	ValueProviderProject   ValueProviderID = "project"
	ValueProviderParent    ValueProviderID = "parent"
	ValueProviderNoteTitle ValueProviderID = "note-title"
	ValueProviderTagName   ValueProviderID = "tag-name"

	// ValueProviderProjectTags edits a project's own tag set, which is a
	// different vocabulary and a different target from the task tags on
	// ValueProviderTags.
	ValueProviderProjectTags ValueProviderID = "project-tags"

	// Filter providers feed the Outliner's view-state filter facets rather than
	// a task mutation; priority is preset-only while states, tags, due, and
	// completion age use pickers.
	ValueProviderFilterPriority      ValueProviderID = "filter-priority"
	ValueProviderFilterStates        ValueProviderID = "filter-states"
	ValueProviderFilterTags          ValueProviderID = "filter-tags"
	ValueProviderFilterDue           ValueProviderID = "filter-due"
	ValueProviderFilterCompletionAge ValueProviderID = "filter-completion-age"

	// ValueProviderFilterProjectTags feeds the population facet: a different
	// vocabulary from ValueProviderFilterTags, and a different question. It picks
	// which projects load rather than which of a loaded project's rows show.
	ValueProviderFilterProjectTags ValueProviderID = "filter-project-tags"

	// ValueProviderProjectTagDestination names the tag that inherits the members a
	// vocabulary-wide tag removal would otherwise strand.
	ValueProviderProjectTagDestination ValueProviderID = "project-tag-destination"
)

// textEntry reports whether the provider's value is typed rather than picked
// from a list, so the value step opens the text prompt instead of the picker.
func (id ValueProviderID) textEntry() bool {
	return id == ValueProviderNoteTitle || id == ValueProviderTagName
}

// tagEntry reports whether the picker offers "a" to mint a tag the vocabulary
// does not contain yet. Both tag namespaces need it because neither is a closed
// enumeration the way states and priorities are.
func (id ValueProviderID) tagEntry() bool {
	return id == ValueProviderTags || id == ValueProviderProjectTags
}

func withValueProvider(descriptor OperationDescriptor, provider ValueProviderID) OperationDescriptor {
	descriptor.ValueProviderID = provider
	return descriptor
}

// OperationApplicability is a handler-free predicate over the active command
// context. RequiredCapabilities are checked before this predicate.
type OperationApplicability func(CommandContext) bool

// OperationDescriptor is the complete non-executable description of one
// stable operation. Execution handlers intentionally do not live here.
type OperationDescriptor struct {
	ID                   OperationID
	Label                string
	Description          string
	Aliases              []string
	Category             OperationCategory
	DirectKey            string
	TargetSource         OperationTargetSource
	TargetCardinality    TargetCardinality
	RequiredCapabilities CapabilitySet
	Applicability        OperationApplicability
	Risk                 OperationRisk
	ValueProviderID      ValueProviderID
}

// Applicable reports whether the descriptor can be offered in ctx.
func (descriptor OperationDescriptor) Applicable(ctx CommandContext) bool {
	if !ctx.Capabilities.HasAll(descriptor.RequiredCapabilities) {
		return false
	}
	return descriptor.Applicability == nil || descriptor.Applicability(ctx)
}

// OperationRegistry owns stable, handler-free operation descriptors.
type OperationRegistry struct {
	descriptors     map[OperationID]OperationDescriptor
	inputStrategies map[InputStrategyID]InputStrategy
	inputChords     map[InputStrategyID]*inputChordNode
}

// NewOperationRegistry creates an empty registry.
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{
		descriptors:     make(map[OperationID]OperationDescriptor),
		inputStrategies: make(map[InputStrategyID]InputStrategy),
		inputChords:     make(map[InputStrategyID]*inputChordNode),
	}
}

// Register validates and adds one descriptor. Duplicate IDs are always
// rejected. An unconditional direct-key collision is rejected when the two
// required-capability sets may be active together; predicate-dependent
// collisions are checked by ForContext and ValidateContext.
func (registry *OperationRegistry) Register(descriptor OperationDescriptor) error {
	return registry.register(descriptor, true)
}

func (registry *OperationRegistry) register(descriptor OperationDescriptor, checkStaticShortcut bool) error {
	if registry.descriptors == nil {
		registry.descriptors = make(map[OperationID]OperationDescriptor)
	}
	normalized, err := normalizeOperationDescriptor(descriptor)
	if err != nil {
		return err
	}
	if _, exists := registry.descriptors[normalized.ID]; exists {
		return fmt.Errorf("operation ID already registered: %s", normalized.ID)
	}
	if checkStaticShortcut && normalized.DirectKey != "" && normalized.Applicability == nil {
		for _, existing := range registry.descriptors {
			if existing.DirectKey != normalized.DirectKey || existing.Applicability != nil {
				continue
			}
			if operationCapabilityContextsOverlap(existing.RequiredCapabilities, normalized.RequiredCapabilities) {
				return fmt.Errorf(
					"operation shortcut already registered in an overlapping context: %s (%s vs %s)",
					normalized.DirectKey,
					existing.ID,
					normalized.ID,
				)
			}
		}
	}
	registry.descriptors[normalized.ID] = normalized
	return nil
}

// Descriptor returns a defensive copy of the descriptor for id.
func (registry *OperationRegistry) Descriptor(id OperationID) (OperationDescriptor, bool) {
	if registry == nil {
		return OperationDescriptor{}, false
	}
	descriptor, ok := registry.descriptors[id]
	if !ok {
		return OperationDescriptor{}, false
	}
	return cloneOperationDescriptor(descriptor), true
}

// All returns every descriptor in deterministic category/label/ID order.
func (registry *OperationRegistry) All() []OperationDescriptor {
	if registry == nil || len(registry.descriptors) == 0 {
		return []OperationDescriptor{}
	}
	descriptors := make([]OperationDescriptor, 0, len(registry.descriptors))
	for _, descriptor := range registry.descriptors {
		descriptors = append(descriptors, cloneOperationDescriptor(descriptor))
	}
	sortOperationDescriptors(descriptors)
	return descriptors
}

// ForContext returns applicable descriptors in deterministic order and rejects
// any direct-key collision that is actually active in ctx.
func (registry *OperationRegistry) ForContext(ctx CommandContext) ([]OperationDescriptor, error) {
	descriptors := make([]OperationDescriptor, 0)
	for _, descriptor := range registry.All() {
		if descriptor.Applicable(ctx) {
			descriptors = append(descriptors, descriptor)
		}
	}
	if err := validateActiveOperationShortcuts(descriptors); err != nil {
		return nil, err
	}
	return descriptors, nil
}

// ValidateContext verifies that applicable operations have unique direct keys.
func (registry *OperationRegistry) ValidateContext(ctx CommandContext) error {
	_, err := registry.ForContext(ctx)
	return err
}

func normalizeOperationDescriptor(descriptor OperationDescriptor) (OperationDescriptor, error) {
	descriptor.ID = OperationID(strings.TrimSpace(string(descriptor.ID)))
	descriptor.Label = strings.TrimSpace(descriptor.Label)
	descriptor.Description = strings.TrimSpace(descriptor.Description)
	descriptor.DirectKey = normalizeOperationKey(descriptor.DirectKey)
	descriptor.ValueProviderID = ValueProviderID(strings.TrimSpace(string(descriptor.ValueProviderID)))
	if descriptor.ID == "" {
		return OperationDescriptor{}, fmt.Errorf("operation ID is required")
	}
	if descriptor.Label == "" {
		return OperationDescriptor{}, fmt.Errorf("operation %s label is required", descriptor.ID)
	}
	if !validOperationCategory(descriptor.Category) {
		return OperationDescriptor{}, fmt.Errorf("operation %s has invalid category %q", descriptor.ID, descriptor.Category)
	}
	if !validTargetCardinality(descriptor.TargetCardinality) {
		return OperationDescriptor{}, fmt.Errorf(
			"operation %s has invalid target cardinality %q",
			descriptor.ID,
			descriptor.TargetCardinality,
		)
	}
	if descriptor.TargetSource == "" {
		descriptor.TargetSource = defaultOperationTargetSource(
			descriptor.Category,
			descriptor.TargetCardinality,
		)
	}
	if !validOperationTargetSource(descriptor.TargetSource) {
		return OperationDescriptor{}, fmt.Errorf(
			"operation %s has invalid target source %q",
			descriptor.ID,
			descriptor.TargetSource,
		)
	}
	if !validOperationRisk(descriptor.Risk) {
		return OperationDescriptor{}, fmt.Errorf("operation %s has invalid risk %q", descriptor.ID, descriptor.Risk)
	}
	if err := validateOperationCapabilities(descriptor.ID, descriptor.RequiredCapabilities); err != nil {
		return OperationDescriptor{}, err
	}
	descriptor.Aliases = normalizeOperationAliases(descriptor.Label, descriptor.Aliases)
	descriptor.RequiredCapabilities = cloneCapabilitySet(descriptor.RequiredCapabilities)
	return descriptor, nil
}

func validateOperationCapabilities(id OperationID, capabilities CapabilitySet) error {
	for capability := range capabilities {
		if !validCapability(capability) {
			return fmt.Errorf("operation %s has unknown required capability %q", id, capability)
		}
	}
	return nil
}

func cloneOperationDescriptor(descriptor OperationDescriptor) OperationDescriptor {
	descriptor.Aliases = append([]string(nil), descriptor.Aliases...)
	descriptor.RequiredCapabilities = cloneCapabilitySet(descriptor.RequiredCapabilities)
	return descriptor
}

func cloneCapabilitySet(capabilities CapabilitySet) CapabilitySet {
	if len(capabilities) == 0 {
		return nil
	}
	cloned := make(CapabilitySet, len(capabilities))
	for capability := range capabilities {
		cloned[capability] = struct{}{}
	}
	return cloned
}

func normalizeOperationAliases(label string, aliases []string) []string {
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(label)): true}
	normalized := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		key := strings.ToLower(alias)
		if alias == "" || seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, alias)
	}
	sort.Slice(normalized, func(i, j int) bool {
		left := strings.ToLower(normalized[i])
		right := strings.ToLower(normalized[j])
		if left != right {
			return left < right
		}
		return normalized[i] < normalized[j]
	})
	return normalized
}

func normalizeOperationKey(key string) string {
	key = strings.TrimSpace(key)
	if utf8.RuneCountInString(key) <= 1 {
		return key
	}
	return strings.ToLower(key)
}

func validOperationCategory(category OperationCategory) bool {
	switch category {
	case OperationCategoryTask,
		OperationCategoryNavigation,
		OperationCategoryMode,
		OperationCategoryScope,
		OperationCategoryFilter,
		OperationCategoryProject,
		OperationCategoryNote,
		OperationCategoryTag,
		OperationCategorySystem:
		return true
	default:
		return false
	}
}

func validTargetCardinality(cardinality TargetCardinality) bool {
	switch cardinality {
	case TargetCardinalityNone, TargetCardinalityOne, TargetCardinalityOneOrMore:
		return true
	default:
		return false
	}
}

func validOperationRisk(risk OperationRisk) bool {
	switch risk {
	case OperationRiskReadOnly, OperationRiskMutation, OperationRiskDestructive:
		return true
	default:
		return false
	}
}

func sortOperationDescriptors(descriptors []OperationDescriptor) {
	sort.SliceStable(descriptors, func(i, j int) bool {
		leftCategory := operationCategoryRank(descriptors[i].Category)
		rightCategory := operationCategoryRank(descriptors[j].Category)
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		leftLabel := strings.ToLower(descriptors[i].Label)
		rightLabel := strings.ToLower(descriptors[j].Label)
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		return descriptors[i].ID < descriptors[j].ID
	})
}

func operationCategoryRank(category OperationCategory) int {
	switch category {
	case OperationCategoryTask:
		return 0
	case OperationCategoryNavigation:
		return 1
	case OperationCategoryMode:
		return 2
	case OperationCategoryScope:
		return 3
	case OperationCategoryFilter:
		return 4
	case OperationCategoryProject:
		return 5
	case OperationCategoryNote:
		return 6
	case OperationCategoryTag:
		return 7
	default:
		return 8
	}
}

func validateActiveOperationShortcuts(descriptors []OperationDescriptor) error {
	owners := make(map[string]OperationID)
	for _, descriptor := range descriptors {
		if descriptor.DirectKey == "" {
			continue
		}
		if owner, exists := owners[descriptor.DirectKey]; exists && owner != descriptor.ID {
			return fmt.Errorf(
				"operation shortcut collision in active context: %s (%s vs %s)",
				descriptor.DirectKey,
				owner,
				descriptor.ID,
			)
		}
		owners[descriptor.DirectKey] = descriptor.ID
	}
	return nil
}

func operationCapabilityContextsOverlap(left, right CapabilitySet) bool {
	for _, family := range []string{"scope:", "presentation:", "route:", "overlay:"} {
		leftValue := capabilityFamilyValue(left, family)
		rightValue := capabilityFamilyValue(right, family)
		if leftValue != "" && rightValue != "" && leftValue != rightValue {
			return false
		}
	}
	return true
}

func capabilityFamilyValue(capabilities CapabilitySet, family string) string {
	for capability := range capabilities {
		value := string(capability)
		if strings.HasPrefix(value, family) {
			return value
		}
	}
	return ""
}
