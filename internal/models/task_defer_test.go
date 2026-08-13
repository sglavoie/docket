package models

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func TestIsDeferred_Nil(t *testing.T) {
	task := &Task{}
	if task.IsDeferred() {
		t.Error("Task with nil DeferredUntil should not be deferred")
	}
}

func TestIsDeferred_Future(t *testing.T) {
	future := time.Now().AddDate(0, 0, 7)
	task := &Task{DeferredUntil: &future}
	if !task.IsDeferred() {
		t.Error("Task with future DeferredUntil should be deferred")
	}
}

func TestIsDeferred_Today(t *testing.T) {
	now := time.Now()
	today := util.StartOfDay(now)
	task := &Task{DeferredUntil: &today}
	if task.IsDeferred() {
		t.Error("Task deferred until today should NOT be deferred (it should reappear today)")
	}
}

func TestIsDeferred_Past(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3)
	task := &Task{DeferredUntil: &past}
	if task.IsDeferred() {
		t.Error("Task with past DeferredUntil should not be deferred")
	}
}

func TestClearDefer(t *testing.T) {
	future := time.Now().AddDate(0, 0, 7)
	task := &Task{DeferredUntil: &future}
	task.ClearDefer()
	if task.DeferredUntil != nil {
		t.Error("ClearDefer should set DeferredUntil to nil")
	}
}
