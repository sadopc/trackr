package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/store"
)

type reportMode int

const (
	reportDaily reportMode = iota
	reportWeekly
)

type reportsModel struct {
	store  *store.Store
	width  int
	height int

	mode      reportMode
	summaries []store.DailySummary
	offset    int // weeks or 7-day blocks offset from today (0 = current)

	chart barchart.Model
}

func newReportsModel(s *store.Store) reportsModel {
	return reportsModel{
		store: s,
		chart: barchart.New(60, 12),
	}
}

func (r *reportsModel) setSize(w, h int) {
	r.width = w
	r.height = h
}

type reportsDataMsg struct {
	summaries []store.DailySummary
}

func (r reportsModel) refresh() tea.Cmd {
	return func() tea.Msg {
		from, to := r.dateRange()
		summaries, _ := r.store.GetDailySummary(from, to)
		return reportsDataMsg{summaries: summaries}
	}
}

func (r reportsModel) dateRange() (time.Time, time.Time) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch r.mode {
	case reportWeekly:
		// Start of current week (Monday)
		weekday := today.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		startOfWeek := today.AddDate(0, 0, -int(weekday-time.Monday))
		startOfWeek = startOfWeek.AddDate(0, 0, -7*r.offset)
		return startOfWeek, startOfWeek.AddDate(0, 0, 7)
	default:
		// Daily: last 7 days
		end := today.AddDate(0, 0, 1-7*r.offset)
		start := end.AddDate(0, 0, -7)
		return start, end
	}
}

func (r reportsModel) update(msg tea.Msg) (reportsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case reportsDataMsg:
		r.summaries = msg.summaries
		r.buildChart()
		return r, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Left):
			r.offset++
			return r, r.refresh()
		case key.Matches(msg, keys.Right):
			if r.offset > 0 {
				r.offset--
			}
			return r, r.refresh()
		case key.Matches(msg, keys.Tab):
			if r.mode == reportDaily {
				r.mode = reportWeekly
			} else {
				r.mode = reportDaily
			}
			r.offset = 0
			return r, r.refresh()
		}
	}
	return r, nil
}

func (r *reportsModel) buildChart() {
	chartWidth := r.width - 8
	if chartWidth < 20 {
		chartWidth = 20
	}
	chartHeight := 12
	if r.height > 30 {
		chartHeight = 16
	}

	r.chart = barchart.New(chartWidth, chartHeight)

	from, to := r.dateRange()

	// Build bars for each day in range
	var bars []barchart.BarData
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		label := d.Format("Mon 02")

		var values []barchart.BarValue
		for _, s := range r.summaries {
			if s.Date == dateStr {
				hours := float64(s.TotalSeconds) / 3600.0
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(s.ProjectColor))
				values = append(values, barchart.BarValue{
					Name:  s.ProjectName,
					Value: hours,
					Style: style,
				})
			}
		}

		if len(values) == 0 {
			values = []barchart.BarValue{{Name: "", Value: 0, Style: lipgloss.NewStyle().Foreground(colorSubtle)}}
		}

		bars = append(bars, barchart.BarData{
			Label:  label,
			Values: values,
		})
	}

	r.chart.PushAll(bars)
	r.chart.Draw()
}

func (r reportsModel) view() string {
	w := r.width - 4

	// Mode tabs
	dailyTab := inactiveTabStyle.Render("Daily")
	weeklyTab := inactiveTabStyle.Render("Weekly")
	if r.mode == reportDaily {
		dailyTab = activeTabStyle.Render("Daily")
	} else {
		weeklyTab = activeTabStyle.Render("Weekly")
	}
	modeTabs := lipgloss.JoinHorizontal(lipgloss.Bottom, dailyTab, weeklyTab)

	// Date range label
	from, to := r.dateRange()
	dateLabel := mutedStyle.Render(fmt.Sprintf("%s — %s", from.Format("Jan 02"), to.Add(-24*time.Hour).Format("Jan 02, 2006")))

	header := lipgloss.JoinHorizontal(lipgloss.Bottom,
		titleStyle.Render("Reports"), "  ", modeTabs, "  ", dateLabel,
	)

	// Chart
	chartView := r.chart.View()

	// Summary table
	tableView := r.renderSummaryTable(w)

	// Legend
	legend := r.renderLegend()

	nav := mutedStyle.Render("  ←/→: navigate  tab: switch mode")

	return panelStyle.Width(w).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header, "", chartView, "", legend, "", tableView, "", nav,
		),
	)
}

func (r reportsModel) renderSummaryTable(w int) string {
	if len(r.summaries) == 0 {
		return mutedStyle.Render("  No data for this period")
	}

	var rows []string
	headerRow := mutedStyle.Render(fmt.Sprintf("  %-12s %-20s %10s %8s", "Date", "Project", "Duration", "Entries"))
	rows = append(rows, headerRow)
	rows = append(rows, mutedStyle.Render("  "+strings.Repeat("─", min(w-6, 54))))

	for _, s := range r.summaries {
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(s.ProjectColor)).Render("●")
		rows = append(rows, fmt.Sprintf("  %-12s %s %-18s %10s %8d",
			s.Date, colorDot, s.ProjectName, formatSeconds(s.TotalSeconds), s.EntryCount,
		))
	}

	return strings.Join(rows, "\n")
}

func (r reportsModel) renderLegend() string {
	// Collect unique projects from summaries
	seen := make(map[int64]bool)
	var items []string
	for _, s := range r.summaries {
		if seen[s.ProjectID] {
			continue
		}
		seen[s.ProjectID] = true
		dot := lipgloss.NewStyle().Foreground(lipgloss.Color(s.ProjectColor)).Render("●")
		items = append(items, fmt.Sprintf("%s %s", dot, s.ProjectName))
	}
	if len(items) == 0 {
		return ""
	}
	return "  " + strings.Join(items, "  ")
}
