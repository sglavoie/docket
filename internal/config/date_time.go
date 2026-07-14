package config

func (s *Settings) GetWeekEndDay() string {
	switch s.WeekEndDay {
	case "fri", "sat", "sun":
		return s.WeekEndDay
	default:
		return "sun"
	}
}
