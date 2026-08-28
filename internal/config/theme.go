package config

// Theme groups palette colors together with background and cursor overrides for
// a single named theme (dark or light). Both themes are compiled in and there is
// no override channel: nothing unmarshals a theme from disk, and `T` toggles
// between the two built-ins. Empty strings mean "use ANSI default".
type Theme struct {
	Palette         PaletteColors
	BackgroundColor string
	CursorColor     string
	CursorTextColor string
}

// PaletteColors holds the per-role TUI colors of a compiled-in theme. There is
// no override channel; the values are only ever set by the built-in dark and
// light palettes. Empty strings mean "use ANSI default".
type PaletteColors struct {
	NormalText          string
	Muted               string
	Dimmed              string
	DimmedBg            string
	DimmedFg            string
	Primary             string
	Secondary           string
	Accent              string
	Success             string
	Warning             string
	Error               string
	Border              string
	Priority1Title      string
	Priority2Title      string
	Priority3Title      string
	TaskContext         string
	TaskContextDeferred string

	SemanticBugTitle              string
	SemanticQuestionTitle         string
	SemanticEpicTitle             string
	SemanticStoryTitle            string
	SemanticFeatureTitle          string
	SemanticIdeaTitle             string
	SemanticBugSelectedTitle      string
	SemanticQuestionSelectedTitle string
	SemanticEpicSelectedTitle     string
	SemanticStorySelectedTitle    string
	SemanticFeatureSelectedTitle  string
	SemanticIdeaSelectedTitle     string
	SemanticBugDeferredTitle      string
	SemanticQuestionDeferredTitle string
	SemanticEpicDeferredTitle     string
	SemanticStoryDeferredTitle    string
	SemanticFeatureDeferredTitle  string
	SemanticIdeaDeferredTitle     string
}
