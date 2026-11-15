package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestOpeningNowRetiresQuietFeedback(t *testing.T) {
	for _, kind := range []string{"success", "info", "warning", "error", "error severity", "refused"} {
		t.Run(kind, func(t *testing.T) {
			app := flashTestApp(t)
			project := app.crossOutliner.Population.Projects[0]
			state := models.StateDoing
			if kind == "refused" {
				state = models.StateBacklog
			}
			project.Tasks = []*models.Task{{ID: "now-task", Title: "Now task", WorkflowState: state, StateChangedAt: time.Now()}}
			app.crossOutliner.RebuildRows()
			app.setSuccessStatus("Task captured")
			switch kind {
			case "info":
				app.setInfoStatus("Information")
			case "warning":
				app.setWarningStatus("Warning")
			case "error":
				app.feedback.err = errors.New("storage failed")
			case "error severity":
				app.feedback.recordSeverity(flashError)
			}
			message, failure := app.feedback.message, app.feedback.err
			history := len(app.feedback.history)
			seq := app.feedback.armFlash(flashCelebrate)
			descriptor, ok := app.operations.Registry.Descriptor(operation.OperationCoreOpenNow)
			if !ok {
				t.Fatal("Now operation missing")
			}
			app.executeCoreInputOperation(operation.OperationInvocation{Descriptor: descriptor})
			if kind == "refused" {
				if app.crossOutliner.NowView.Active || app.feedback.message == "" || app.feedback.flashSeq != seq {
					t.Fatal("refusal cleared feedback or opened Now")
				}
				return
			}
			if !app.crossOutliner.NowView.Active {
				t.Fatal("Now did not open")
			}
			if kind == "success" || kind == "info" {
				if app.feedback.message != "" || app.feedback.err != nil || app.feedback.flash.active() {
					t.Fatal("quiet feedback survived")
				}
				if app.feedback.advanceFlash(seq) || app.feedback.flashSeq <= seq {
					t.Fatal("retired tick survived")
				}
			} else if app.feedback.message != message || app.feedback.err != failure || app.feedback.flashSeq != seq {
				t.Fatal("warning or error was changed")
			}
			if len(app.feedback.history) != history {
				t.Fatal("opening changed remembered feedback")
			}
		})
	}
}
