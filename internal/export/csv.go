package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/sadopc/trackr/internal/store"
)

func ToCSV(entries []store.TimeEntry, projects map[int64]*store.Project, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	if err := w.Write([]string{"ID", "Project", "Start", "End", "Duration (s)", "Duration", "Notes"}); err != nil {
		return err
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
		dur := formatDuration(e.Duration)

		row := []string{
			fmt.Sprintf("%d", e.ID),
			projectName,
			e.StartTime.Local().Format(time.RFC3339),
			endStr,
			fmt.Sprintf("%d", e.Duration),
			dur,
			e.Notes,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return w.Error()
}

func formatDuration(secs int64) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
