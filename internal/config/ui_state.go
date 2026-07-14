package config

type RecentPaletteItem struct {
	Category string `json:"category"`
	ID       string `json:"id"`
	Count    int    `json:"count,omitempty"`
}

const DefaultStartView = "focus-board"

type InfoPanelMode int

const (
	InfoPanelOff            InfoPanelMode = 0
	InfoPanelInfoCards      InfoPanelMode = 1
	InfoPanelProjectSummary InfoPanelMode = 2
	infoPanelModeCount                    = 3
)

type DashboardBottomLayout int

const (
	DashboardLayoutHorizontal DashboardBottomLayout = 0
	DashboardLayoutVertical   DashboardBottomLayout = 1
	DashboardLayoutListOnly   DashboardBottomLayout = 2
	dashboardLayoutCount                            = 3
)

const (
	DefaultDashboardHorizontalSplitPct = 70
	DefaultDashboardVerticalSplitPct   = 60
	MinDashboardSplitPct               = 10
	MaxDashboardSplitPct               = 90
)

func clampSplitPct(pct int) int {
	if pct < MinDashboardSplitPct {
		return MinDashboardSplitPct
	}
	if pct > MaxDashboardSplitPct {
		return MaxDashboardSplitPct
	}
	return pct
}
