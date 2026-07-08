package config

// DefaultResumeCardLimit is the default number of parked tasks shown in the
// dashboard resume card.
const DefaultResumeCardLimit = 5

// ResumeCardLimitUpperBound matches the [1, 20] range enforced by the settings
// UI validator and the loader's clampIntPtrRange.
const ResumeCardLimitUpperBound = 20

// GetResumeCardLimit returns the maximum number of parked tasks shown in the
// dashboard resume card. Returns 5 if unset or set below the minimum valid value
// of 1. Values above ResumeCardLimitUpperBound are clamped to the upper bound.
func (c *Config) GetResumeCardLimit() int {
	if c.ResumeCardLimit == nil || *c.ResumeCardLimit < 1 {
		return DefaultResumeCardLimit
	}
	if *c.ResumeCardLimit > ResumeCardLimitUpperBound {
		return ResumeCardLimitUpperBound
	}
	return *c.ResumeCardLimit
}
