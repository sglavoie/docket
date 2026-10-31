package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// RebasePlanningRestore applies the recorded membership changes to current,
// retaining unrelated memberships and their order. Both horizons are checked
// together for each changed project-qualified identity. Reordering-only intent
// retains exact-image checks: before/after images cannot identify which of two
// reordered tasks the user moved. This function performs no filesystem I/O.
func RebasePlanningRestore(expectedBytes, desiredBytes, currentBytes []byte) ([]byte, bool) {
	expected, workspaceID, ok := decodePlanningRestore(expectedBytes)
	if !ok {
		return nil, false
	}
	desired, desiredID, ok := decodePlanningRestore(desiredBytes)
	if !ok || desiredID != workspaceID {
		return nil, false
	}
	current, currentID, ok := decodePlanningRestore(currentBytes)
	if !ok || currentID != workspaceID {
		return nil, false
	}
	expectedMembership := planningMemberships(expected)
	desiredMembership := planningMemberships(desired)
	currentMembership := planningMemberships(current)
	touched := make(map[ProjectTaskLocator]bool)
	for ref, horizon := range expectedMembership {
		if desiredMembership[ref] != horizon {
			touched[ref] = true
		}
	}
	for ref, horizon := range desiredMembership {
		if expectedMembership[ref] != horizon {
			touched[ref] = true
		}
	}
	for ref := range touched {
		if currentMembership[ref] != expectedMembership[ref] {
			return nil, false
		}
	}
	for _, pair := range [][2][]ProjectTaskLocator{{expected.Today, desired.Today}, {expected.Next, desired.Next}} {
		if !slices.Equal(untouchedPlanningRefs(pair[0], touched), untouchedPlanningRefs(pair[1], touched)) {
			return nil, false
		}
	}
	if !planningRestorePositionsMatch(expected.Today, current.Today, touched) ||
		!planningRestorePositionsMatch(expected.Next, current.Next, touched) {
		return nil, false
	}
	current.Today = restorePlanningRefs(desired.Today, current.Today, touched)
	current.Next = restorePlanningRefs(desired.Next, current.Next, touched)
	data, err := encodeDomainEnvelope(domainPlanning, workspaceID, current)
	return data, err == nil
}

// PlanningRestoreAdditions names only the memberships the restore would add,
// so the engine can validate those tasks against the proposed project images.
// Existing unrelated memberships need no project reads or reconciliation.
func PlanningRestoreAdditions(expectedBytes, desiredBytes []byte) (*PlanningState, string, error) {
	expected, workspaceID, ok := decodePlanningRestore(expectedBytes)
	if !ok {
		return nil, "", fmt.Errorf("config: invalid expected planning restore image")
	}
	desired, desiredID, ok := decodePlanningRestore(desiredBytes)
	if !ok || desiredID != workspaceID {
		return nil, "", fmt.Errorf("config: invalid desired planning restore image or workspace identity")
	}
	desired.Today = slices.DeleteFunc(desired.Today, func(ref ProjectTaskLocator) bool { return slices.Contains(expected.Today, ref) })
	desired.Next = slices.DeleteFunc(desired.Next, func(ref ProjectTaskLocator) bool { return slices.Contains(expected.Next, ref) })
	return desired, workspaceID, nil
}

// PlanningRestoreState decodes proposed planning bytes for read-only restore
// validation, with the same strict envelope and membership checks as rebasing.
func PlanningRestoreState(data []byte) (*PlanningState, error) {
	planning, _, ok := decodePlanningRestore(data)
	if !ok {
		return nil, fmt.Errorf("config: invalid proposed planning restore image")
	}
	return planning, nil
}

// Decode strictly so a restore never silently drops unknown or future fields.
func decodePlanningRestore(data []byte) (*PlanningState, string, bool) {
	var env domainEnvelope
	if !decodePlanningRestoreJSON(data, &env) || env.SchemaVersion != CurrentDomainSchemaVersion ||
		env.Kind != domainPlanning.name() || validateWorkspaceID(env.WorkspaceID) != nil {
		return nil, "", false
	}
	var planning *PlanningState
	if !decodePlanningRestoreJSON(env.Data, &planning) || planning == nil || planning.Validate() != nil {
		return nil, "", false
	}
	for _, ref := range planning.Today {
		if slices.Contains(planning.Next, ref) {
			return nil, "", false
		}
	}
	return planning, env.WorkspaceID, true
}

func decodePlanningRestoreJSON(data []byte, out any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out) == nil && requireJSONEOF(decoder) == nil
}

func planningMemberships(planning *PlanningState) map[ProjectTaskLocator]int {
	members := make(map[ProjectTaskLocator]int, len(planning.Today)+len(planning.Next))
	for _, ref := range planning.Today {
		members[ref] = 1
	}
	for _, ref := range planning.Next {
		members[ref] = 2
	}
	return members
}

func untouchedPlanningRefs(refs []ProjectTaskLocator, touched map[ProjectTaskLocator]bool) []ProjectTaskLocator {
	return slices.DeleteFunc(slices.Clone(refs), func(ref ProjectTaskLocator) bool { return touched[ref] })
}

// A changed task must still stand on the same side of every surviving original
// neighbor. Insertions and deletions of unrelated tasks do not move it, and
// unrelated tasks can reorder among themselves without costing the restore.
func planningRestorePositionsMatch(expected, current []ProjectTaskLocator, touched map[ProjectTaskLocator]bool) bool {
	positions := make(map[ProjectTaskLocator]int, len(current))
	for i, ref := range current {
		positions[ref] = i
	}
	for i, ref := range expected {
		if !touched[ref] {
			continue
		}
		position, exists := positions[ref]
		if !exists {
			return false
		}
		for j, other := range expected {
			if otherPosition, exists := positions[other]; exists && (j < i) != (otherPosition < position) {
				return false
			}
		}
	}
	return true
}

// Insert restored members before the next surviving desired member, or at the
// end. Walking backwards preserves the recorded order of multiple insertions
// while leaving the relative order of all current unrelated members intact.
func restorePlanningRefs(desired, current []ProjectTaskLocator, touched map[ProjectTaskLocator]bool) []ProjectTaskLocator {
	result := untouchedPlanningRefs(current, touched)
	insertAt := len(result)
	for i := len(desired) - 1; i >= 0; i-- {
		ref := desired[i]
		if touched[ref] {
			result = slices.Insert(result, insertAt, ref)
		} else if index := slices.Index(result, ref); index >= 0 {
			insertAt = index
		}
	}
	return result
}
