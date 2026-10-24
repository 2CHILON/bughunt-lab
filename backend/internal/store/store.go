// Package store holds tasks in memory and persists them to a JSON file.
package store

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
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
	nextID int
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
		if t == nil {
			continue // tolerate" null" entries written by older versions
		}
		s.tasks[t.ID] = t
		s.order = append(s.order, t.ID)
		if t.ID > s.nextID {
			s.nextID = t.ID
		}
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

func (t *Task) clone() *Task {
	c := *t
	c.Tags = append([]string(nil), t.Tags...)
	return &c
}
func cloneAll(in []*Task) []*Task {
	out := make([]*Task, len(in))
	for i, t := range in {
		out[i] = t.clone()
	}
	return out
}
func (s *Store) Create(in *Task) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := in.clone()
	s.nextID++
	t.ID = s.nextID
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	if t.Status == "" {
		t.Status = "todo"
	}
	s.tasks[t.ID] = t
	s.order = append(s.order, t.ID)
	s.save()
	return t.clone()
}

// Get returns the task with the given id, or nil.
func (s *Store) Get(id int) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil
	}
	return t.clone()
}

// Update applies fn to the stored task while holding the write lock.
// It returns a copy of the result, or (nil, false) if the task dosen't exist.
func (s *Store) Update(id int, fn func(*Task)) (*Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	fn(t)
	t.ID = id // the callback must not be able to change the identity
	t.UpdatedAt = time.Now()
	s.save()
	return t.clone(), true
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; !ok {
		return false
	}
	delete(s.tasks, id)
	for i, v := range s.order {
		if v == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
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
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	start := (page - 1) * limit
	if start >= len(out) {
		return []*Task{} // past the last page: empty, not a panic
	}
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	return cloneAll(out[start:end])
}

// Search returns tasks whose title, description or tag contains q
// (case-insensitive substring match), in insertion order.
func (s *Store) Search(q string) []*Task {
	needle := strings.ToLower(strings.TrimSpace(q))

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := []*Task{}
	for _, id := range s.order {
		t := s.tasks[id]
		if strings.Contains(strings.ToLower(t.Title), needle) ||
		strings.Contains(strings.ToLower(t.Description), needle) ||
		matchesTag(t.Tags, needle) {
			out = append(out, t.clone())
		}
	}
	return out
}

func matchesTag(tags []string, needle string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), needle) {
			return true
		}
	}
	return false

}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st := Stats{ByStatus: map[string]int{}, TopTags: []TagCount{}}
	today := time.Now().Format("2006-01-02") // local date; DueDate is YYYY-MM-DD, so string comparison works
	tagCounts := map[string]int{}
	sum := 0

	for _, t := range s.tasks {
		st.Total++
		st.ByStatus[t.Status]++
		if t.DueDate != "" && t.DueDate < today && t.Status != "done" {
			st.Overdue++
		}
		sum += t.Priority
		for _, tag := range t.Tags {
			tagCounts[tag]++
		}
	}
	if st.Total > 0 {
		st.AvgPriority = float64(sum) / float64(st.Total)
	}

	tags := make([]TagCount, 0, len(tagCounts))
	for tag, n := range tagCounts {
		tags = append(tags, TagCount{tag, n})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Tag < tags[j].Tag
	})
	if len(tags) > 5 {
		tags = tags[:5]
	}
	st.TopTags = tags
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
