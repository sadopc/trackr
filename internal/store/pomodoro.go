package store

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) StartPomodoro(timeEntryID *int64, workDuration, breakDuration, targetCount int) (*PomodoroSession, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO pomodoro_sessions (time_entry_id, work_duration, break_duration, target_count, status, started_at)
		 VALUES (?, ?, ?, ?, 'working', ?)`,
		timeEntryID, workDuration, breakDuration, targetCount, now,
	)
	if err != nil {
		return nil, fmt.Errorf("start pomodoro: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetPomodoro(id)
}

func (s *Store) GetPomodoro(id int64) (*PomodoroSession, error) {
	p := &PomodoroSession{}
	var startedAt string
	var completedAt sql.NullString
	var entryID sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, time_entry_id, work_duration, break_duration, completed_count, target_count, status, started_at, completed_at
		 FROM pomodoro_sessions WHERE id = ?`, id,
	).Scan(&p.ID, &entryID, &p.WorkDuration, &p.BreakDuration, &p.CompletedCount, &p.TargetCount, &p.Status, &startedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("get pomodoro %d: %w", id, err)
	}
	if entryID.Valid {
		p.TimeEntryID = &entryID.Int64
	}
	p.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		p.CompletedAt = &t
	}
	return p, nil
}

func (s *Store) CompletePomodoro(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET status = 'completed', completed_at = ?, completed_count = target_count WHERE id = ?`,
		now, id,
	)
	return err
}

func (s *Store) IncrementPomodoro(id int64) error {
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET completed_count = completed_count + 1 WHERE id = ?`, id,
	)
	return err
}

func (s *Store) UpdatePomodoroStatus(id int64, status string) error {
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET status = ? WHERE id = ?`, status, id,
	)
	return err
}

func (s *Store) CancelPomodoro(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET status = 'cancelled', completed_at = ? WHERE id = ?`,
		now, id,
	)
	return err
}

func (s *Store) GetPomodoroStats(from, to time.Time) (completed int, totalWork int64, err error) {
	err = s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(work_duration * completed_count), 0)
		FROM pomodoro_sessions
		WHERE status = 'completed'
		  AND started_at >= ? AND started_at < ?`,
		from.Format(time.RFC3339), to.Format(time.RFC3339),
	).Scan(&completed, &totalWork)
	return
}
