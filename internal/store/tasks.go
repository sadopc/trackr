package store

import (
	"fmt"
	"time"
)

func (s *Store) CreateTask(projectID int64, name, tags string) (*Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO tasks (project_id, name, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		projectID, name, tags, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetTask(id)
}

func (s *Store) GetTask(id int64) (*Task, error) {
	t := &Task{}
	var createdAt, updatedAt string
	var archived int
	err := s.db.QueryRow(
		`SELECT id, project_id, name, tags, archived, created_at, updated_at FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Name, &t.Tags, &archived, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task %d: %w", id, err)
	}
	t.Archived = archived == 1
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return t, nil
}

func (s *Store) ListTasks(projectID int64, includeArchived bool) ([]Task, error) {
	query := `SELECT id, project_id, name, tags, archived, created_at, updated_at FROM tasks WHERE project_id = ?`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	query += ` ORDER BY name`

	rows, err := s.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var createdAt, updatedAt string
		var archived int
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Tags, &archived, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.Archived = archived == 1
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTask(id int64, name, tags string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE tasks SET name = ?, tags = ?, updated_at = ? WHERE id = ?`,
		name, tags, now, id,
	)
	return err
}

func (s *Store) ArchiveTask(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE tasks SET archived = 1, updated_at = ? WHERE id = ?`, now, id,
	)
	return err
}
