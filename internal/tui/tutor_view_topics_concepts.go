package tui

// tutorConceptTopics returns tutor topics for product concepts.
func tutorConceptTopics() []TutorTopic {
	groups := [][]TutorTopic{
		tutorConceptTopicsCore(),
		tutorConceptTopicsWorkflow(),
		tutorConceptTopicsPlanning(),
		tutorConceptTopicsCustomization(),
		tutorConceptTopicsPipeline(),
	}

	total := 0
	for _, group := range groups {
		total += len(group)
	}

	topics := make([]TutorTopic, 0, total)
	for _, group := range groups {
		topics = append(topics, group...)
	}

	return topics
}
