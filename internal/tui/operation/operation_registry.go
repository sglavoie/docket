package operation

import (
	"fmt"
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
	OperationCategorySort       OperationCategory = "sort"
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
//
// The checklist rule lives here rather than in the Applicability field so that
// a descriptor written without a predicate stays unconditional, which is what
// Register reads when it refuses a static direct-key collision.
func (descriptor OperationDescriptor) Applicable(ctx CommandContext) bool {
	if !ctx.Capabilities.HasAll(descriptor.RequiredCapabilities) {
		return false
	}
	if ctx.Entity.HasChecklistCursor && operationRefusedOnChecklistRow(descriptor) {
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
