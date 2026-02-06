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
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return fmt.Errorf("task %q not found", args[0])
	}

	profile := cfg.GetProjectScoringProfile(project.ID)
	if profile == nil {
		return fmt.Errorf("no scoring profile assigned to project %q", project.Name)
	}

	totalScore := scoring.ComputeScore(task, profile)
	results := scoring.ScoreBreakdown(task, profile)

	fmt.Printf("Task #%d: %s\n", task.ShortID, task.Title)
	fmt.Printf("Profile: %s (%s)\n\n", profile.Name, profile.Formula)
	fmt.Println(scoring.FormatBreakdown(results, profile.Formula, totalScore))
	return nil
}
