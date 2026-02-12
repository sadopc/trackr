package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/export"
	"github.com/sadopc/trackr/internal/store"
)

// App is the root Bubble Tea model.
type App struct {
	store  *store.Store
	width  int
	height int

	activeView    viewState
	showHelp      bool
	exportPicking bool
	exportCursor  int

	dashboard dashboardModel
	projects  projectsModel
	reports   reportsModel
	pomodoro  pomodoroModel
	settings  settingsModel

	help   help.Model
	status string
}

func NewApp(s *store.Store) App {
	h := help.New()
	h.ShowAll = false

	return App{
		store:      s,
		activeView: viewDashboard,
		dashboard:  newDashboardModel(s),
		projects:   newProjectsModel(s),
		reports:    newReportsModel(s),
		pomodoro:   newPomodoroModel(s),
		settings:   newSettingsModel(s),
		help:       h,
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.dashboard.Init(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		contentHeight := a.height - 4 // header + footer
		a.dashboard.setSize(a.width, contentHeight)
		a.projects.setSize(a.width, contentHeight)
		a.reports.setSize(a.width, contentHeight)
		a.pomodoro.setSize(a.width, contentHeight)
		a.settings.setSize(a.width, contentHeight)
		return a, nil

	case tea.KeyMsg:
		// Export picker
		if a.exportPicking {
			return a.updateExportPicker(msg)
		}

		// If a child view is capturing input (e.g. form), delegate first.
		if a.isFormActive() {
			return a.updateActiveView(msg)
		}

		switch {
		case key.Matches(msg, keys.Export):
			a.exportPicking = true
			a.exportCursor = 0
			return a, nil
		case key.Matches(msg, keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, keys.Help):
			a.showHelp = !a.showHelp
			a.help.ShowAll = a.showHelp
			return a, nil
		case key.Matches(msg, keys.Tab1):
			a.activeView = viewDashboard
			return a, a.dashboard.loadData()
		case key.Matches(msg, keys.Tab2):
			a.activeView = viewProjects
			return a, a.projects.refresh()
		case key.Matches(msg, keys.Tab3):
			a.activeView = viewReports
			return a, a.reports.refresh()
		case key.Matches(msg, keys.Tab4):
			a.activeView = viewPomodoro
			return a, nil
		case key.Matches(msg, keys.Tab5):
			a.activeView = viewSettings
			return a, a.settings.refresh()
		case key.Matches(msg, keys.Tab):
			a.activeView = (a.activeView + 1) % 5
			return a, a.refreshCurrentView()
		}

	case tickMsg:
		cmds = append(cmds, tickCmd())
		// Always route ticks to dashboard timer
		var cmd tea.Cmd
		a.dashboard, cmd = a.dashboard.update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Also route to pomodoro
		a.pomodoro, cmd = a.pomodoro.update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case statusMsg:
		a.status = msg.text
		return a, nil

	case timerStoppedMsg:
		a.status = "Timer stopped"
		return a, nil

	case timerStartedMsg:
		a.status = "Timer started"
		return a, nil

	case exportDoneMsg:
		a.status = "Exported to " + msg.path
		a.exportPicking = false
		return a, nil
	}

	return a.updateActiveView(msg)
}

func (a App) updateActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.activeView {
	case viewDashboard:
		a.dashboard, cmd = a.dashboard.update(msg)
	case viewProjects:
		a.projects, cmd = a.projects.update(msg)
	case viewReports:
		a.reports, cmd = a.reports.update(msg)
	case viewPomodoro:
		a.pomodoro, cmd = a.pomodoro.update(msg)
	case viewSettings:
		a.settings, cmd = a.settings.update(msg)
	}
	return a, cmd
}

func (a App) isFormActive() bool {
	switch a.activeView {
	case viewProjects:
		return a.projects.formActive
	case viewSettings:
		return a.settings.formActive
	case viewPomodoro:
		return a.pomodoro.formActive
	}
	return false
}

