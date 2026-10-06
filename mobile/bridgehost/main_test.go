package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestServeAnswersEachRequestLine holds the host to one response line per
// request line, blank lines skipped, through the exported binding.
func TestServeAnswersEachRequestLine(t *testing.T) {
	input := strings.Join([]string{
		`{"bridge_version":1,"request_id":"a","op":"workspace.status","workspace":"w"}`,
		``,
		`not json`,
	}, "\n")
	var out bytes.Buffer
	if err := serve(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two response lines, got %q", out.String())
	}
	for i, want := range []string{"not_open", "invalid_request"} {
		var response struct {
			OK    bool `json:"ok"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &response); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if response.OK || response.Error.Code != want {
			t.Fatalf("line %d answered %s, want %s", i, lines[i], want)
		}
	}
}
