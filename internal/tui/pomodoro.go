package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/store"
)

type pomodoroPhase int

const (
	pomodoroIdle pomodoroPhase = iota
	pomodoroWork
	pomodoroShortBreak
	pomodoroLongBreak
	pomodoroCompleted
)

var phaseNames = map[pomodoroPhase]string{
	pomodoroIdle:       "IDLE",
	pomodoroWork:       "WORK",
	pomodoroShortBreak: "SHORT BREAK",
	pomodoroLongBreak:  "LONG BREAK",
	pomodoroCompleted:  "COMPLETED",
}

type pomodoroModel struct {
	store  *store.Store
	width  int
	height int

	phase          pomodoroPhase
	completedCount int
	targetCount    int

	// Countdown state
	remaining time.Duration
	phaseEnd  time.Time

	// Durations from settings
	workDuration      time.Duration
	breakDuration     time.Duration
	longBreakDuration time.Duration

	sessionID int64 // pomodoro_sessions.id
	entryID   *int64

	formActive bool
}

func newPomodoroModel(s *store.Store) pomodoroModel {
	m := pomodoroModel{
		store:       s,
		phase:       pomodoroIdle,
		targetCount: 4,
	}
	m.loadSettings()
	return m
}

func (p *pomodoroModel) loadSettings() {
	p.workDuration = p.getSettingDuration("pomodoro_work", 25*time.Minute)
	p.breakDuration = p.getSettingDuration("pomodoro_break", 5*time.Minute)
	p.longBreakDuration = p.getSettingDuration("pomodoro_long_break", 15*time.Minute)

	if v, err := p.store.GetSetting("pomodoro_count"); err == nil {
		if n, err := strconv.Atoi(v); err == nil {
			p.targetCount = n
		}
	}
}

func (p *pomodoroModel) getSettingDuration(key string, fallback time.Duration) time.Duration {
	if v, err := p.store.GetSetting(key); err == nil {
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}

func (p *pomodoroModel) setSize(w, h int) {
	p.width = w
	p.height = h
}

func (p pomodoroModel) update(msg tea.Msg) (pomodoroModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if p.phase == pomodoroWork || p.phase == pomodoroShortBreak || p.phase == pomodoroLongBreak {
			p.remaining = time.Until(p.phaseEnd)
			if p.remaining <= 0 {
				return p.advancePhase()
			}
		}
		return p, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Start):
			if p.phase == pomodoroIdle || p.phase == pomodoroCompleted {
				return p.startSession()
			}
		case key.Matches(msg, keys.Stop):
			if p.phase != pomodoroIdle {
				return p.cancelSession()
			}
		case key.Matches(msg, keys.Pause):
			// Skip break
			if p.phase == pomodoroShortBreak || p.phase == pomodoroLongBreak {
				return p.startWorkPhase()
			}
		}
	}
	return p, nil
}

func (p pomodoroModel) startSession() (pomodoroModel, tea.Cmd) {
	p.completedCount = 0
	p.loadSettings()

	session, err := p.store.StartPomodoro(nil,
		int(p.workDuration.Seconds()),
		int(p.breakDuration.Seconds()),
		p.targetCount,
	)
	if err != nil {
		return p, func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("Error: %v", err), isError: true}
		}
	}
	p.sessionID = session.ID

	return p.startWorkPhase()
}

func (p pomodoroModel) startWorkPhase() (pomodoroModel, tea.Cmd) {
	p.phase = pomodoroWork
	p.remaining = p.workDuration
	p.phaseEnd = time.Now().Add(p.workDuration)
	if p.sessionID > 0 {
		p.store.UpdatePomodoroStatus(p.sessionID, "working")
	}
	return p, nil
}

