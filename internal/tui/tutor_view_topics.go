package tui

// tutorTopics returns all tutor topics organized by category.
func tutorTopics() []TutorTopic {
	concepts := tutorConceptTopics()
	cliTopics := tutorCLITopics()
	tuiTopics := tutorTUITopics()

	topics := make([]TutorTopic, 0, len(concepts)+len(cliTopics)+len(tuiTopics))
	topics = append(topics, concepts...)
	topics = append(topics, cliTopics...)
	topics = append(topics, tuiTopics...)

	return topics
}
