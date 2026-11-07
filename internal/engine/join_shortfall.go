package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/syncexchange"
	"github.com/sglavoie/docket/internal/synclog"
)

// ErrJoinNotDelivered is a join whose import built nothing: the exchange held
// no history this device could start the workspace from yet.
var ErrJoinNotDelivered = errors.New("join: the exchange has not delivered a workspace this device can build")

// joinShortfall explains a join whose import left this device with no history
// at all, from what the import reported, or returns nil when it built one. A
// join is otherwise judged by the catalog it delivered, which says nothing
// useful when nothing was delivered: the reason is in what was held, what is
// still downloading, and which checkpoint the device was waiting for.
func joinShortfall(history *synclog.History, results []syncexchange.Result) error {
	if history == nil {
		return nil
	}
	if state, err := history.LoadState(); err != nil {
		return err
	} else if state != nil {
		return nil
	}
	var (
		upgrades   = map[string]bool{}
		held       int
		incomplete int
		failures   []string
		checkpoint string
	)
	for _, result := range results {
		isCheckpoint := strings.HasPrefix(result.Path, "checkpoints/")
		switch result.Outcome {
		case syncexchange.Held:
			held++
			if result.Upgrade != "" {
				upgrades[result.Upgrade] = true
			}
		case syncexchange.Incomplete:
			incomplete++
			if isCheckpoint && len(result.Missing) > 0 {
				checkpoint = fmt.Sprintf("checkpoint %s is still downloading (%d file(s) to go)", shortID(result.TxnID), len(result.Missing))
			}
		case syncexchange.Failed, syncexchange.Invalid, syncexchange.Foreign:
			failures = append(failures, fmt.Sprintf("%s: %s", result.Path, result.Reason))
		default:
			// Imported and known results leave nothing to explain.
		}
	}
	var parts []string
	if checkpoint != "" {
		parts = append(parts, checkpoint)
	}
	if held > 0 {
		reasons := make([]string, 0, len(upgrades))
		for upgrade := range upgrades {
			reasons = append(reasons, upgrade)
		}
		sort.Strings(reasons)
		line := fmt.Sprintf("%d item(s) are written at schemas this device does not read", held)
		if len(reasons) > 0 {
			line += " (" + strings.Join(reasons, "; ") + ")"
		}
		parts = append(parts, line)
		if checkpoint == "" {
			parts = append(parts, "a desktop running the current Docket publishes a checkpoint this device can start from; if it has exchanged since updating, the checkpoint may still be downloading, so try again shortly, otherwise run 'docket sync exchange' there first")
		}
	}
	if incomplete > 0 && checkpoint == "" {
		parts = append(parts, fmt.Sprintf("%d item(s) are still downloading", incomplete))
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		parts = append(parts, failures[0])
	}
	if len(parts) == 0 {
		parts = append(parts, "the exchange holds no history yet; enroll the workspace on a desktop first")
	}
	return fmt.Errorf("%w: %s", ErrJoinNotDelivered, strings.Join(parts, "; "))
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
