package config

type RecentPaletteItem struct {
	Category string `json:"category"`
	ID       string `json:"id"`
	Count    int    `json:"count,omitempty"`
}

const (
	// CurrentTUIStateVersion versions route and presentation state inside the
	// machine, planning, and settings domains independently from their envelope
	// schema. Each owning domain can therefore migrate and commit atomically.
	//
	// Version 3 retires the configurable startup destination (start view/route),
	// Home layout sizing, and sidebar state: the Outliner is the sole root and
	// always launches globally in Today, so those fields are ignored on decode
	// and omitted on the next save.
	CurrentTUIStateVersion = 3

	// MaxRecentPaletteItems is the durable cap for the one palette history that
	// survives the TUI-state migration.
	MaxRecentPaletteItems = 10
)