func (a App) refreshCurrentView() tea.Cmd {
	switch a.activeView {
	case viewDashboard:
		return a.dashboard.loadData()
	case viewProjects:
		return a.projects.refresh()
	case viewReports:
		return a.reports.refresh()
	case viewSettings:
		return a.settings.refresh()
	}
	return nil
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	header := a.renderHeader()
	footer := a.renderFooter()

	var content string
	switch a.activeView {
	case viewDashboard:
		content = a.dashboard.view()
	case viewProjects:
		content = a.projects.view()
	case viewReports:
		content = a.reports.view()
	case viewPomodoro:
		content = a.pomodoro.view()
	case viewSettings:
		content = a.settings.view()
	}

	// Calculate available height for content
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := a.height - headerHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Show export picker overlay
	if a.exportPicking {
		content = a.renderExportPicker(contentHeight)
	}

	content = lipgloss.NewStyle().
		Width(a.width).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (a App) renderHeader() string {
	var tabs []string
	for i, name := range viewNames {
		if viewState(i) == a.activeView {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(name))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	title := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("trackr")
	gap := a.width - lipgloss.Width(title) - lipgloss.Width(tabRow) - 4
	if gap < 1 {
		gap = 1
	}
	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return headerStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Bottom, title, spacer, tabRow),
	)
}

func (a App) renderFooter() string {
	helpView := a.help.View(keys)

	status := ""
	if a.status != "" {
		status = mutedStyle.Render(" " + a.status)
	}

	// Timer indicator in footer
	timerInfo := ""
	if a.dashboard.isRunning() {
		elapsed := a.dashboard.elapsed()
		timerInfo = successStyle.Render(" ● " + formatDuration(elapsed))
		if a.dashboard.isPaused() {
			timerInfo = warningStyle.Render(" ⏸ " + formatDuration(elapsed))
		}
	}

	left := footerStyle.Render(helpView)
	right := timerInfo + status

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Bottom, left, spacer, right)
}

func (a App) renderExportPicker(_ int) string {
	title := titleStyle.Render("Export Format")
	formats := []string{"CSV", "JSON"}
	var rows []string
	rows = append(rows, title)
	rows = append(rows, "")
	for i, f := range formats {
		cursor := "  "
		style := normalItemStyle
		if i == a.exportCursor {
			cursor = "> "
			style = selectedItemStyle
		}
		rows = append(rows, style.Render(cursor+f))
	}
	rows = append(rows, "")
	rows = append(rows, mutedStyle.Render("  enter: export  esc: cancel"))

	w := a.width - 4
	return activePanelStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (a App) updateExportPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if a.exportCursor > 0 {
			a.exportCursor--
		}
	case key.Matches(msg, keys.Down):
		if a.exportCursor < 1 {
			a.exportCursor++
		}
	case key.Matches(msg, keys.Enter):
		a.exportPicking = false
		return a, a.doExport(a.exportCursor)
	case key.Matches(msg, keys.Back):
		a.exportPicking = false
	}
	return a, nil
}

func (a App) doExport(format int) tea.Cmd {
	return func() tea.Msg {
		entries, err := a.store.ListEntries(store.EntryFilter{})
		if err != nil {
			return statusMsg{text: fmt.Sprintf("Export error: %v", err), isError: true}
		}

		// Build project lookup
		projects := make(map[int64]*store.Project)
		plist, _ := a.store.ListProjects(true)
		for i := range plist {
			projects[plist[i].ID] = &plist[i]
		}

		home, _ := os.UserHomeDir()
		dateStr := time.Now().Format("2006-01-02")

		var path string
		if format == 0 {
			path = filepath.Join(home, fmt.Sprintf("trackr-export-%s.csv", dateStr))
			if err := export.ToCSV(entries, projects, path); err != nil {
				return statusMsg{text: fmt.Sprintf("CSV error: %v", err), isError: true}
			}
		} else {
			path = filepath.Join(home, fmt.Sprintf("trackr-export-%s.json", dateStr))
			if err := export.ToJSON(entries, projects, path); err != nil {
				return statusMsg{text: fmt.Sprintf("JSON error: %v", err), isError: true}
			}
		}

		return exportDoneMsg{path: path}
	}
}
