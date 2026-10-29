package models

import (
	"fmt"
	"testing"
	"time"
)

func TestQueryExactDueDateKeepsStoredCalendarDayAcrossZones(t *testing.T) {
	west := time.FixedZone("west", -10*3600)
	east := time.FixedZone("east", 14*3600)
	for _, zones := range []struct {
		stored, reader *time.Location
	}{{west, east}, {east, west}} {
		for _, day := range []int{19, 20, 21} {
			for _, hour := range []int{0, 9, 23} {
				t.Run(fmt.Sprintf("%s to %s/day %d/hour %d", zones.stored, zones.reader, day, hour), func(t *testing.T) {
					due := time.Date(2026, time.March, day, hour, 0, 0, 0, zones.stored)
					task := queryTestTask()
					task.DueDate = &due
					now := time.Date(2026, time.March, 20, 12, 0, 0, 0, zones.reader)
					clause := positiveClause(t, QueryFieldDue, QueryOperatorIs, "2026-03-20")
					query := Query{Clauses: []QueryClause{clause}}
					if got, want := query.Admits(task, queryTestProject(), now), day == 20; got != want {
						t.Errorf("exact due date admitted %s = %t, want %t", due, got, want)
					}
					query.Clauses[0].Polarity = QueryPolarityNegative
					if got, want := query.Admits(task, queryTestProject(), now), day != 20; got != want {
						t.Errorf("negative due date admitted %s = %t, want %t", due, got, want)
					}
				})
			}
		}
	}
}

func TestQueryExactDaysSurviveSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, loc)
	// Havana's March 8 starts at 01:00. Parsing that date as midnight
	// silently normalizes it onto March 7, which must not change the query.
	for _, field := range []QueryField{QueryFieldDue, QueryFieldDefer} {
		for _, day := range []int{7, 8, 9} {
			t.Run(fmt.Sprintf("%s/day %d", field, day), func(t *testing.T) {
				stored := time.Date(2026, time.March, day, 12, 0, 0, 0, loc)
				task := queryTestTask()
				if field == QueryFieldDue {
					task.DueDate = &stored
				} else {
					task.DeferredUntil = &stored
				}
				query := Query{Clauses: []QueryClause{positiveClause(t, field, QueryOperatorIs, "2026-03-08")}}
				if got, want := query.Admits(task, queryTestProject(), now), day == 8; got != want {
					t.Errorf("exact day admitted %s = %t, want %t", stored, got, want)
				}
			})
		}
	}
}

func TestQueryDeferralDayUsesReadersZone(t *testing.T) {
	stored := time.Date(2026, time.March, 20, 23, 0, 0, 0, time.FixedZone("west", -10*3600))
	now := time.Date(2026, time.March, 21, 12, 0, 0, 0, time.FixedZone("east", 14*3600))
	task := queryTestTask()
	task.DeferredUntil = &stored
	for _, day := range []int{20, 21} {
		query := Query{Clauses: []QueryClause{positiveClause(t, QueryFieldDefer, QueryOperatorIs, fmt.Sprintf("2026-03-%d", day))}}
		if got, want := query.Admits(task, queryTestProject(), now), day == 21; got != want {
			t.Errorf("wake day %d admitted %s = %t, want %t", day, stored, got, want)
		}
	}
}
