package cmd

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
)

func configTaskLocatorsForProject(projectID string, taskIDs map[string]bool) []config.ProjectTaskLocator {
	locators := make([]config.ProjectTaskLocator, 0, len(taskIDs))
	if projectID == "" {
		return locators
	}
	for taskID := range taskIDs {
		if taskID != "" {
			locators = append(locators, config.ProjectTaskLocator{ProjectID: projectID, TaskID: taskID})
		}
	}
	return locators
}

func configTaskLocatorsFromApp(locators []app.TaskLocator) []config.ProjectTaskLocator {
	result := make([]config.ProjectTaskLocator, 0, len(locators))
	for _, locator := range locators {
		if locator.ProjectID != "" && locator.TaskID != "" {
			result = append(result, config.ProjectTaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
		}
	}
	return result
}

func appTaskLocatorsFromConfig(locators []config.ProjectTaskLocator) []app.TaskLocator {
	result := make([]app.TaskLocator, 0, len(locators))
	for _, locator := range locators {
		if locator.ProjectID != "" && locator.TaskID != "" {
			result = append(result, app.TaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
		}
	}
	return result
}
