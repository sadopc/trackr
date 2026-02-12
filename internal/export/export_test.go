package export

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sadopc/trackr/internal/store"
)

func sampleData() ([]store.TimeEntry, map[int64]*store.Project) {
	now := time.Now().UTC()
	end := now
	tid := int64(10)

	entries := []store.TimeEntry{
		{
			ID:        1,
			ProjectID: 1,
			TaskID:    nil,
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   &end,
			Duration:  3600,
			Notes:     "worked on feature",
			CreatedAt: now,
		},
		{
			ID:        2,
			ProjectID: 2,
			TaskID:    &tid,
			StartTime: now.Add(-30 * time.Minute),
			EndTime:   &end,
			Duration:  1800,
			Notes:     "",
			CreatedAt: now,
		},
		{
			ID:        3,
			ProjectID: 1,
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   nil, // still running
			Duration:  0,
			Notes:     "",
			CreatedAt: now,
		},
	}

	projects := map[int64]*store.Project{
		1: {ID: 1, Name: "Project Alpha", Color: "#FF0000"},
		2: {ID: 2, Name: "Project Beta", Color: "#00FF00"},
	}

	return entries, projects
}

// ============================================================
// CSV
// ============================================================

func TestToCSV(t *testing.T) {
	entries, projects := sampleData()
	path := filepath.Join(t.TempDir(), "test.csv")

	err := ToCSV(entries, projects, path)
	if err != nil {
		t.Fatalf("ToCSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	// header + 3 data rows
	if len(records) != 4 {
		t.Fatalf("expected 4 rows (1 header + 3 data), got %d", len(records))
	}

	// Check header
	header := records[0]
	expectedHeader := []string{"ID", "Project", "Start", "End", "Duration (s)", "Duration", "Notes"}
	for i, h := range expectedHeader {
		if header[i] != h {
			t.Fatalf("header[%d] = %q, want %q", i, header[i], h)
		}
	}

	// Check first data row
	row := records[1]
	if row[0] != "1" {
		t.Fatalf("ID = %q, want 1", row[0])
	}
	if row[1] != "Project Alpha" {
		t.Fatalf("Project = %q, want Project Alpha", row[1])
	}
	if row[4] != "3600" {
		t.Fatalf("Duration (s) = %q, want 3600", row[4])
	}
	if row[5] != "01:00:00" {
		t.Fatalf("Duration = %q, want 01:00:00", row[5])
	}
	if row[6] != "worked on feature" {
		t.Fatalf("Notes = %q, want 'worked on feature'", row[6])
	}

	// Check running entry has empty end time
	runningRow := records[3]
	if runningRow[3] != "" {
		t.Fatalf("running entry should have empty end time, got %q", runningRow[3])
	}
}

func TestToCSVEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.csv")

	err := ToCSV(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	if len(records) != 1 {
		t.Fatalf("expected 1 row (header only), got %d", len(records))
	}
}

func TestToCSVUnknownProject(t *testing.T) {
	entries := []store.TimeEntry{
		{
			ID:        1,
			ProjectID: 999,
			StartTime: time.Now(),
			Duration:  60,
		},
	}
	path := filepath.Join(t.TempDir(), "unknown.csv")

	err := ToCSV(entries, map[int64]*store.Project{}, path)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	if records[1][1] != "Unknown" {
		t.Fatalf("expected 'Unknown' for missing project, got %q", records[1][1])
	}
}

func TestToCSVBadPath(t *testing.T) {
	err := ToCSV(nil, nil, "/nonexistent/dir/file.csv")
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}

func TestToCSVSpecialCharacters(t *testing.T) {
	now := time.Now()
	end := now
	entries := []store.TimeEntry{
		{
			ID:        1,
			ProjectID: 1,
			StartTime: now,
			EndTime:   &end,
			Duration:  60,
			Notes:     `notes with "quotes" and, commas`,
		},
	}
	projects := map[int64]*store.Project{
		1: {ID: 1, Name: `Project "Special"`},
	}
	path := filepath.Join(t.TempDir(), "special.csv")

	err := ToCSV(entries, projects, path)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV should be valid even with special chars: %v", err)
	}
	if records[1][1] != `Project "Special"` {
		t.Fatalf("project name mangled: %q", records[1][1])
	}
	if records[1][6] != `notes with "quotes" and, commas` {
		t.Fatalf("notes mangled: %q", records[1][6])
	}
}

// ============================================================
// JSON
// ============================================================

func TestToJSON(t *testing.T) {
	entries, projects := sampleData()
	path := filepath.Join(t.TempDir(), "test.json")

	err := ToJSON(entries, projects, path)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var result jsonExport
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result.Count != 3 {
		t.Fatalf("count = %d, want 3", result.Count)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(result.Entries))
	}
	if result.ExportedAt == "" {
		t.Fatal("exported_at should not be empty")
	}

	// Check first entry
	e := result.Entries[0]
	if e.ID != 1 {
		t.Fatalf("ID = %d, want 1", e.ID)
	}
	if e.Project != "Project Alpha" {
		t.Fatalf("Project = %q, want Project Alpha", e.Project)
	}
	if e.DurationSec != 3600 {
		t.Fatalf("DurationSec = %d, want 3600", e.DurationSec)
	}
	if e.Duration != "01:00:00" {
		t.Fatalf("Duration = %q, want 01:00:00", e.Duration)
	}
	if e.Notes != "worked on feature" {
		t.Fatalf("Notes = %q", e.Notes)
	}

	// Running entry should have empty end_time
	running := result.Entries[2]
	if running.EndTime != "" {
		t.Fatalf("running entry end_time should be empty, got %q", running.EndTime)
	}
}

func TestToJSONEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")

	err := ToJSON(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var result jsonExport
	json.Unmarshal(data, &result)

	if result.Count != 0 {
		t.Fatalf("count = %d, want 0", result.Count)
	}
	if result.Entries != nil {
		t.Fatal("entries should be nil/null for empty export")
	}
}

func TestToJSONUnknownProject(t *testing.T) {
	entries := []store.TimeEntry{
		{ID: 1, ProjectID: 999, StartTime: time.Now(), Duration: 60},
	}
	path := filepath.Join(t.TempDir(), "unknown.json")

	ToJSON(entries, map[int64]*store.Project{}, path)

	data, _ := os.ReadFile(path)
	var result jsonExport
	json.Unmarshal(data, &result)
	if result.Entries[0].Project != "Unknown" {
		t.Fatalf("expected 'Unknown', got %q", result.Entries[0].Project)
	}
}

func TestToJSONBadPath(t *testing.T) {
	err := ToJSON(nil, nil, "/nonexistent/dir/file.json")
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}

func TestToJSONPrettyPrinted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pretty.json")
	ToJSON(nil, nil, path)

	data, _ := os.ReadFile(path)
	// Pretty-printed JSON should contain newlines and indentation
	if !strings.Contains(string(data), "\n") {
		t.Fatal("JSON should be pretty-printed with newlines")
	}
	if !strings.Contains(string(data), "  ") {
		t.Fatal("JSON should be indented with spaces")
	}
}

func TestToJSONValidTimestamps(t *testing.T) {
	entries, projects := sampleData()
	path := filepath.Join(t.TempDir(), "ts.json")
	ToJSON(entries, projects, path)

	data, _ := os.ReadFile(path)
	var result jsonExport
	json.Unmarshal(data, &result)

	// exported_at should be valid RFC3339
	_, err := time.Parse(time.RFC3339, result.ExportedAt)
	if err != nil {
		t.Fatalf("exported_at is not valid RFC3339: %q", result.ExportedAt)
	}

	// entry timestamps should be valid RFC3339
	for _, e := range result.Entries {
		_, err := time.Parse(time.RFC3339, e.StartTime)
		if err != nil {
			t.Fatalf("start_time is not valid RFC3339: %q", e.StartTime)
		}
	}
}

// ============================================================
// formatDuration (internal helper)
// ============================================================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{0, "00:00:00"},
		{1, "00:00:01"},
		{60, "00:01:00"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{86400, "24:00:00"},
		{90061, "25:01:01"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.secs)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}
