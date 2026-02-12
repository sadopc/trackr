package store

import (
	"fmt"
	"time"
)

func (s *Store) CreateProject(name, color, category string) (*Project, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO projects (name, color, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, color, category, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetProject(id)
}

func (s *Store) GetProject(id int64) (*Project, error) {
	p := &Project{}
	var createdAt, updatedAt string
	var archived int
	err := s.db.QueryRow(
		`SELECT id, name, color, category, archived, created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Color, &p.Category, &archived, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	p.Archived = archived == 1
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return p, nil
}

func (s *Store) ListProjects(includeArchived bool) ([]Project, error) {
	query := `SELECT id, name, color, category, archived, created_at, updated_at FROM projects`
	if !includeArchived {
		query += ` WHERE archived = 0`
	}
	query += ` ORDER BY name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt, updatedAt string
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Color, &p.Category, &archived, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.Archived = archived == 1
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) UpdateProject(id int64, name, color, category string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE projects SET name = ?, color = ?, category = ?, updated_at = ? WHERE id = ?`,
		name, color, category, now, id,
	)
	return err
}

func (s *Store) ArchiveProject(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE projects SET archived = 1, updated_at = ? WHERE id = ?`, now, id,
	)
	return err
}
