package control

import (
	"encoding/json"
	"testing"
)

func TestPublishCachesEditingSeparatelyFromPulse(t *testing.T) {
	client := &conn{out: make(chan []byte, 4)}
	server := &Server{conns: map[*conn]struct{}{client: {}}}
	server.Publish(Event{Event: EventPulse, Pulse: &Pulse{Inbox: 2}})
	server.Publish(Event{Event: EventEditing, Editing: &Editing{Undo: "Typing", Search: true}})
	var event Event
	if err := json.Unmarshal(server.lastEditing, &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != EventEditing || event.Editing == nil || *event.Editing != (Editing{Undo: "Typing", Search: true}) {
		t.Fatalf("editing: %+v", event)
	}
	if err := json.Unmarshal(server.lastPulse, &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != EventPulse || event.Pulse.Inbox != 2 {
		t.Fatalf("pulse: %+v", event)
	}
	<-client.out
	var wire map[string]any
	if err := json.Unmarshal(<-client.out, &wire); err != nil {
		t.Fatal(err)
	}
	editing := wire["editing"].(map[string]any)
	if editing["redo"] != "" {
		t.Fatal("native decoder needs an explicit empty redo label")
	}
}
