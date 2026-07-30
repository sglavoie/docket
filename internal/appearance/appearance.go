package appearance

// SystemAppearance represents the detected OS color scheme.
type SystemAppearance string

const (
	Dark    SystemAppearance = "dark"
	Light   SystemAppearance = "light"
	Unknown SystemAppearance = "unknown"
)

// Detector reports the current OS appearance.
type Detector interface {
	Detect() SystemAppearance
}
