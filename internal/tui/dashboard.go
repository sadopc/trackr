package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/store"
)

type dashboardModel struct {
	store  *store.Store
	timer  timerModel
	width  int
	height int

	todayTotal    int64
	todaySummary  []store.DailySummary
	recentEntries []store.TimeEntry
	projects      []store.Project

	// Project picker state
	picking       bool
	pickerCursor  int
}

func newDashboardModel(s *store.Store) dashboardModel {
	return dashboardModel{
		store: s,
		timer: newTimerModel(s),
	}
}

func (d dashboardModel) Init() tea.Cmd {
	return d.loadData()
}

func (d *dashboardModel) setSize(w, h int) {
	d.width = w
	d.height = h
}

func (d dashboardModel) isRunning() bool { return d.timer.running() }
func (d dashboardModel) isPaused() bool  { return d.timer.paused() }
func (d dashboardModel) elapsed() time.Duration {
	return d.timer.currentElapsed()
}

type dashboardDataMsg struct {
	todayTotal    int64
	todaySummary  []store.DailySummary
	recentEntries []store.TimeEntry
	projects      []store.Project
}

func (d dashboardModel) loadData() tea.Cmd {
	return func() tea.Msg {
		total, _ := d.store.GetTodayTotal()

		now := time.Now().UTC()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		dayEnd := dayStart.Add(24 * time.Hour)
		summary, _ := d.store.GetDailySummary(dayStart, dayEnd)

		entries, _ := d.store.ListEntries(store.EntryFilter{Limit: 5})
		projects, _ := d.store.ListProjects(false)

		return dashboardDataMsg{
			todayTotal:    total,
			todaySummary:  summary,
			recentEntries: entries,
			projects:      projects,
		}
	}
}

func (d dashboardModel) update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardDataMsg:
		d.todayTotal = msg.todayTotal
		d.todaySummary = msg.todaySummary
		d.recentEntries = msg.recentEntries
		d.projects = msg.projects
		return d, nil

	case tickMsg:
		d.timer.tick()
		return d, nil

	case tea.KeyMsg:
		d.timer.recordActivity()

		if d.picking {
			return d.updatePicker(msg)
		}

		switch {
		case key.Matches(msg, keys.Start):
			if d.timer.running() {
				return d, nil
			}
			if len(d.projects) == 0 {
				return d, func() tea.Msg {
					return statusMsg{text: "No projects yet. Press 2 to go to Projects and create one.", isError: true}
				}
			}
			if len(d.projects) == 1 {
				return d.startTimer(d.projects[0].ID, d.projects[0].Name, nil, "")
			}
			d.picking = true
			d.pickerCursor = 0
			return d, nil

		case key.Matches(msg, keys.Stop):
			return d.stopTimer()

		case key.Matches(msg, keys.Pause):
			d.timer.toggle()
			return d, nil
		}
	}
	return d, nil
}

func (d dashboardModel) updatePicker(msg tea.Msg) (dashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if d.pickerCursor > 0 {
				d.pickerCursor--
			}
		case key.Matches(msg, keys.Down):
			if d.pickerCursor < len(d.projects)-1 {
				d.pickerCursor++
			}
		case key.Matches(msg, keys.Enter):
			p := d.projects[d.pickerCursor]
			d.picking = false
			return d.startTimer(p.ID, p.Name, nil, "")
		case key.Matches(msg, keys.Back):
			d.picking = false
		}
	}
	return d, nil
}

func (d dashboardModel) startTimer(projectID int64, projectName string, taskID *int64, taskName string) (dashboardModel, tea.Cmd) {
	if err := d.timer.start(projectID, projectName, taskID, taskName); err != nil {
		return d, func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("Error: %v", err), isError: true}
		}
	}
	return d, func() tea.Msg { return timerStartedMsg{} }
}

func (d dashboardModel) stopTimer() (dashboardModel, tea.Cmd) {
	entry, err := d.timer.stop()
	if err != nil {
		return d, func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("Error: %v", err), isError: true}
		}
	}
	return d, tea.Batch(
		d.loadData(),
		func() tea.Msg { return timerStoppedMsg{entry: entry} },
	)
}

