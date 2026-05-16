package tui

// tutorTopics returns all tutor topics organized by category.
// "Getting Started" topics are prepended so they appear first in the All tab
// and as the first category in the tab bar.
func tutorTopics() []TutorTopic {
	gs := tutorGettingStartedTopics()
	concepts := tutorConceptTopics()
	cliTopics := tutorCLITopics()
	tuiTopics := tutorTUITopics()

	topics := make([]TutorTopic, 0, len(gs)+len(concepts)+len(cliTopics)+len(tuiTopics))
	topics = append(topics, gs...)
	topics = append(topics, concepts...)
	topics = append(topics, cliTopics...)
	topics = append(topics, tuiTopics...)

	return topics
}
