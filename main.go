package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type Habit struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Points   int       `json:"points"`
	Category string    `json:"category"`
	Icon     string    `json:"icon"`
	Created  time.Time `json:"created"`
}

type Entry struct {
	ID        int       `json:"id"`
	HabitID   int       `json:"habitId"`
	HabitName string    `json:"habitName"`
	Points    int       `json:"points"`
	Note      string    `json:"note"`
	Created   time.Time `json:"created"`
}

type Settings struct {
	PlayerName string `json:"playerName"`
	BaseScore  int    `json:"baseScore"`
}

type SeasonArchive struct {
	Year      int     `json:"year"`
	Score     int     `json:"score"`
	Rank      string  `json:"rank"`
	Entries   []Entry `json:"entries"`
	BaseScore int     `json:"baseScore"`
}

type State struct {
	SeasonYear  int             `json:"seasonYear"`
	Archives    []SeasonArchive `json:"archives"`
	Habits      []Habit         `json:"habits"`
	Entries     []Entry         `json:"entries"`
	Settings    Settings        `json:"settings"`
	NextHabitID int             `json:"nextHabitId"`
	NextEntryID int             `json:"nextEntryId"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data State
}

type Rank struct {
	Name     string `json:"name"`
	Required int    `json:"required"`
}

var ranks = []Rank{
	{"Bronze I", 0}, {"Bronze II", 100}, {"Bronze III", 250}, {"Bronze IV", 500},
	{"Silver I", 750}, {"Silver II", 1000}, {"Silver III", 1500}, {"Silver IV", 2000},
	{"Gold I", 2500}, {"Gold II", 3000}, {"Gold III", 4000}, {"Gold IV", 5000},
	{"Platinum I", 6000}, {"Platinum II", 7000}, {"Platinum III", 8500}, {"Platinum IV", 10000},
	{"Diamond I", 12000}, {"Diamond II", 14000}, {"Diamond III", 16000}, {"Diamond IV", 18000},
	{"Master", 20000},
}

func defaultState() State {
	return State{
		SeasonYear: time.Now().Year(),
		Archives:   []SeasonArchive{},
		Habits: []Habit{
			{ID: 1, Name: "Workout", Points: 50, Category: "Health", Icon: "🏋️", Created: time.Now()},
			{ID: 2, Name: "1 hour of lectures", Points: 30, Category: "School", Icon: "📚", Created: time.Now()},
			{ID: 3, Name: "LeetCode", Points: 20, Category: "Career", Icon: "💻", Created: time.Now()},
			{ID: 4, Name: "Checked Instagram", Points: -100, Category: "Distraction", Icon: "📵", Created: time.Now()},
			{ID: 5, Name: "1 hour free coding", Points: 150, Category: "Career", Icon: "🧠", Created: time.Now()},
		},
		Entries:     []Entry{},
		Settings:    Settings{PlayerName: "Player", BaseScore: 0},
		NextHabitID: 6,
		NextEntryID: 1,
	}
}

func newStore(path string) (*Store, error) {
	s := &Store{path: path, data: defaultState()}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("read state: %w", err)
		}
		if s.data.NextHabitID == 0 {
			s.data.NextHabitID = maxHabitID(s.data.Habits) + 1
		}
		if s.data.NextEntryID == 0 {
			s.data.NextEntryID = maxEntryID(s.data.Entries) + 1
		}
		if s.data.SeasonYear == 0 { // migrate data from v1 without losing the user's current XP
			s.data.SeasonYear = time.Now().Year()
		}
		if err := s.rolloverLocked(time.Now()); err != nil {
			return nil, err
		}
		return s, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func maxHabitID(items []Habit) int {
	m := 0
	for _, h := range items {
		if h.ID > m {
			m = h.ID
		}
	}
	return m
}

func maxEntryID(items []Entry) int {
	m := 0
	for _, e := range items {
		if e.ID > m {
			m = e.ID
		}
	}
	return m
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Seasons end on December 31 at 11:59:59 PM local server time.
// At midnight on January 1, archive the season and reset XP, not habits.
func (s *Store) rolloverLocked(now time.Time) error {
	if s.data.SeasonYear == 0 {
		s.data.SeasonYear = now.Year()
	}
	if now.Year() <= s.data.SeasonYear {
		return nil
	}
	score := totalScore(s.data)
	rank, _ := rankFor(score)
	archivedEntries := append([]Entry{}, s.data.Entries...)
	s.data.Archives = append(s.data.Archives, SeasonArchive{
		Year: s.data.SeasonYear, Score: score, Rank: rank.Name,
		Entries: archivedEntries, BaseScore: s.data.Settings.BaseScore,
	})
	s.data.Entries = []Entry{}
	s.data.Settings.BaseScore = 0
	s.data.SeasonYear = now.Year()
	return s.saveLocked()
}

func (s *Store) refreshSeason() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rolloverLocked(time.Now())
}

func (s *Store) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

func totalScore(st State) int {
	score := st.Settings.BaseScore
	for _, e := range st.Entries {
		score += e.Points
	}
	return score
}

func rankFor(score int) (Rank, *Rank) {
	current := ranks[0]
	for i := range ranks {
		if score >= ranks[i].Required {
			current = ranks[i]
		} else {
			return current, &ranks[i]
		}
	}
	return current, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func main() {
	store, err := newStore(filepath.Join("data", "state.json"))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		st := store.snapshot()
		score := totalScore(st)
		current, next := rankFor(score)
		writeJSON(w, 200, map[string]any{
			"habits": st.Habits, "entries": st.Entries, "settings": st.Settings,
			"score": score, "rank": current, "nextRank": next, "ranks": ranks,
			"seasonYear": st.SeasonYear, "seasonEnd": time.Date(st.SeasonYear+1, 1, 1, 0, 0, 0, 0, time.Local).Format(time.RFC3339),
			"archives": st.Archives,
		})
	})

	mux.HandleFunc("POST /api/habits", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name     string `json:"name"`
			Points   int    `json:"points"`
			Category string `json:"category"`
			Icon     string `json:"icon"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid request"})
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "Habit name is required"})
			return
		}
		if in.Points == 0 {
			writeJSON(w, 400, map[string]string{"error": "Points cannot be 0"})
			return
		}
		if in.Category == "" {
			in.Category = "General"
		}
		if in.Icon == "" {
			in.Icon = "⚡"
		}

		store.mu.Lock()
		h := Habit{ID: store.data.NextHabitID, Name: in.Name, Points: in.Points, Category: in.Category, Icon: in.Icon, Created: time.Now()}
		store.data.NextHabitID++
		store.data.Habits = append(store.data.Habits, h)
		err := store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save habit"})
			return
		}
		writeJSON(w, 201, h)
	})

	mux.HandleFunc("PUT /api/habits/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid habit id"})
			return
		}
		var in struct {
			Name     string `json:"name"`
			Points   int    `json:"points"`
			Category string `json:"category"`
			Icon     string `json:"icon"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid request"})
			return
		}
		if strings.TrimSpace(in.Name) == "" || in.Points == 0 {
			writeJSON(w, 400, map[string]string{"error": "Name and non-zero points are required"})
			return
		}

		store.mu.Lock()
		found := false
		for i := range store.data.Habits {
			if store.data.Habits[i].ID == id {
				store.data.Habits[i].Name = strings.TrimSpace(in.Name)
				store.data.Habits[i].Points = in.Points
				store.data.Habits[i].Category = in.Category
				store.data.Habits[i].Icon = in.Icon
				found = true
				break
			}
		}
		if !found {
			store.mu.Unlock()
			writeJSON(w, 404, map[string]string{"error": "Habit not found"})
			return
		}
		err = store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save habit"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("DELETE /api/habits/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid habit id"})
			return
		}
		store.mu.Lock()
		idx := -1
		for i, h := range store.data.Habits {
			if h.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			store.mu.Unlock()
			writeJSON(w, 404, map[string]string{"error": "Habit not found"})
			return
		}
		store.data.Habits = append(store.data.Habits[:idx], store.data.Habits[idx+1:]...)
		err = store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			HabitID int    `json:"habitId"`
			Note    string `json:"note"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid request"})
			return
		}
		store.mu.Lock()
		var habit *Habit
		for i := range store.data.Habits {
			if store.data.Habits[i].ID == in.HabitID {
				habit = &store.data.Habits[i]
				break
			}
		}
		if habit == nil {
			store.mu.Unlock()
			writeJSON(w, 404, map[string]string{"error": "Habit not found"})
			return
		}
		e := Entry{ID: store.data.NextEntryID, HabitID: habit.ID, HabitName: habit.Name, Points: habit.Points, Note: strings.TrimSpace(in.Note), Created: time.Now()}
		store.data.NextEntryID++
		store.data.Entries = append(store.data.Entries, e)
		err := store.saveLocked()
		score := totalScore(store.data)
		current, next := rankFor(score)
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not log habit"})
			return
		}
		writeJSON(w, 201, map[string]any{"entry": e, "score": score, "rank": current, "nextRank": next})
	})

	mux.HandleFunc("DELETE /api/entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid entry id"})
			return
		}
		store.mu.Lock()
		idx := -1
		for i, e := range store.data.Entries {
			if e.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			store.mu.Unlock()
			writeJSON(w, 404, map[string]string{"error": "Entry not found"})
			return
		}
		store.data.Entries = append(store.data.Entries[:idx], store.data.Entries[idx+1:]...)
		err = store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var in Settings
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "Invalid request"})
			return
		}
		if strings.TrimSpace(in.PlayerName) == "" {
			in.PlayerName = "Player"
		}
		store.mu.Lock()
		store.data.Settings = in
		err := store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save settings"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		store.mu.Lock()
		score := totalScore(store.data)
		rank, _ := rankFor(score)
		store.data.Archives = append(store.data.Archives, SeasonArchive{
			Year: store.data.SeasonYear, Score: score, Rank: rank.Name,
			Entries: append([]Entry{}, store.data.Entries...), BaseScore: store.data.Settings.BaseScore,
		})
		store.data.Entries = []Entry{}
		store.data.Settings.BaseScore = 0
		err := store.saveLocked()
		store.mu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not reset"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Habit Tracker running at http://localhost:%s", port)
	log.Printf("Progress is saved locally to %s", store.path)
	if err := http.ListenAndServe(addr, logMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if err := store.refreshSeason(); err != nil {
				writeJSON(w, 500, map[string]string{"error": "Could not process season rollover"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	}))); err != nil {
		log.Fatal(err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
