package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/scoring"
)

var scoreShowCmd = &cobra.Command{
	Use:   "show <task-ref>",
	Short: "Show score breakdown for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runScoreShow,
}

func init() {
	scoreCmd.AddCommand(scoreShowCmd)
}

func runScoreShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("requires exactly 1 arg: task-ref")
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	profile := cfg.GetProjectScoringProfile(project.ID)
	if profile == nil {
		return fmt.Errorf("no scoring profile assigned to project %q", project.Name)
	}

	totalScore := scoring.ComputeScore(task, profile)
	normalizedScore := scoring.ScorePercent(totalScore, profile)
	results := scoring.ScoreBreakdown(task, profile)

	fmt.Printf("Task #%d: %s\n", task.ShortID, task.Title)
	fmt.Printf("Profile: %s (%s)\n\n", profile.Name, profile.Formula)
	fmt.Println(scoring.FormatBreakdown(results, profile.Formula, totalScore, normalizedScore))
	return nil
}
