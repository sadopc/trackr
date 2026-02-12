package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	colorPrimary   = lipgloss.Color("#6C63FF")
	colorSecondary = lipgloss.Color("#2EC4B6")
	colorAccent    = lipgloss.Color("#FF6B6B")
	colorMuted     = lipgloss.Color("#666666")
	colorSuccess   = lipgloss.Color("#2ECC71")
	colorWarning   = lipgloss.Color("#F39C12")
	colorError     = lipgloss.Color("#E74C3C")
	colorBg        = lipgloss.Color("#1A1B26")
	colorFg        = lipgloss.Color("#C0CAF5")
	colorSubtle    = lipgloss.Color("#414868")
	colorHighlight = lipgloss.Color("#7AA2F7")
)

// Styles
var (
	// Tabs
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorPrimary).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	// Panels
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(1, 2)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2)

	// Timer
	timerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Align(lipgloss.Center)

	timerRunningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSuccess).
				Align(lipgloss.Center)

	timerPausedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning).
				Align(lipgloss.Center)

	// Text
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	highlightStyle = lipgloss.NewStyle().
			Foreground(colorHighlight)

	// Header/footer
	headerStyle = lipgloss.NewStyle().
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Status
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// List items
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(colorFg)
)
