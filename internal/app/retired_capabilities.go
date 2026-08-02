package app

import (
	"errors"
	"fmt"
)

// RetiredCapability identifies a mutation family that schema 6 cannot
// represent. The legacy model and schema-5 codecs retain their fields until
// their scheduled cleanup, but no application service may create, change, or
// remove those values after this boundary is installed.
type RetiredCapability string

const (
	RetiredCapabilityDependency RetiredCapability = "dependency"
	RetiredCapabilityDecision   RetiredCapability = "decision"
)

// ErrRetiredCapability is the stable error category returned by every retired
// mutation seam. Callers should use errors.Is rather than matching text.
var ErrRetiredCapability = errors.New("retired capability")

// RetiredCapabilityError names the unavailable mutation family while
// preserving a single errors.Is category for adapters and tests.
type RetiredCapabilityError struct {
	Capability RetiredCapability
}

func (e *RetiredCapabilityError) Error() string {
	return fmt.Sprintf("%s mutations are retired and unavailable", e.Capability)
}

func (e *RetiredCapabilityError) Unwrap() error { return ErrRetiredCapability }

// RejectRetiredCapability is the centralized fail-closed gate shared by the
// application service and presentation adapters that still contain temporary
// compile-only legacy paths. It intentionally never consults workspace schema:
// schema-5 data remains readable and round-trippable, but mutations are locked
// down before schema 6 becomes live.
func RejectRetiredCapability(capability RetiredCapability) error {
	switch capability {
	case RetiredCapabilityDependency,
		RetiredCapabilityDecision:
		return &RetiredCapabilityError{Capability: capability}
	default:
		return fmt.Errorf("%w: unknown capability %q", ErrRetiredCapability, capability)
	}
}
