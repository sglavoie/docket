package config

// Default section limit values. 0 means disabled (no warning).
const (
	DefaultSectionLimitActive  = 5
	DefaultSectionLimitNext    = 5
	DefaultSectionLimitInbox   = 5
	DefaultSectionLimitStandby = 5
	DefaultSectionLimitWaiting = 3
	DefaultSectionLimitSomeday = 10
	DefaultSectionLimitReview  = 5
)

func sectionLimitValue(limit *int, defaultValue int) int {
	if limit == nil {
		return defaultValue
	}
	if *limit < 0 {
		return 0
	}
	return *limit
}

// GetSectionLimit returns the configured limit for the given section name.
// Returns 0 if disabled. Section names match the outliner section names:
// "" (Active), "Next", "Inbox", "Standby", "Waiting", "Someday", "Review".
func (c *Config) GetSectionLimit(section string) int {
	switch section {
	case "":
		return sectionLimitValue(c.SectionLimitActive, DefaultSectionLimitActive)
	case "Next":
		return sectionLimitValue(c.SectionLimitNext, DefaultSectionLimitNext)
	case "Inbox":
		return sectionLimitValue(c.SectionLimitInbox, DefaultSectionLimitInbox)
	case "Standby":
		return sectionLimitValue(c.SectionLimitStandby, DefaultSectionLimitStandby)
	case "Waiting":
		return sectionLimitValue(c.SectionLimitWaiting, DefaultSectionLimitWaiting)
	case "Someday":
		return sectionLimitValue(c.SectionLimitSomeday, DefaultSectionLimitSomeday)
	case "Review":
		return sectionLimitValue(c.SectionLimitReview, DefaultSectionLimitReview)
	default:
		return 0
	}
}
