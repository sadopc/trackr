package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/store"
)

type settingsModel struct {
	store  *store.Store
	width  int
	height int

	settings   []store.Setting
	formActive bool
	form       *huh.Form

	// Form values as pointers (survive value copies)
	pomodoroWork      *string
	pomodoroBreak     *string
	pomodoroLongBreak *string
	pomodoroCount     *string
	idleTimeout       *string
	idleAction        *string
	dailyGoal         *string
	weekStart         *string
}

func newSettingsModel(s *store.Store) settingsModel {
	pw, pb, plb, pc := "", "", "", ""
	it, ia, dg, ws := "", "", "", ""
	return settingsModel{
		store:             s,
		pomodoroWork:      &pw,
		pomodoroBreak:     &pb,
		pomodoroLongBreak: &plb,
		pomodoroCount:     &pc,
		idleTimeout:       &it,
		idleAction:        &ia,
		dailyGoal:         &dg,
		weekStart:         &ws,
	}
}

func (s *settingsModel) setSize(w, h int) {
	s.width = w
	s.height = h
}

type settingsDataMsg struct {
	settings []store.Setting
}

func (s settingsModel) refresh() tea.Cmd {
	return func() tea.Msg {
		settings, _ := s.store.GetAllSettings()
		return settingsDataMsg{settings: settings}
	}
}

func (s settingsModel) update(msg tea.Msg) (settingsModel, tea.Cmd) {
	if s.formActive && s.form != nil {
		return s.updateForm(msg)
	}

	switch msg := msg.(type) {
	case settingsDataMsg:
		s.settings = msg.settings
		return s, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Enter), key.Matches(msg, keys.New):
			return s.showForm()
		}
	}
	return s, nil
}

func (s settingsModel) showForm() (settingsModel, tea.Cmd) {
	// Load current values
	*s.pomodoroWork = secsToMin(s.getVal("pomodoro_work", "1500"))
	*s.pomodoroBreak = secsToMin(s.getVal("pomodoro_break", "300"))
	*s.pomodoroLongBreak = secsToMin(s.getVal("pomodoro_long_break", "900"))
	*s.pomodoroCount = s.getVal("pomodoro_count", "4")
	*s.idleTimeout = secsToMin(s.getVal("idle_timeout", "300"))
	*s.idleAction = s.getVal("idle_action", "pause")
	*s.dailyGoal = secsToHours(s.getVal("daily_goal", "28800"))
	*s.weekStart = s.getVal("week_start", "monday")

	s.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Pomodoro work (min)").Value(s.pomodoroWork),
			huh.NewInput().Title("Pomodoro break (min)").Value(s.pomodoroBreak),
			huh.NewInput().Title("Long break (min)").Value(s.pomodoroLongBreak),
			huh.NewInput().Title("Pomodoros before long break").Value(s.pomodoroCount),
		).Title("Pomodoro"),
		huh.NewGroup(
			huh.NewInput().Title("Idle timeout (min)").Value(s.idleTimeout),
			huh.NewSelect[string]().Title("Idle action").
				Options(
					huh.NewOption("Pause", "pause"),
					huh.NewOption("Stop", "stop"),
				).Value(s.idleAction),
			huh.NewInput().Title("Daily goal (hours)").Value(s.dailyGoal),
			huh.NewSelect[string]().Title("Week starts on").
				Options(
					huh.NewOption("Monday", "monday"),
					huh.NewOption("Sunday", "sunday"),
				).Value(s.weekStart),
		).Title("General"),
	).WithShowHelp(true).WithShowErrors(true)

	s.formActive = true
	return s, s.form.Init()
}

func (s settingsModel) updateForm(msg tea.Msg) (settingsModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "esc" {
			s.formActive = false
			s.form = nil
			return s, nil
		}
	}

	form, cmd := s.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		s.form = f
	}

	if s.form.State == huh.StateCompleted {
		s.formActive = false
		s.saveSettings()
		return s, s.refresh()
	}

	return s, cmd
}

func (s settingsModel) saveSettings() {
	s.store.SetSetting("pomodoro_work", minToSecs(*s.pomodoroWork))
	s.store.SetSetting("pomodoro_break", minToSecs(*s.pomodoroBreak))
	s.store.SetSetting("pomodoro_long_break", minToSecs(*s.pomodoroLongBreak))
	s.store.SetSetting("pomodoro_count", *s.pomodoroCount)
	s.store.SetSetting("idle_timeout", minToSecs(*s.idleTimeout))
	s.store.SetSetting("idle_action", *s.idleAction)
	s.store.SetSetting("daily_goal", hoursToSecs(*s.dailyGoal))
	s.store.SetSetting("week_start", *s.weekStart)
}

func (s settingsModel) getVal(k, fallback string) string {
	v, err := s.store.GetSetting(k)
	if err != nil {
		return fallback
	}
	return v
}

func (s settingsModel) view() string {
	w := s.width - 4

	if s.formActive && s.form != nil {
		title := titleStyle.Render("Settings")
		formView := s.form.View()
		return panelStyle.Width(w).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", formView),
		)
	}

	title := titleStyle.Render("Settings")
	hint := mutedStyle.Render("Press enter to edit settings")

	var rows []string
	rows = append(rows, title)
	rows = append(rows, "")

	for _, setting := range s.settings {
		label := lipgloss.NewStyle().Width(24).Render(setting.Key)
		value := highlightStyle.Render(formatSettingValue(setting.Key, setting.Value))
		rows = append(rows, fmt.Sprintf("  %s %s", label, value))
	}

	rows = append(rows, "")
	rows = append(rows, hint)

	return panelStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func formatSettingValue(k, v string) string {
	switch k {
	case "pomodoro_work", "pomodoro_break", "pomodoro_long_break", "idle_timeout":
		if secs, err := strconv.Atoi(v); err == nil {
			return fmt.Sprintf("%d min", secs/60)
		}
	case "daily_goal":
		if secs, err := strconv.Atoi(v); err == nil {
			return fmt.Sprintf("%.1f hours", float64(secs)/3600)
		}
	}
	return v
}

func secsToMin(s string) string {
	if secs, err := strconv.Atoi(s); err == nil {
		return strconv.Itoa(secs / 60)
	}
	return s
}

func minToSecs(s string) string {
	if mins, err := strconv.Atoi(s); err == nil {
		return strconv.Itoa(mins * 60)
	}
	return s
}

func secsToHours(s string) string {
	if secs, err := strconv.Atoi(s); err == nil {
		return fmt.Sprintf("%.1f", float64(secs)/3600)
	}
	return s
}

func hoursToSecs(s string) string {
	if hours, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.Itoa(int(hours * 3600))
	}
	return s
}
