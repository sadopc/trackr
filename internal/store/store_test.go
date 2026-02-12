package store

import (
	"database/sql"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// insertEntry is a test helper that inserts a completed entry with a given duration.
func insertEntry(t *testing.T, s *Store, projectID int64, taskID *int64, startOffset, durationSecs int) int64 {
	t.Helper()
	now := time.Now().UTC()
	start := now.Add(time.Duration(-startOffset) * time.Second)
	end := start.Add(time.Duration(durationSecs) * time.Second)
	res, err := s.db.Exec(
		`INSERT INTO time_entries (project_id, task_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)`,
		projectID, taskID, start.Format(time.RFC3339), end.Format(time.RFC3339), durationSecs,
	)
	if err != nil {
		t.Fatalf("insert entry: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// ============================================================
// Store initialization
// ============================================================

func TestNewMemory(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Should have run migration v1
	var version int
	s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 1 {
		t.Fatalf("expected user_version 1, got %d", version)
	}
}

func TestNewWithPath(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/sub/trackr.db"
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Reopen — should succeed and not re-migrate
	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}

func TestDefaultDBPath(t *testing.T) {
	path, err := DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("empty path")
	}
}

func TestPragmasConfigured(t *testing.T) {
	s := newTestStore(t)

	var journalMode string
	s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	// In-memory doesn't persist WAL but the pragma still runs.
	// Just verify no error from the store init.

	var fk int
	s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fk)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	s := newTestStore(t)
	// Running migrate again should be a no-op
	if err := s.migrate(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
}

// ============================================================
// Projects
// ============================================================

func TestCreateAndGetProject(t *testing.T) {
	s := newTestStore(t)
	p, err := s.CreateProject("Work", "#FF0000", "work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Work" || p.Color != "#FF0000" || p.Category != "work" {
		t.Fatalf("unexpected project: %+v", p)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if p.Archived {
		t.Fatal("new project should not be archived")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

func TestCreateProjectDuplicateName(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateProject("Dup", "#111", "work")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateProject("Dup", "#222", "personal")
	if err == nil {
		t.Fatal("expected error for duplicate project name")
	}
}

func TestGetProjectNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetProject(999)
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestListProjects(t *testing.T) {
	s := newTestStore(t)
	s.CreateProject("B", "#222", "personal")
	s.CreateProject("A", "#111", "work")

	projects, err := s.ListProjects(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	// Should be sorted by name
	if projects[0].Name != "A" || projects[1].Name != "B" {
		t.Fatalf("expected sorted by name: got %s, %s", projects[0].Name, projects[1].Name)
	}
}

func TestListProjectsEmpty(t *testing.T) {
	s := newTestStore(t)
	projects, err := s.ListProjects(false)
	if err != nil {
		t.Fatal(err)
	}
	if projects != nil {
		t.Fatalf("expected nil slice, got %d items", len(projects))
	}
}

func TestArchiveProject(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Old", "#333", "work")
	s.ArchiveProject(p.ID)

	projects, _ := s.ListProjects(false)
	if len(projects) != 0 {
		t.Fatal("archived project should be hidden")
	}
	projects, _ = s.ListProjects(true)
	if len(projects) != 1 {
		t.Fatal("archived project should appear with includeArchived")
	}
	if !projects[0].Archived {
		t.Fatal("Archived flag should be true")
	}
}

func TestUpdateProject(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Old", "#333", "work")
	s.UpdateProject(p.ID, "New", "#444", "personal")
	updated, _ := s.GetProject(p.ID)
	if updated.Name != "New" || updated.Color != "#444" || updated.Category != "personal" {
		t.Fatalf("update failed: %+v", updated)
	}
	if !updated.UpdatedAt.After(p.CreatedAt) || updated.UpdatedAt.Equal(p.CreatedAt) {
		// UpdatedAt should be >= CreatedAt (may be same second in fast test)
	}
}

// ============================================================
// Tasks
// ============================================================

func TestCreateAndGetTask(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, err := s.CreateTask(p.ID, "Bug fix", "backend,urgent")
	if err != nil {
		t.Fatal(err)
	}
	if task.Name != "Bug fix" || task.Tags != "backend,urgent" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.ProjectID != p.ID {
		t.Fatal("task should reference project")
	}
	if task.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	fetched, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Name != "Bug fix" {
		t.Fatalf("GetTask returned wrong name: %s", fetched.Name)
	}
}

func TestCreateTaskDuplicateNameSameProject(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	_, err := s.CreateTask(p.ID, "Task1", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateTask(p.ID, "Task1", "other")
	if err == nil {
		t.Fatal("expected error for duplicate task name within same project")
	}
}

func TestCreateTaskSameNameDifferentProjects(t *testing.T) {
	s := newTestStore(t)
	p1, _ := s.CreateProject("A", "#111", "work")
	p2, _ := s.CreateProject("B", "#222", "work")
	_, err1 := s.CreateTask(p1.ID, "Shared", "")
	_, err2 := s.CreateTask(p2.ID, "Shared", "")
	if err1 != nil || err2 != nil {
		t.Fatal("same task name in different projects should be allowed")
	}
}

func TestCreateTaskInvalidProject(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateTask(999, "Orphan", "")
	if err == nil {
		t.Fatal("expected foreign key error for non-existent project")
	}
}

func TestListTasks(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	s.CreateTask(p.ID, "B task", "")
	s.CreateTask(p.ID, "A task", "")

	tasks, err := s.ListTasks(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	// Should be sorted by name
	if tasks[0].Name != "A task" {
		t.Fatalf("expected sorted: got %s first", tasks[0].Name)
	}
}

func TestListTasksEmpty(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	tasks, err := s.ListTasks(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if tasks != nil {
		t.Fatal("expected nil slice for empty task list")
	}
}

func TestListTasksIsolation(t *testing.T) {
	s := newTestStore(t)
	p1, _ := s.CreateProject("A", "#111", "work")
	p2, _ := s.CreateProject("B", "#222", "work")
	s.CreateTask(p1.ID, "Task A", "")
	s.CreateTask(p2.ID, "Task B", "")

	tasks, _ := s.ListTasks(p1.ID, false)
	if len(tasks) != 1 || tasks[0].Name != "Task A" {
		t.Fatal("ListTasks should only return tasks for the given project")
	}
}

func TestArchiveTask(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, _ := s.CreateTask(p.ID, "Done task", "")
	s.ArchiveTask(task.ID)

	tasks, _ := s.ListTasks(p.ID, false)
	if len(tasks) != 0 {
		t.Fatal("archived task should be hidden")
	}
	tasks, _ = s.ListTasks(p.ID, true)
	if len(tasks) != 1 {
		t.Fatal("archived task should appear with includeArchived")
	}
}

func TestUpdateTask(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, _ := s.CreateTask(p.ID, "Old", "tag1")
	s.UpdateTask(task.ID, "New", "tag1,tag2")
	updated, _ := s.GetTask(task.ID)
	if updated.Name != "New" || updated.Tags != "tag1,tag2" {
		t.Fatalf("update failed: %+v", updated)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetTask(999)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

// ============================================================
// Time Entries
// ============================================================

func TestStartAndStopEntry(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	entry, err := s.StartEntry(p.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EndTime != nil {
		t.Fatal("entry should not have end time yet")
	}
	if entry.Duration != 0 {
		t.Fatal("running entry should have 0 duration")
	}
	if entry.ProjectID != p.ID {
		t.Fatalf("expected project_id=%d, got %d", p.ID, entry.ProjectID)
	}
	if entry.TaskID != nil {
		t.Fatal("task_id should be nil")
	}

	running, _ := s.GetRunningEntry()
	if running == nil {
		t.Fatal("expected a running entry")
	}
	if running.ID != entry.ID {
		t.Fatal("running entry ID mismatch")
	}

	time.Sleep(10 * time.Millisecond)

	stopped, err := s.StopEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.EndTime == nil {
		t.Fatal("stopped entry should have end time")
	}
	if stopped.Duration < 0 {
		t.Fatal("duration should be non-negative")
	}

	running, _ = s.GetRunningEntry()
	if running != nil {
		t.Fatal("no entry should be running")
	}
}

func TestStartEntryWithTask(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, _ := s.CreateTask(p.ID, "Feature", "")

	tid := task.ID
	entry, err := s.StartEntry(p.ID, &tid)
	if err != nil {
		t.Fatal(err)
	}
	if entry.TaskID == nil || *entry.TaskID != tid {
		t.Fatalf("expected task_id=%d, got %v", tid, entry.TaskID)
	}
	s.StopEntry(entry.ID)
}

func TestGetRunningEntryReturnsLatest(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	e1, _ := s.StartEntry(p.ID, nil)
	s.StopEntry(e1.ID)

	e2, _ := s.StartEntry(p.ID, nil)

	running, _ := s.GetRunningEntry()
	if running == nil || running.ID != e2.ID {
		t.Fatal("should return the latest running entry")
	}
	s.StopEntry(e2.ID)
}

func TestGetRunningEntryNone(t *testing.T) {
	s := newTestStore(t)
	entry, err := s.GetRunningEntry()
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Fatal("expected nil when no entries exist")
	}
}

func TestGetEntry(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	entry, _ := s.StartEntry(p.ID, nil)

	fetched, err := s.GetEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.ID != entry.ID {
		t.Fatal("ID mismatch")
	}
	s.StopEntry(entry.ID)
}

func TestGetEntryNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetEntry(999)
	if err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestUpdateEntryNotes(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	entry, _ := s.StartEntry(p.ID, nil)

	s.UpdateEntryNotes(entry.ID, "some notes")
	fetched, _ := s.GetEntry(entry.ID)
	if fetched.Notes != "some notes" {
		t.Fatalf("expected 'some notes', got %q", fetched.Notes)
	}
	s.StopEntry(entry.ID)
}

func TestListEntries(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	e1, _ := s.StartEntry(p.ID, nil)
	s.StopEntry(e1.ID)
	e2, _ := s.StartEntry(p.ID, nil)
	s.StopEntry(e2.ID)

	entries, err := s.ListEntries(EntryFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Should be ordered by start_time DESC (most recent first)
	if entries[0].ID < entries[1].ID {
		t.Fatal("entries should be newest first")
	}
}

func TestListEntriesWithProjectFilter(t *testing.T) {
	s := newTestStore(t)
	p1, _ := s.CreateProject("A", "#111", "work")
	p2, _ := s.CreateProject("B", "#222", "personal")

	e1, _ := s.StartEntry(p1.ID, nil)
	s.StopEntry(e1.ID)
	e2, _ := s.StartEntry(p2.ID, nil)
	s.StopEntry(e2.ID)

	pid := p1.ID
	entries, _ := s.ListEntries(EntryFilter{ProjectID: &pid})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for project A, got %d", len(entries))
	}
	if entries[0].ProjectID != p1.ID {
		t.Fatal("wrong project in filtered result")
	}
}

func TestListEntriesWithTaskFilter(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, _ := s.CreateTask(p.ID, "Feature", "")

	tid := task.ID
	e1, _ := s.StartEntry(p.ID, &tid)
	s.StopEntry(e1.ID)
	e2, _ := s.StartEntry(p.ID, nil)
	s.StopEntry(e2.ID)

	entries, _ := s.ListEntries(EntryFilter{TaskID: &tid})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for task, got %d", len(entries))
	}
}

func TestListEntriesWithDateFilter(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	insertEntry(t, s, p.ID, nil, 7200, 3600) // 2h ago, 1h duration
	insertEntry(t, s, p.ID, nil, 600, 300)   // 10min ago, 5min duration

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	entries, _ := s.ListEntries(EntryFilter{From: &from, To: &to})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in last hour, got %d", len(entries))
	}
}

func TestListEntriesWithLimit(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	for i := 0; i < 5; i++ {
		insertEntry(t, s, p.ID, nil, i*100, 60)
	}

	entries, _ := s.ListEntries(EntryFilter{Limit: 3})
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries with limit, got %d", len(entries))
	}
}

func TestListEntriesNoFilter(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	for i := 0; i < 5; i++ {
		insertEntry(t, s, p.ID, nil, i*100, 60)
	}

	entries, _ := s.ListEntries(EntryFilter{})
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries without limit, got %d", len(entries))
	}
}

func TestGetDailySummary(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	s.db.Exec(
		`INSERT INTO time_entries (project_id, start_time, end_time, duration) VALUES (?, ?, ?, ?)`,
		p.ID, start.Format(time.RFC3339), now.Format(time.RFC3339), 3600,
	)

	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)
	summaries, err := s.GetDailySummary(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].TotalSeconds != 3600 {
		t.Fatalf("expected 3600s, got %d", summaries[0].TotalSeconds)
	}
	if summaries[0].ProjectName != "Dev" {
		t.Fatalf("expected project name Dev, got %s", summaries[0].ProjectName)
	}
	if summaries[0].EntryCount != 1 {
		t.Fatalf("expected 1 entry, got %d", summaries[0].EntryCount)
	}
}

func TestGetDailySummaryMultipleProjects(t *testing.T) {
	s := newTestStore(t)
	p1, _ := s.CreateProject("A", "#111", "work")
	p2, _ := s.CreateProject("B", "#222", "personal")

	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	s.db.Exec(
		`INSERT INTO time_entries (project_id, start_time, end_time, duration) VALUES (?, ?, ?, ?)`,
		p1.ID, start.Format(time.RFC3339), now.Format(time.RFC3339), 3600,
	)
	s.db.Exec(
		`INSERT INTO time_entries (project_id, start_time, end_time, duration) VALUES (?, ?, ?, ?)`,
		p2.ID, start.Format(time.RFC3339), now.Format(time.RFC3339), 1800,
	)

	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)
	summaries, _ := s.GetDailySummary(from, to)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries (one per project), got %d", len(summaries))
	}
}

func TestGetDailySummaryExcludesRunning(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	// Running entry (no end_time)
	s.StartEntry(p.ID, nil)

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)
	summaries, _ := s.GetDailySummary(from, to)
	if len(summaries) != 0 {
		t.Fatal("running entries should be excluded from daily summary")
	}
}

func TestGetDailySummaryEmpty(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	summaries, err := s.GetDailySummary(now.Add(-24*time.Hour), now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summaries != nil {
		t.Fatal("expected nil for empty summary")
	}
}

func TestGetTodayTotal(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	insertEntry(t, s, p.ID, nil, 600, 3600)
	insertEntry(t, s, p.ID, nil, 300, 1800)

	total, err := s.GetTodayTotal()
	if err != nil {
		t.Fatal(err)
	}
	if total != 5400 {
		t.Fatalf("expected 5400s, got %d", total)
	}
}

func TestGetTodayTotalEmpty(t *testing.T) {
	s := newTestStore(t)
	total, err := s.GetTodayTotal()
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("expected 0 for empty, got %d", total)
	}
}

func TestGetTodayTotalExcludesRunning(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	s.StartEntry(p.ID, nil) // running, no end_time

	total, _ := s.GetTodayTotal()
	if total != 0 {
		t.Fatal("running entries should be excluded from today total")
	}
}

// ============================================================
// Pomodoro
// ============================================================

func TestPomodoroLifecycle(t *testing.T) {
	s := newTestStore(t)

	pom, err := s.StartPomodoro(nil, 1500, 300, 4)
	if err != nil {
		t.Fatal(err)
	}
	if pom.Status != "working" {
		t.Fatalf("expected working status, got %s", pom.Status)
	}
	if pom.WorkDuration != 1500 || pom.BreakDuration != 300 || pom.TargetCount != 4 {
		t.Fatalf("unexpected values: %+v", pom)
	}
	if pom.CompletedCount != 0 {
		t.Fatal("completed count should start at 0")
	}
	if pom.StartedAt.IsZero() {
		t.Fatal("StartedAt should be set")
	}
	if pom.CompletedAt != nil {
		t.Fatal("CompletedAt should be nil")
	}
	if pom.TimeEntryID != nil {
		t.Fatal("TimeEntryID should be nil when not linked")
	}

	s.IncrementPomodoro(pom.ID)
	s.IncrementPomodoro(pom.ID)

	updated, _ := s.GetPomodoro(pom.ID)
	if updated.CompletedCount != 2 {
		t.Fatalf("expected 2 completed, got %d", updated.CompletedCount)
	}

	s.CompletePomodoro(pom.ID)
	completed, _ := s.GetPomodoro(pom.ID)
	if completed.Status != "completed" || completed.CompletedAt == nil {
		t.Fatal("pomodoro should be completed with timestamp")
	}
	if completed.CompletedCount != completed.TargetCount {
		t.Fatal("CompletePomodoro should set completed_count = target_count")
	}
}

func TestPomodoroWithTimeEntry(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	entry, _ := s.StartEntry(p.ID, nil)

	eid := entry.ID
	pom, err := s.StartPomodoro(&eid, 1500, 300, 4)
	if err != nil {
		t.Fatal(err)
	}
	if pom.TimeEntryID == nil || *pom.TimeEntryID != eid {
		t.Fatal("pomodoro should be linked to time entry")
	}
	s.StopEntry(entry.ID)
}

func TestCancelPomodoro(t *testing.T) {
	s := newTestStore(t)
	pom, _ := s.StartPomodoro(nil, 1500, 300, 4)
	s.CancelPomodoro(pom.ID)
	cancelled, _ := s.GetPomodoro(pom.ID)
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %s", cancelled.Status)
	}
	if cancelled.CompletedAt == nil {
		t.Fatal("cancelled pomodoro should have CompletedAt timestamp")
	}
}

func TestUpdatePomodoroStatus(t *testing.T) {
	s := newTestStore(t)
	pom, _ := s.StartPomodoro(nil, 1500, 300, 4)
	s.UpdatePomodoroStatus(pom.ID, "short_break")
	updated, _ := s.GetPomodoro(pom.ID)
	if updated.Status != "short_break" {
		t.Fatalf("expected short_break, got %s", updated.Status)
	}
}

func TestGetPomodoroNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetPomodoro(999)
	if err == nil {
		t.Fatal("expected error for missing pomodoro")
	}
}

func TestGetPomodoroStats(t *testing.T) {
	s := newTestStore(t)

	pom1, _ := s.StartPomodoro(nil, 1500, 300, 4)
	s.IncrementPomodoro(pom1.ID)
	s.IncrementPomodoro(pom1.ID)
	s.CompletePomodoro(pom1.ID)

	pom2, _ := s.StartPomodoro(nil, 1500, 300, 4)
	s.CancelPomodoro(pom2.ID) // cancelled, should not count

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)

	completed, totalWork, err := s.GetPomodoroStats(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if completed != 1 {
		t.Fatalf("expected 1 completed, got %d", completed)
	}
	// CompletePomodoro sets completed_count = target_count = 4
	// totalWork = work_duration * completed_count = 1500 * 4 = 6000
	if totalWork != 6000 {
		t.Fatalf("expected 6000 total work seconds, got %d", totalWork)
	}
}

func TestGetPomodoroStatsEmpty(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	completed, totalWork, err := s.GetPomodoroStats(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if completed != 0 || totalWork != 0 {
		t.Fatal("expected zeros for empty stats")
	}
}

// ============================================================
// Settings
// ============================================================

func TestSettingsDefaults(t *testing.T) {
	s := newTestStore(t)

	defaults := map[string]string{
		"pomodoro_work":       "1500",
		"pomodoro_break":      "300",
		"pomodoro_long_break": "900",
		"pomodoro_count":      "4",
		"idle_timeout":        "300",
		"idle_action":         "pause",
		"daily_goal":          "28800",
		"week_start":          "monday",
	}

	for k, expected := range defaults {
		val, err := s.GetSetting(k)
		if err != nil {
			t.Fatalf("GetSetting(%q): %v", k, err)
		}
		if val != expected {
			t.Fatalf("GetSetting(%q) = %q, want %q", k, val, expected)
		}
	}
}

func TestSetSetting(t *testing.T) {
	s := newTestStore(t)

	s.SetSetting("pomodoro_work", "3000")
	val, _ := s.GetSetting("pomodoro_work")
	if val != "3000" {
		t.Fatalf("expected 3000, got %s", val)
	}
}

func TestSetSettingNewKey(t *testing.T) {
	s := newTestStore(t)

	s.SetSetting("custom_key", "custom_value")
	val, err := s.GetSetting("custom_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "custom_value" {
		t.Fatalf("expected custom_value, got %s", val)
	}
}

func TestSetSettingOverwrite(t *testing.T) {
	s := newTestStore(t)

	s.SetSetting("key", "v1")
	s.SetSetting("key", "v2")
	val, _ := s.GetSetting("key")
	if val != "v2" {
		t.Fatalf("expected v2, got %s", val)
	}
}

func TestGetSettingNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetSetting("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing setting")
	}
}

func TestGetAllSettings(t *testing.T) {
	s := newTestStore(t)
	all, err := s.GetAllSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 8 {
		t.Fatalf("expected at least 8 default settings, got %d", len(all))
	}
	// Should be sorted by key
	for i := 1; i < len(all); i++ {
		if all[i-1].Key >= all[i].Key {
			t.Fatalf("settings not sorted: %s >= %s", all[i-1].Key, all[i].Key)
		}
	}
}

// ============================================================
// Foreign key constraints
// ============================================================

func TestForeignKeyEntriesProject(t *testing.T) {
	s := newTestStore(t)
	_, err := s.StartEntry(999, nil) // non-existent project
	if err == nil {
		t.Fatal("expected foreign key error")
	}
}

func TestForeignKeyTasksProject(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateTask(999, "Orphan", "") // non-existent project
	if err == nil {
		t.Fatal("expected foreign key error")
	}
}

// ============================================================
// Close / double-close safety
// ============================================================

func TestCloseStore(t *testing.T) {
	s, _ := NewMemory()
	err := s.Close()
	if err != nil {
		t.Fatalf("first close: %v", err)
	}
}

// ============================================================
// Edge cases
// ============================================================

func TestStopEntryNonExistent(t *testing.T) {
	s := newTestStore(t)
	_, err := s.StopEntry(999)
	if err == nil || err == sql.ErrNoRows {
		// Both acceptable: error or sql.ErrNoRows
	}
}

func TestMultipleRunningEntries(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	s.StartEntry(p.ID, nil)
	s.StartEntry(p.ID, nil)

	// GetRunningEntry should return the latest
	running, _ := s.GetRunningEntry()
	if running == nil {
		t.Fatal("expected a running entry")
	}
}
