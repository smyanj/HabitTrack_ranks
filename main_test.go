package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSeasonRolloverAndPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s, err := newStore(p)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.data.SeasonYear = 2025
	s.data.Settings.BaseScore = -352
	s.data.Entries = append(s.data.Entries, Entry{ID: 1, HabitID: 1, HabitName: "Workout", Points: 50})
	s.data.NextEntryID = 2
	if err = s.saveLocked(); err != nil {
		t.Fatal(err)
	}
	if err = s.rolloverLocked(time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if score := totalScore(s.data); score != 0 {
		t.Fatalf("new season score %d", score)
	}
	if len(s.data.Habits) != 5 {
		t.Fatal("habits lost")
	}
	if len(s.data.Archives) != 1 || s.data.Archives[0].Score != -302 {
		t.Fatalf("bad archive %+v", s.data.Archives)
	}
	s.mu.Unlock()
	reloaded, err := newStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.data.SeasonYear != 2026 || len(reloaded.data.Archives) != 1 {
		t.Fatal("season not persisted")
	}
}
func TestNoEarlyRollover(t *testing.T) {
	s := &Store{data: defaultState()}
	s.data.SeasonYear = 2026
	s.data.Settings.BaseScore = 85
	if err := s.rolloverLocked(time.Date(2026, 12, 31, 23, 59, 59, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if totalScore(s.data) != 85 {
		t.Fatal("premature reset")
	}
}
