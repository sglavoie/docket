//go:build !darwin

package appearance

type noopDetector struct{}

func newDetector() Detector { return noopDetector{} }

func (d noopDetector) Detect() SystemAppearance { return Unknown }
