package export

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sadopc/trackr/internal/store"
)

type jsonExport struct {
	ExportedAt string        `json:"exported_at"`
	Count      int           `json:"count"`
	Entries    []jsonEntry   `json:"entries"`
}

type jsonEntry struct {
	ID          int64   `json:"id"`
	Project     string  `json:"project"`
	ProjectID   int64   `json:"project_id"`
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time,omitempty"`
	DurationSec int64   `json:"duration_seconds"`
	Duration    string  `json:"duration"`
	Notes       string  `json:"notes,omitempty"`
}

func ToJSON(entries []store.TimeEntry, projects map[int64]*store.Project, path string) error {
	export := jsonExport{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Count:      len(entries),
	}

	for _, e := range entries {
		projectName := "Unknown"
		if p, ok := projects[e.ProjectID]; ok {
			projectName = p.Name
		}
		endStr := ""
		if e.EndTime != nil {
			endStr = e.EndTime.Local().Format(time.RFC3339)
		}

		export.Entries = append(export.Entries, jsonEntry{
			ID:          e.ID,
			Project:     projectName,
			ProjectID:   e.ProjectID,
			StartTime:   e.StartTime.Local().Format(time.RFC3339),
			EndTime:     endStr,
			DurationSec: e.Duration,
			Duration:    formatDuration(e.Duration),
			Notes:       e.Notes,
		})
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json file: %w", err)
	}
	return nil
}