func (d dashboardModel) view() string {
	if d.width < 20 {
		return "Terminal too small"
	}

	contentWidth := d.width - 4

	// Timer panel
	timerPanel := d.renderTimerPanel(contentWidth)

	// Today summary panel
	summaryPanel := d.renderSummaryPanel(contentWidth)

	// Recent entries or project picker
	var bottomPanel string
	if d.picking {
		bottomPanel = d.renderProjectPicker(contentWidth)
	} else {
		bottomPanel = d.renderRecentPanel(contentWidth)
	}

	return lipgloss.JoinVertical(lipgloss.Left, timerPanel, summaryPanel, bottomPanel)
}

func (d dashboardModel) renderTimerPanel(w int) string {
	var timeDisplay string
	var indicator string

	if d.timer.running() {
		elapsed := d.timer.currentElapsed()
		timeStr := formatDuration(elapsed)

		if d.timer.paused() {
			timeDisplay = timerPausedStyle.Width(w - 6).Render(timeStr)
			if d.timer.isIdle {
				indicator = warningStyle.Render("⏸  IDLE")
			} else {
				indicator = warningStyle.Render("⏸  PAUSED")
			}
		} else {
			timeDisplay = timerRunningStyle.Width(w - 6).Render(timeStr)
			indicator = successStyle.Render("●  RUNNING")
		}

		projectLine := highlightStyle.Render(d.timer.projectName)
		if d.timer.taskName != "" {
			projectLine += mutedStyle.Render(" / " + d.timer.taskName)
		}

		content := lipgloss.JoinVertical(lipgloss.Center,
			timeDisplay,
			indicator,
			projectLine,
		)
		return activePanelStyle.Width(w).Render(content)
	}

	timeDisplay = timerStyle.Width(w - 6).Render("00:00:00")
	indicator = mutedStyle.Render("■  STOPPED")
	hint := mutedStyle.Render("Press s to start tracking")

	content := lipgloss.JoinVertical(lipgloss.Center,
		timeDisplay,
		indicator,
		hint,
	)
	return panelStyle.Width(w).Render(content)
}

func (d dashboardModel) renderSummaryPanel(w int) string {
	title := titleStyle.Render("Today")
	total := highlightStyle.Render(formatSeconds(d.todayTotal))
	header := fmt.Sprintf("%s  %s", title, total)

	if len(d.todaySummary) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			mutedStyle.Render("No entries today"),
		)
		return panelStyle.Width(w).Render(content)
	}

	var rows []string
	rows = append(rows, header)
	for _, s := range d.todaySummary {
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(s.ProjectColor)).Render("●")
		row := fmt.Sprintf("  %s %-20s %s  (%d entries)",
			colorDot,
			s.ProjectName,
			formatSeconds(s.TotalSeconds),
			s.EntryCount,
		)
		rows = append(rows, row)
	}

	return panelStyle.Width(w).Render(strings.Join(rows, "\n"))
}

func (d dashboardModel) renderRecentPanel(w int) string {
	title := titleStyle.Render("Recent Entries")
	if len(d.recentEntries) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			mutedStyle.Render("No entries yet"),
		)
		return panelStyle.Width(w).Render(content)
	}

	var rows []string
	rows = append(rows, title)
	for _, e := range d.recentEntries {
		project, _ := d.store.GetProject(e.ProjectID)
		pName := "?"
		if project != nil {
			pName = project.Name
		}
		dur := formatSeconds(e.Duration)
		startStr := e.StartTime.Local().Format("15:04")
		status := "✓"
		if e.EndTime == nil {
			status = "●"
			dur = "running"
		}
		row := fmt.Sprintf("  %s %s  %-16s %s", status, startStr, pName, dur)
		rows = append(rows, row)
	}

	return panelStyle.Width(w).Render(strings.Join(rows, "\n"))
}

func (d dashboardModel) renderProjectPicker(w int) string {
	title := titleStyle.Render("Select Project")

	var rows []string
	rows = append(rows, title)
	for i, p := range d.projects {
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Color)).Render("●")
		cursor := "  "
		style := normalItemStyle
		if i == d.pickerCursor {
			cursor = "> "
			style = selectedItemStyle
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s %s", cursor, colorDot, p.Name)))
	}
	rows = append(rows, "")
	rows = append(rows, mutedStyle.Render("  enter: select  esc: cancel"))

	return activePanelStyle.Width(w).Render(strings.Join(rows, "\n"))
}
