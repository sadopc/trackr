package tui

import (
	"fmt"
	"time"

	"github.com/sadopc/trackr/internal/store"
)

// viewState represents the currently active view.
type viewState int

const (
	viewDashboard viewState = iota
	viewProjects
	viewReports
	viewPomodoro
	viewSettings
)

var viewNames = []string{"Dashboard", "Projects", "Reports", "Pomodoro", "Settings"}

// --- Messages ---

type timerStartedMsg struct {
	entry *store.TimeEntry
}

type timerStoppedMsg struct {
	entry *store.TimeEntry
}

type timerPausedMsg struct{}
type timerResumedMsg struct{}

type projectCreatedMsg struct {
	project *store.Project
}

type projectUpdatedMsg struct{}

type taskCreatedMsg struct {
	task *store.Task
}

type statusMsg struct {
	text    string
	isError bool
}

type tickMsg time.Time

type pomodoroPhaseMsg struct {
	phase string // "work", "short_break", "long_break", "completed"
}

type exportDoneMsg struct {
	path string
}

type formDoneMsg struct{}
type formCancelMsg struct{}

// --- Helpers ---

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func formatSeconds(secs int64) string {
	return formatDuration(time.Duration(secs) * time.Second)
}

func formatHours(secs int64) string {
	h := float64(secs) / 3600
	return fmt.Sprintf("%.1fh", h)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
