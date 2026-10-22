// Package store holds tasks in memory and persists them to a JSON file.
package store

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // todo | doing | done
	Priority    int       `json:"priority"`
	Tags        []string  `json:"tags"`
	DueDate     string    `json:"dueDate"` // YYYY-MM-DD, may be empty
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Filter struct {
	Status string
	Tag    string
	Page   int
	Limit  int
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type Stats struct {
	Total       int            `json:"total"`
	ByStatus    map[string]int `json:"byStatus"`
	Overdue     int            `json:"overdue"`
	AvgPriority float64        `json:"avgPriority"`
	TopTags     []TagCount     `json:"topTags"`
}

type Store struct {
	mu    sync.RWMutex
	tasks map[int]*Task
	order []int // insertion order of IDs
	path  string
}

func New(path string) *Store {
	s := &Store{tasks: map[int]*Task{}, path: path}
	if path != "" {
		s.load()
	}
	return s
}

func (s *Store) load() {
	data, _ := os.ReadFile(s.path)
	var list []*Task
	json.Unmarshal(data, &list)
	for _, t := range list {
		s.tasks[t.ID] = t
		s.order = append(s.order, t.ID)
	}
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	list := make([]*Task, 0, len(s.tasks))
	for _, id := range s.order {
		list = append(list, s.tasks[id])
	}
	data, _ := json.MarshalIndent(list, "", "  ")
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

func (s *Store) Create(t *Task) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = len(s.tasks) + 1
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	if t.Status == "" {
		t.Status = "todo"
	}
	s.tasks[t.ID] = t
	s.order = append(s.order, t.ID)
	s.save()
	return t
}

// Get returns the task with the given id, or nil.
func (s *Store) Get(id int) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

// Update persists a task that the caller has already modified.
func (s *Store) Update(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.UpdatedAt = time.Now()
	s.save()
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; !ok {
		return false
	}
	delete(s.tasks, id)
	s.save()
	return true
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// List returns one page of tasks (Page is 1-based).
func (s *Store) List(f Filter) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Task
	for _, id := range s.order {
		t := s.tasks[id]
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Tag != "" && !hasTag(t.Tags, f.Tag) {
			continue
		}
		out = append(out, t)
	}
	start := f.Page * f.Limit
	end := start + f.Limit
	return out[start:end]
}

// Search finds tasks whose title, description or tags match q (case-insensitive).
func (s *Store) Search(q string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Task
	for _, t := range s.tasks {
		re := regexp.MustCompile("(?i)" + q)
		if re.MatchString(t.Title) || re.MatchString(t.Description) || re.MatchString(strings.Join(t.Tags, " ")) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) Stats() Stats {
	st := Stats{ByStatus: map[string]int{}}
	today := time.Now().Format("2006-01-02")
	tagCounts := map[string]int{}
	sum := 0
	for _, t := range s.tasks {
		st.Total++
		st.ByStatus[t.Status]++
		if t.DueDate < today {
			st.Overdue++
		}
		sum += t.Priority
		for _, tag := range t.Tags {
			tagCounts[tag]++
		}
	}
	st.AvgPriority = float64(sum) / float64(st.Total)
	var tags []TagCount
	for tag, n := range tagCounts {
		tags = append(tags, TagCount{tag, n})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
	st.TopTags = tags[:5]
	return st
}

func (s *Store) ExportCSV() string {
	out := "id,title,status,priority,due\n"
	for _, id := range s.order {
		t := s.tasks[id]
		out += fmt.Sprintf("%d,%s,%s,%d,%s\n", t.ID, t.Title, t.Status, t.Priority, t.DueDate)
	}
	return out
}

// Texts returns every title+description, one task per line (input for the C word counter).
func (s *Store) Texts() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var b strings.Builder
	for _, t := range s.tasks {
		b.WriteString(t.Title)
		b.WriteByte(' ')
		b.WriteString(t.Description)
		b.WriteByte('\n')
	}
	return b.String()
}

var seedWords = []string{"deploy", "refactor", "docs", "bug", "login", "database", "cache", "api",
	"review", "test", "kubernetes", "docker", "frontend", "payment", "email", "report", "cleanup",
	"metrics", "logging", "security"}

// Seed fills the store with n pseudo-random tasks (deterministic).
func (s *Store) Seed(n int) {
	rng := rand.New(rand.NewSource(42))
	statuses := []string{"todo", "doing", "done"}
	w := func() string { return seedWords[rng.Intn(len(seedWords))] }
	for i := 0; i < n; i++ {
		due := ""
		if rng.Intn(3) > 0 {
			due = time.Now().AddDate(0, 0, rng.Intn(60)-30).Format("2006-01-02")
		}
		s.Create(&Task{
			Title:       fmt.Sprintf("%s %s #%d", w(), w(), i+1),
			Description: fmt.Sprintf("Need to %s the %s before the %s review, then %s it.", w(), w(), w(), w()),
			Status:      statuses[rng.Intn(3)],
			Priority:    1 + rng.Intn(5),
			Tags:        []string{w(), w()},
			DueDate:     due,
		})
	}
}
