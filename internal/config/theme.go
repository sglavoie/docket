package config

// Theme groups palette colors together with background and cursor overrides for
// a single named theme (dark or light). Empty strings mean "use ANSI default".
type Theme struct {
	Palette         PaletteColors `json:"palette,omitzero"`
	BackgroundColor string        `json:"background_color,omitempty"`
	CursorColor     string        `json:"cursor_color,omitempty"`
	CursorTextColor string        `json:"cursor_text_color,omitempty"`
}

// PaletteColors holds user-configurable TUI palette overrides.
// Empty strings mean "use ANSI default".
type PaletteColors struct {
	NormalText     string `json:"normal_text,omitempty"`
	Muted          string `json:"muted,omitempty"`
	Dimmed         string `json:"dimmed,omitempty"`
	DimmedBg       string `json:"dimmed_bg,omitempty"`
	DimmedFg       string `json:"dimmed_fg,omitempty"`
	Primary        string `json:"primary,omitempty"`
	Secondary      string `json:"secondary,omitempty"`
	Accent         string `json:"accent,omitempty"`
	Success        string `json:"success,omitempty"`
	Warning        string `json:"warning,omitempty"`
	Error          string `json:"error,omitempty"`
	Border         string `json:"border,omitempty"`
	Priority1Title string `json:"priority1_title,omitempty"`
	Priority2Title string `json:"priority2_title,omitempty"`
	Priority3Title string `json:"priority3_title,omitempty"`

	SemanticBugTitle             string `json:"semantic_bug_title,omitempty"`
	SemanticEpicTitle            string `json:"semantic_epic_title,omitempty"`
	SemanticStoryTitle           string `json:"semantic_story_title,omitempty"`
	SemanticFeatureTitle         string `json:"semantic_feature_title,omitempty"`
	SemanticIdeaTitle            string `json:"semantic_idea_title,omitempty"`
	SemanticBugSelectedTitle     string `json:"semantic_bug_selected_title,omitempty"`
	SemanticEpicSelectedTitle    string `json:"semantic_epic_selected_title,omitempty"`
	SemanticStorySelectedTitle   string `json:"semantic_story_selected_title,omitempty"`
	SemanticFeatureSelectedTitle string `json:"semantic_feature_selected_title,omitempty"`
	SemanticIdeaSelectedTitle    string `json:"semantic_idea_selected_title,omitempty"`
	SemanticBugSnoozedTitle      string `json:"semantic_bug_snoozed_title,omitempty"`
	SemanticEpicSnoozedTitle     string `json:"semantic_epic_snoozed_title,omitempty"`
	SemanticStorySnoozedTitle    string `json:"semantic_story_snoozed_title,omitempty"`
	SemanticFeatureSnoozedTitle  string `json:"semantic_feature_snoozed_title,omitempty"`
	SemanticIdeaSnoozedTitle     string `json:"semantic_idea_snoozed_title,omitempty"`
}
