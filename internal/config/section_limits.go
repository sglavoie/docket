package config

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
