package tui

import (
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
)

// settingsFieldMeta describes a single focusable field in the settings view.
type settingsFieldMeta struct {
	FocusIndex int
	Tab        int
	SubTab     int
	TabLabel   string // Path label for search results, e.g. "General > Recent Activity"
	Label      string // Field label shown in search results
	Keywords   string // Extra terms for search matching
	Default    string // Default value as display string (empty = skip default hint)
}

// settingsFields is the registry of all focusable settings fields.
// Populated by settings_state.go's init() after tabFieldStart is computed.
var settingsFields []settingsFieldMeta

//nolint:gocyclo // intentionally enumerates all settings fields in one place
func buildSettingsFieldRegistry() []settingsFieldMeta {
	var fields []settingsFieldMeta

	add := func(focusIdx, tab, subTab int, tabLabel, label, keywords, def string) {
		fields = append(fields, settingsFieldMeta{
			FocusIndex: focusIdx,
			Tab:        tab,
			SubTab:     subTab,
			TabLabel:   tabLabel,
			Label:      label,
			Keywords:   keywords,
			Default:    def,
		})
	}

	// Tab 0: General (4 fields)
	base := tabFieldStart[tabGeneral]
	add(base+0, tabGeneral, 0, "General", "Editor", "editor command open tasks", "code")
	add(base+1, tabGeneral, 0, "General", "External editor", "external editor ctrl o reference preview gui", "code")
	add(base+2, tabGeneral, 0, "General", "Default Context", "context project personal", "personal")
	add(base+3, tabGeneral, 0, "General", "Undo History Depth", "undo history depth", strconv.Itoa(config.DefaultUndoHistoryDepth))

	// Tab 1: Palette (18 fields)
	base = tabFieldStart[tabPalette]
	add(base+0, tabPalette, 0, "Appearance > Palette Colors", "Theme Toggle", "dark light theme palette toggle", "Dark")
	add(base+1, tabPalette, 0, "Appearance > Palette Colors", "Normal Text", "color palette normal text foreground", "")
	add(base+2, tabPalette, 0, "Appearance > Palette Colors", "Muted", "color muted palette dim", "")
	add(base+3, tabPalette, 0, "Appearance > Palette Colors", "Primary", "color primary palette accent", "")
	add(base+4, tabPalette, 0, "Appearance > Palette Colors", "Secondary", "color secondary palette", "")
	add(base+5, tabPalette, 0, "Appearance > Palette Colors", "Accent", "color accent palette highlight", "")
	add(base+6, tabPalette, 0, "Appearance > Palette Colors", "Success", "color success palette green", "")
	add(base+7, tabPalette, 0, "Appearance > Palette Colors", "Warning", "color warning palette yellow orange", "")
	add(base+8, tabPalette, 0, "Appearance > Palette Colors", "Error", "color error palette red", "")
	add(base+9, tabPalette, 0, "Appearance > Palette Colors", "Border", "color border palette outline", "")
	add(base+10, tabPalette, 1, "Appearance > BG & Cursor", "BG Color", "background color palette", "")
	add(base+11, tabPalette, 1, "Appearance > BG & Cursor", "Cursor Color", "cursor color palette background", "")
	add(base+12, tabPalette, 1, "Appearance > BG & Cursor", "Cursor Text Color", "cursor text color palette foreground", "")
	add(base+13, tabPalette, 2, "Appearance > Priorities", "Priority 1 Title", "priority p1 color title urgent", "")
	add(base+14, tabPalette, 2, "Appearance > Priorities", "Priority 2 Title", "priority p2 color title high", "")
	add(base+15, tabPalette, 2, "Appearance > Priorities", "Priority 3 Title", "priority p3 color title normal", "")
	add(base+16, tabPalette, 2, "Appearance > Priorities", "Priority 4 Title", "priority p4 color title low", "")
	add(base+17, tabPalette, 2, "Appearance > Priorities", "Priority 5 Title", "priority p5 color title someday", "")

	// Tab 2: Review (5 fields)
	base = tabFieldStart[tabReview]
	add(base+0, tabReview, 0, "Review", "Due Soon Days", "due soon review days threshold", "7")
	add(base+1, tabReview, 0, "Review", "Future Days", "future review days threshold", "30")
	add(base+2, tabReview, 0, "Review", "Old Days", "old stale review days threshold updated", "14")
	add(base+3, tabReview, 0, "Review", "Completed Hours", "completed recent review hours window", "24")
	add(base+4, tabReview, 0, "Review", "Stale Badge Days", "stale badge days inline task", "0")

	// Tab 3: durable Behavior fields.
	// subTab 0: Recent Activity
	base = subTabFieldStart(tabBehavior, 0)
	add(base+0, tabBehavior, 0, "General > Recent Activity", "Task Limit", "recent activity limit count max", strconv.Itoa(20))
	add(base+1, tabBehavior, 0, "General > Recent Activity", "Show Completed", "recent activity completed show include", "No")
	// subTab 1: Linked Tasks
	base = subTabFieldStart(tabBehavior, 1)
	add(base+0, tabBehavior, 1, "General > Linked Tasks", "Jump After Creation", "linked task jump create new", "No")
	// subTab 2: Pinned Tasks
	base = subTabFieldStart(tabBehavior, 2)
	add(base+0, tabBehavior, 2, "General > Pinned Tasks", "Max Pinned", "pinned max tasks bar items", strconv.Itoa(config.DefaultMaxPinnedBarItems))
	// subTab 3: Recent Tasks
	base = subTabFieldStart(tabBehavior, 3)
	add(base+0, tabBehavior, 3, "General > Recent Tasks", "Max Recent Visits", "recent tasks max visits quick switch", strconv.Itoa(config.DefaultMaxRecentTaskVisits))
	// subTab 5: Section Limits
	base = subTabFieldStart(tabBehavior, 4)
	add(base+0, tabBehavior, 4, "General > Section Limits", "Active Limit", "active section limit gtd wip", strconv.Itoa(config.DefaultSectionLimitActive))
	add(base+1, tabBehavior, 4, "General > Section Limits", "Next Limit", "next section limit", strconv.Itoa(config.DefaultSectionLimitNext))
	add(base+2, tabBehavior, 4, "General > Section Limits", "Inbox Limit", "inbox section limit", strconv.Itoa(config.DefaultSectionLimitInbox))
	add(base+3, tabBehavior, 4, "General > Section Limits", "Standby Limit", "standby section limit", strconv.Itoa(config.DefaultSectionLimitStandby))
	add(base+4, tabBehavior, 4, "General > Section Limits", "Waiting Limit", "waiting section limit blocked", strconv.Itoa(config.DefaultSectionLimitWaiting))
	add(base+5, tabBehavior, 4, "General > Section Limits", "Someday Limit", "someday section limit maybe", strconv.Itoa(config.DefaultSectionLimitSomeday))
	add(base+6, tabBehavior, 4, "General > Section Limits", "Review Limit", "review section limit", strconv.Itoa(config.DefaultSectionLimitReview))
	// subTab 5: Startup
	base = subTabFieldStart(tabBehavior, 5)
	add(base+0, tabBehavior, 5, "General > Startup", "Start View", "startup start destination home planner tasks project review manage", "Planner")
	// subTab 6: Home.
	base = subTabFieldStart(tabBehavior, 6)
	add(base+0, tabBehavior, 6, "General > Home", "Parked Task Limit", "home attention resume parked tasks limit", strconv.Itoa(config.DefaultResumeCardLimit))
	add(base+1, tabBehavior, 6, "General > Home", "Projects Height %", "home projects attention height split percentage layout stacked", strconv.Itoa(config.DefaultHomeProjectsHeightPct))

	// Tab 4: Attachments (2 fields)
	base = tabFieldStart[tabAttachments]
	add(base+0, tabAttachments, 0, "General > Attachments", "Attachment Editor", "attachment editor file open default", "")
	add(base+1, tabAttachments, 0, "General > Attachments", "Code Editor", "code editor reference open", "code")

	// Tab 7: Notifications (15 fields across 3 sub-tabs)
	// subTab 0: Reviews (4 fields)
	base = subTabFieldStart(tabNotifications, 0)
	add(base+0, tabNotifications, 0, "Notifications > Reviews", "Daily Review Alert", "daily review notification alert enabled", "Enabled")
	add(base+1, tabNotifications, 0, "Notifications > Reviews", "Daily Review Hour", "daily review hour notification time", strconv.Itoa(config.DefaultAlertDailyReviewHour))
	add(base+2, tabNotifications, 0, "Notifications > Reviews", "Weekly Review Alert", "weekly review notification alert enabled", "Enabled")
	add(base+3, tabNotifications, 0, "Notifications > Reviews", "Weekly Review Day", "weekly review day notification monday", "Monday")
	// subTab 1: Tasks (9 fields)
	base = subTabFieldStart(tabNotifications, 1)
	add(base+0, tabNotifications, 1, "Notifications > Tasks", "Overdue Alert", "overdue task notification alert", "Enabled")
	add(base+1, tabNotifications, 1, "Notifications > Tasks", "Due Soon Alert", "due soon task notification", "Enabled")
	add(base+2, tabNotifications, 1, "Notifications > Tasks", "Due Soon Days", "due soon days notification threshold", strconv.Itoa(config.DefaultAlertDueSoonDays))
	add(base+3, tabNotifications, 1, "Notifications > Tasks", "Stale Alert", "stale task notification", "Enabled")
	add(base+4, tabNotifications, 1, "Notifications > Tasks", "Stale Days", "stale days notification threshold", strconv.Itoa(config.DefaultAlertStaleDays))
	add(base+5, tabNotifications, 1, "Notifications > Tasks", "Inbox Alert", "inbox notification backlog threshold", "Enabled")
	add(base+6, tabNotifications, 1, "Notifications > Tasks", "Inbox Threshold", "inbox threshold count notification", strconv.Itoa(config.DefaultAlertInboxThreshold))
	add(base+7, tabNotifications, 1, "Notifications > Tasks", "Blocked Alert", "blocked task notification days", "Enabled")
	add(base+8, tabNotifications, 1, "Notifications > Tasks", "Blocked Days", "blocked days notification threshold", strconv.Itoa(config.DefaultAlertBlockedDays))
	// subTab 2: Projects (2 fields)
	base = subTabFieldStart(tabNotifications, 2)
	add(base+0, tabNotifications, 2, "Notifications > Projects", "Project Health Alert", "project health notification stale", "Enabled")
	add(base+1, tabNotifications, 2, "Notifications > Projects", "Project Stale Days", "project stale days notification threshold", strconv.Itoa(config.DefaultAlertProjectStaleDays))

	// Tab 8: Appearance (5 fields)
	base = tabFieldStart[tabAppearance]
	add(base+0, tabAppearance, 0, "Appearance", "Zebra Striping", "zebra striping rows alternating background", "Disabled")
	add(base+1, tabAppearance, 0, "Appearance", "Week-End Day", "week end day friday saturday sunday eow", "Sunday")
	add(base+2, tabAppearance, 0, "Appearance", "Time Format", "time format 12 24 hour clock", "24-hour (17:00)")
	add(base+3, tabAppearance, 0, "Appearance", "Auto Theme", "auto theme macos system appearance dark light", "Disabled")
	add(base+4, tabAppearance, 0, "Appearance", "Use Emoji", "emoji unicode icons symbols dashboard palette", "Disabled")

	return fields
}

// searchSettingsFields returns fields matching the query (case-insensitive substring).
func searchSettingsFields(query string) []settingsFieldMeta {
	if query == "" {
		return settingsFields
	}
	q := strings.ToLower(query)
	var results []settingsFieldMeta
	for _, f := range settingsFields {
		haystack := strings.ToLower(f.TabLabel + " " + f.Label + " " + f.Keywords)
		if strings.Contains(haystack, q) {
			results = append(results, f)
		}
	}
	return results
}

// settingsFieldByIndex returns the metadata for a specific focusIndex.
func settingsFieldByIndex(focusIndex int) (settingsFieldMeta, bool) {
	for _, f := range settingsFields {
		if f.FocusIndex == focusIndex {
			return f, true
		}
	}
	return settingsFieldMeta{}, false
}
