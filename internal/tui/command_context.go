package tui

import (
	"fmt"
	"strings"
)

// Capability is a declarative feature exposed by the active route or overlay.
// Routes and overlays construct CommandContext values from their active mode.
type Capability string

const (
	CapabilityCommandDiscovery Capability = "command-discovery"
	// CapabilitySimplifiedInput exposes the staged final key grammar without
	// making its operations visible to the legacy command adapter.
	CapabilitySimplifiedInput Capability = "input:simplified"
	// CapabilityScopeHub limits scope/context lifecycle operations to the
	// routed Scope Hub that supplies their frozen targets.
	CapabilityScopeHub Capability = "surface:scope-hub"
	// CapabilityProjectHub limits global project creation to the routed project
	// selection and administration surface.
	CapabilityProjectHub   Capability = "surface:project-hub"
	CapabilityAppScope     Capability = "scope:app"
	CapabilityProjectScope Capability = "scope:project"
	CapabilityTaskScope    Capability = "scope:task"
)

// capabilityFamilies are the prefixes reserved for capabilities generated per
// route, overlay, presentation, scope, surface, or input strategy.
var capabilityFamilies = []string{
	"presentation:",
	"scope:",
	"route:",
	"overlay:",
	"surface:",
	"input:",
}

// validCapability reports whether a capability is a declared constant or a
// member of a reserved dynamic family. It catches typos at registration.
func validCapability(capability Capability) bool {
	switch capability { //nolint:exhaustive // Test-only harness capabilities are registered separately.
	case CapabilityCommandDiscovery,
		CapabilitySimplifiedInput,
		CapabilityScopeHub,
		CapabilityProjectHub,
		CapabilityAppScope,
		CapabilityProjectScope,
		CapabilityTaskScope:
		return true
	}
	for _, family := range capabilityFamilies {
		if strings.HasPrefix(string(capability), family) &&
			len(capability) > len(family) {
			return true
		}
	}
	return false
}

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

// SelectionContext preserves project-qualified task identity independently of
// cursor or row position.
type SelectionContext struct {
	Tasks []TaskIdentity
	// Entities mirrors Tasks with the lifecycle and project mutability facts
	// needed to decide whether an operation applies to every selected target.
	// It stays pointer-free so a frozen action sheet cannot be retargeted by a
	// later row rebuild.
	Entities []EntityContext
	// EntityIdentities is the typed selection for surfaces whose rows are not
	// tasks. Hosts that only select tasks may leave it empty; selected-entity
	// resolution then derives it from Tasks.
	EntityIdentities []EntityIdentity
	// Visual distinguishes an explicit visual selection (including one selected
	// task) from the ordinary cursor fallback used by selected-target operations.
	Visual bool
}

// Count returns the number of selected tasks.
func (s SelectionContext) Count() int { return len(s.Tasks) }

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
	// CurrentEntity is the typed identity of the focused row. Surfaces whose
	// rows are not tasks or projects set it directly; task and project surfaces
	// may leave it empty and let resolution derive it from Entity.
	CurrentEntity EntityIdentity
	// ScopeProjectIDs is the stable project set admitted by the active scope.
	// It lets operation execution resolve scoped-project targets without
	// depending on renderer models or palette-specific messages.
	ScopeProjectIDs []string
	// FocusedProjectID is the project the active scope is focused on, empty at
	// base or multi-project scope. It is deliberately independent of the cursor
	// so a frozen tool invocation cannot borrow an unrelated row's project.
	FocusedProjectID string
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