func (p pomodoroModel) advancePhase() (pomodoroModel, tea.Cmd) {
	switch p.phase {
	case pomodoroWork:
		p.completedCount++
		if p.sessionID > 0 {
			p.store.IncrementPomodoro(p.sessionID)
		}

		if p.completedCount >= p.targetCount {
			p.phase = pomodoroCompleted
			if p.sessionID > 0 {
				p.store.CompletePomodoro(p.sessionID)
			}
			return p, func() tea.Msg {
				return statusMsg{text: "Pomodoro session complete! \a"}
			}
		}

		// Every 4th pomodoro gets a long break
		if p.completedCount%p.targetCount == 0 {
			p.phase = pomodoroLongBreak
			p.remaining = p.longBreakDuration
			p.phaseEnd = time.Now().Add(p.longBreakDuration)
		} else {
			p.phase = pomodoroShortBreak
			p.remaining = p.breakDuration
			p.phaseEnd = time.Now().Add(p.breakDuration)
		}
		if p.sessionID > 0 {
			p.store.UpdatePomodoroStatus(p.sessionID, string(phaseNames[p.phase]))
		}
		return p, func() tea.Msg {
			return statusMsg{text: "Break time! \a"}
		}

	case pomodoroShortBreak, pomodoroLongBreak:
		return p.startWorkPhase()
	}
	return p, nil
}

func (p pomodoroModel) cancelSession() (pomodoroModel, tea.Cmd) {
	if p.sessionID > 0 {
		p.store.CancelPomodoro(p.sessionID)
	}
	p.phase = pomodoroIdle
	p.remaining = 0
	return p, func() tea.Msg {
		return statusMsg{text: "Pomodoro cancelled"}
	}
}

func (p pomodoroModel) view() string {
	w := p.width - 4

	title := titleStyle.Render("Pomodoro Timer")

	// Big countdown display
	var timeDisplay string
	var phaseLabel string
	var indicator string

	switch p.phase {
	case pomodoroIdle:
		timeDisplay = timerStyle.Width(w - 6).Render(formatPomodoroTime(p.workDuration))
		phaseLabel = mutedStyle.Render("Ready to start")
		indicator = mutedStyle.Render("Press s to begin")
	case pomodoroWork:
		timeDisplay = accentStyle.Bold(true).Width(w - 6).Align(lipgloss.Center).Render(formatPomodoroTime(p.remaining))
		phaseLabel = accentStyle.Bold(true).Render("WORK")
		indicator = p.renderProgress()
	case pomodoroShortBreak:
		timeDisplay = successStyle.Bold(true).Width(w - 6).Align(lipgloss.Center).Render(formatPomodoroTime(p.remaining))
		phaseLabel = successStyle.Bold(true).Render("SHORT BREAK")
		indicator = p.renderProgress()
	case pomodoroLongBreak:
		timeDisplay = highlightStyle.Bold(true).Width(w - 6).Align(lipgloss.Center).Render(formatPomodoroTime(p.remaining))
		phaseLabel = highlightStyle.Bold(true).Render("LONG BREAK")
		indicator = p.renderProgress()
	case pomodoroCompleted:
		timeDisplay = successStyle.Bold(true).Width(w - 6).Align(lipgloss.Center).Render("Done!")
		phaseLabel = successStyle.Bold(true).Render("SESSION COMPLETE")
		indicator = p.renderProgress()
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		timeDisplay,
		phaseLabel,
		"",
		indicator,
	)

	// Controls
	var controls string
	switch p.phase {
	case pomodoroIdle, pomodoroCompleted:
		controls = mutedStyle.Render("s: start  q: quit")
	case pomodoroWork:
		controls = mutedStyle.Render("x: cancel")
	case pomodoroShortBreak, pomodoroLongBreak:
		controls = mutedStyle.Render("space: skip break  x: cancel")
	}

	return panelStyle.Width(w).Render(
		lipgloss.JoinVertical(lipgloss.Center, content, "", controls),
	)
}

func (p pomodoroModel) renderProgress() string {
	var parts []string
	for i := 0; i < p.targetCount; i++ {
		if i < p.completedCount {
			parts = append(parts, successStyle.Render("●"))
		} else if i == p.completedCount && p.phase == pomodoroWork {
			parts = append(parts, accentStyle.Render("◐"))
		} else {
			parts = append(parts, mutedStyle.Render("○"))
		}
	}
	progress := strings.Join(parts, " ")
	counter := mutedStyle.Render(fmt.Sprintf("  %d/%d", p.completedCount, p.targetCount))
	return progress + counter
}

func formatPomodoroTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
