package store

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) StartEntry(projectID int64, taskID *int64) (*TimeEntry, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO time_entries (project_id, task_id, start_time, created_at) VALUES (?, ?, ?, ?)`,
		projectID, taskID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("start entry: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetEntry(id)
}

func (s *Store) StopEntry(id int64) (*TimeEntry, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Get start_time to compute duration.
	var startStr string
	err := s.db.QueryRow(`SELECT start_time FROM time_entries WHERE id = ?`, id).Scan(&startStr)
	if err != nil {
		return nil, fmt.Errorf("get entry start: %w", err)
	}
	start, _ := time.Parse(time.RFC3339, startStr)
	duration := int64(now.Sub(start).Seconds())

	_, err = s.db.Exec(
		`UPDATE time_entries SET end_time = ?, duration = ? WHERE id = ?`,
		nowStr, duration, id,
	)
	if err != nil {
		return nil, fmt.Errorf("stop entry: %w", err)
	}
	return s.GetEntry(id)
}

func (s *Store) GetEntry(id int64) (*TimeEntry, error) {
	e := &TimeEntry{}
	var startTime, createdAt string
	var endTime sql.NullString
	var taskID sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, project_id, task_id, start_time, end_time, duration, notes, created_at
		 FROM time_entries WHERE id = ?`, id,
	).Scan(&e.ID, &e.ProjectID, &taskID, &startTime, &endTime, &e.Duration, &e.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get entry %d: %w", id, err)
	}
	if taskID.Valid {
		e.TaskID = &taskID.Int64
	}
	e.StartTime, _ = time.Parse(time.RFC3339, startTime)
	if endTime.Valid {
		t, _ := time.Parse(time.RFC3339, endTime.String)
		e.EndTime = &t
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return e, nil
}

func (s *Store) GetRunningEntry() (*TimeEntry, error) {
	e := &TimeEntry{}
	var startTime, createdAt string
	var endTime sql.NullString
	var taskID sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, project_id, task_id, start_time, end_time, duration, notes, created_at
		 FROM time_entries WHERE end_time IS NULL ORDER BY id DESC LIMIT 1`,
	).Scan(&e.ID, &e.ProjectID, &taskID, &startTime, &endTime, &e.Duration, &e.Notes, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get running entry: %w", err)
	}
	if taskID.Valid {
		e.TaskID = &taskID.Int64
	}
	e.StartTime, _ = time.Parse(time.RFC3339, startTime)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return e, nil
}

func (s *Store) UpdateEntryNotes(id int64, notes string) error {
	_, err := s.db.Exec(`UPDATE time_entries SET notes = ? WHERE id = ?`, notes, id)
	return err
}

func (s *Store) ListEntries(f EntryFilter) ([]TimeEntry, error) {
	query := `SELECT id, project_id, task_id, start_time, end_time, duration, notes, created_at FROM time_entries WHERE 1=1`
	var args []any

	if f.ProjectID != nil {
		query += ` AND project_id = ?`
		args = append(args, *f.ProjectID)
	}
	if f.TaskID != nil {
		query += ` AND task_id = ?`
		args = append(args, *f.TaskID)
	}
	if f.From != nil {
		query += ` AND start_time >= ?`
		args = append(args, f.From.Format(time.RFC3339))
	}
	if f.To != nil {
		query += ` AND start_time < ?`
		args = append(args, f.To.Format(time.RFC3339))
	}
	query += ` ORDER BY start_time DESC`
	if f.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, f.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	var entries []TimeEntry
	for rows.Next() {
		var e TimeEntry
		var startTime, createdAt string
		var endTime sql.NullString
		var taskID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.ProjectID, &taskID, &startTime, &endTime, &e.Duration, &e.Notes, &createdAt); err != nil {
			return nil, err
		}
		if taskID.Valid {
			e.TaskID = &taskID.Int64
		}
		e.StartTime, _ = time.Parse(time.RFC3339, startTime)
		if endTime.Valid {
			t, _ := time.Parse(time.RFC3339, endTime.String)
			e.EndTime = &t
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) GetDailySummary(from, to time.Time) ([]DailySummary, error) {
	rows, err := s.db.Query(`
		SELECT date(e.start_time) AS day, e.project_id, p.name, p.color,
		       COALESCE(SUM(e.duration), 0), COUNT(*)
		FROM time_entries e
		JOIN projects p ON p.id = e.project_id
		WHERE e.end_time IS NOT NULL
		  AND e.start_time >= ? AND e.start_time < ?
		GROUP BY day, e.project_id
		ORDER BY day, p.name`,
		from.Format(time.RFC3339), to.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("daily summary: %w", err)
	}
	defer rows.Close()

	var summaries []DailySummary
	for rows.Next() {
		var ds DailySummary
		if err := rows.Scan(&ds.Date, &ds.ProjectID, &ds.ProjectName, &ds.ProjectColor, &ds.TotalSeconds, &ds.EntryCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, ds)
	}
	return summaries, rows.Err()
}

func (s *Store) GetTodayTotal() (int64, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var total sql.NullInt64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM time_entries
		WHERE date(start_time) = ? AND end_time IS NOT NULL`, today,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}
