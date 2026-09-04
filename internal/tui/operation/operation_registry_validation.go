package operation

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

func normalizeOperationDescriptor(descriptor OperationDescriptor) (OperationDescriptor, error) {
	descriptor.ID = OperationID(strings.TrimSpace(string(descriptor.ID)))
	descriptor.Label = strings.TrimSpace(descriptor.Label)
	descriptor.Description = strings.TrimSpace(descriptor.Description)
	descriptor.DirectKey = NormalizeOperationKey(descriptor.DirectKey)
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
	if !ValidOperationTargetSource(descriptor.TargetSource) {
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

func NormalizeOperationKey(key string) string {
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
		OperationCategorySort,
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
	case OperationCategorySort:
		return 5
	case OperationCategoryProject:
		return 6
	case OperationCategoryNote:
		return 7
	case OperationCategoryTag:
		return 8
	default:
		return 9
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
