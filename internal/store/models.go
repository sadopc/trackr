package store

import "time"

type Project struct {
	ID        int64
	Name      string
	Color     string
	Category  string
	Archived  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Task struct {
	ID        int64
	ProjectID int64
	Name      string
	Tags      string
	Archived  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TimeEntry struct {
	ID        int64
	ProjectID int64
	TaskID    *int64
	StartTime time.Time
	EndTime   *time.Time
	Duration  int64 // seconds
	Notes     string
	CreatedAt time.Time
}

type PomodoroSession struct {
	ID             int64
	TimeEntryID    *int64
	WorkDuration   int
	BreakDuration  int
	CompletedCount int
	TargetCount    int
	Status         string // idle, working, short_break, long_break, completed, cancelled
	StartedAt      time.Time
	CompletedAt    *time.Time
}

type Setting struct {
	Key   string
	Value string
}

// EntryFilter is used to filter time entries in queries.
type EntryFilter struct {
	ProjectID *int64
	TaskID    *int64
	From      *time.Time
	To        *time.Time
	Limit     int
}

// DailySummary represents aggregated time per project per day.
type DailySummary struct {
	Date        string
	ProjectID   int64
	ProjectName string
	ProjectColor string
	TotalSeconds int64
	EntryCount  int
}
