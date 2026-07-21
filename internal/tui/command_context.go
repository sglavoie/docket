package tui

import "fmt"

// Capability is a declarative feature exposed by the active route or overlay.
// Routes and overlays construct CommandContext values from their active mode.
type Capability string

const (
	CapabilityCommandDiscovery Capability = "command-discovery"
	CapabilityAppScope         Capability = "scope:app"
	CapabilityProjectScope     Capability = "scope:project"
	CapabilityTaskScope        Capability = "scope:task"
)

// CapabilitySet is the active route/overlay capability set.
type CapabilitySet map[Capability]struct{}

// NewCapabilitySet builds a capability set from the supplied values.
func NewCapabilitySet(capabilities ...Capability) CapabilitySet {
	set := make(CapabilitySet, len(capabilities))
	for _, capability := range capabilities {
		set[capability] = struct{}{}
	}
	return set
}

// Has reports whether capability is present.
func (s CapabilitySet) Has(capability Capability) bool {
	_, ok := s[capability]
	return ok
}

// HasAll reports whether every required capability is present.
func (s CapabilitySet) HasAll(required CapabilitySet) bool {
	for capability := range required {
		if !s.Has(capability) {
			return false
		}
	}
	return true
}

// Add inserts capabilities into the set.
func (s CapabilitySet) Add(capabilities ...Capability) {
	for _, capability := range capabilities {
		s[capability] = struct{}{}
	}
}

// SelectionContext preserves stable entity identity independently of cursor or
// row position. TaskIDs may contain one focused task or a multi-selection.
type SelectionContext struct {
	ProjectID string
	TaskIDs   []string
}

// Count returns the number of selected tasks.
func (s SelectionContext) Count() int { return len(s.TaskIDs) }

// EntityContext describes the entity a command would act on. State remains a
// string at this shell seam so command discovery does not depend on a domain
// model package or force route migrations to share a concrete entity type.
type EntityContext struct {
	ProjectID string
	TaskID    string
	State     string
	Mutable   bool
	Archived  bool
}

// CommandContext is the complete input to capability-based command
// availability. Existing commands continue to use Views and Available; new or
// migrated commands may additionally declare RequiredCapabilities and
// ContextAvailable.
type CommandContext struct {
	View         PresentationID
	Scope        RouteScope
	Capabilities CapabilitySet
	Selection    SelectionContext
	Entity       EntityContext
}

// presentationCapability scopes mature renderer commands without affecting
// typed route identity or history.
func presentationCapability(view PresentationID) Capability {
	return Capability(fmt.Sprintf("presentation:%d", view))
}

// PresentationCommandContext builds discovery capabilities for one renderer.
func PresentationCommandContext(view PresentationID) CommandContext {
	scope := presentationScope(view)
	capabilities := NewCapabilitySet(CapabilityCommandDiscovery, presentationCapability(view))
	switch scope {
	case ScopeApp:
		capabilities.Add(CapabilityAppScope)
	case ScopeTask:
		capabilities.Add(CapabilityTaskScope)
	default:
		capabilities.Add(CapabilityProjectScope)
	}
	return CommandContext{
		View:         view,
		Scope:        scope,
		Capabilities: capabilities,
	}
}
