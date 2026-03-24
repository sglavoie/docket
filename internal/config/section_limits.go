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

// GetSectionLimit returns the configured limit for the given section name.
// Returns 0 if disabled. Section names match the outliner section names:
// "" (Active), "Next", "Inbox", "Standby", "Waiting", "Someday", "Review".
func (c *Config) GetSectionLimit(section string) int {
	switch section {
	case "":
		if c.SectionLimitActive == nil {
			return DefaultSectionLimitActive
		}
		return *c.SectionLimitActive
	case "Next":
		if c.SectionLimitNext == nil {
			return DefaultSectionLimitNext
		}
		return *c.SectionLimitNext
	case "Inbox":
		if c.SectionLimitInbox == nil {
			return DefaultSectionLimitInbox
		}
		return *c.SectionLimitInbox
	case "Standby":
		if c.SectionLimitStandby == nil {
			return DefaultSectionLimitStandby
		}
		return *c.SectionLimitStandby
	case "Waiting":
		if c.SectionLimitWaiting == nil {
			return DefaultSectionLimitWaiting
		}
		return *c.SectionLimitWaiting
	case "Someday":
		if c.SectionLimitSomeday == nil {
			return DefaultSectionLimitSomeday
		}
		return *c.SectionLimitSomeday
	case "Review":
		if c.SectionLimitReview == nil {
			return DefaultSectionLimitReview
		}
		return *c.SectionLimitReview
	default:
		return 0
	}
}
