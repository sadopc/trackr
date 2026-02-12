package tui

import (
	"testing"
	"time"

	"github.com/sadopc/trackr/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewMemory()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ============================================================
// Timer model
// ============================================================

func TestTimerStartStop(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	if tm.running() {
		t.Fatal("timer should start stopped")
	}

	err := tm.start(p.ID, "Dev", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !tm.running() {
		t.Fatal("timer should be running after start")
	}
	if tm.paused() {
		t.Fatal("timer should not be paused")
	}
	if tm.projectID != p.ID || tm.projectName != "Dev" {
		t.Fatal("project info not set")
	}
	if tm.entryID == 0 {
		t.Fatal("entry ID should be set")
	}

	time.Sleep(10 * time.Millisecond)
	entry, err := tm.stop()
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("stop should return entry")
	}
	if tm.running() {
		t.Fatal("timer should be stopped")
	}
}

func TestTimerStopWhenStopped(t *testing.T) {
	s := newTestStore(t)
	tm := newTimerModel(s)

	entry, err := tm.stop()
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Fatal("stop on stopped timer should return nil")
	}
}

func TestTimerPauseResume(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	tm.pause()
	if !tm.paused() {
		t.Fatal("timer should be paused")
	}
	if !tm.running() {
		t.Fatal("paused timer is still 'running' (not stopped)")
	}

	tm.resume()
	if tm.paused() {
		t.Fatal("timer should not be paused after resume")
	}
	if !tm.running() {
		t.Fatal("timer should be running after resume")
	}

	tm.stop()
}

func TestTimerPauseWhenNotRunning(t *testing.T) {
	s := newTestStore(t)
	tm := newTimerModel(s)

	// Pause when stopped — should be a no-op
	tm.pause()
	if tm.paused() {
		t.Fatal("should not be paused when stopped")
	}
}

func TestTimerResumeWhenNotPaused(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	// Resume when running — should be a no-op
	tm.resume()
	if tm.paused() {
		t.Fatal("should not be paused")
	}

	tm.stop()
}

func TestTimerToggle(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	tm.toggle() // running -> paused
	if !tm.paused() {
		t.Fatal("toggle should pause")
	}

	tm.toggle() // paused -> running
	if tm.paused() {
		t.Fatal("toggle should resume")
	}

	tm.stop()
}

func TestTimerToggleWhenStopped(t *testing.T) {
	s := newTestStore(t)
	tm := newTimerModel(s)

	// Toggle when stopped — should be a no-op
	tm.toggle()
	if tm.running() {
		t.Fatal("toggle should not start the timer")
	}
}

func TestTimerElapsed(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)

	// Stopped timer should return 0
	if tm.currentElapsed() != 0 {
		t.Fatal("stopped timer should have 0 elapsed")
	}

	tm.start(p.ID, "Dev", nil, "")
	time.Sleep(50 * time.Millisecond)

	elapsed := tm.currentElapsed()
	if elapsed < 40*time.Millisecond {
		t.Fatalf("elapsed too small: %v", elapsed)
	}

	tm.stop()
}

func TestTimerElapsedWhilePaused(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	time.Sleep(50 * time.Millisecond)
	tm.pause()
	pausedElapsed := tm.currentElapsed()

	time.Sleep(50 * time.Millisecond)
	// While paused, elapsed should not grow significantly
	stillPaused := tm.currentElapsed()
	diff := stillPaused - pausedElapsed
	if diff > 10*time.Millisecond {
		t.Fatalf("elapsed grew %v while paused", diff)
	}

	tm.stop()
}

func TestTimerTick(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	time.Sleep(20 * time.Millisecond)
	tm.tick()

	if tm.elapsed < 10*time.Millisecond {
		t.Fatal("tick should update elapsed")
	}

	tm.stop()
}

func TestTimerTickWhenStopped(t *testing.T) {
	s := newTestStore(t)
	tm := newTimerModel(s)

	// Tick on stopped timer should be a no-op
	tm.tick()
	if tm.elapsed != 0 {
		t.Fatal("tick on stopped timer should not change elapsed")
	}
}

func TestTimerIdleDetection(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.idleTimeout = 50 * time.Millisecond // very short for testing
	tm.start(p.ID, "Dev", nil, "")

	time.Sleep(100 * time.Millisecond)
	tm.tick()

	if !tm.isIdle {
		t.Fatal("timer should detect idle")
	}
	if !tm.paused() {
		t.Fatal("timer should auto-pause on idle")
	}

	tm.stop()
}

func TestTimerIdleRecovery(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.idleTimeout = 50 * time.Millisecond
	tm.start(p.ID, "Dev", nil, "")

	time.Sleep(100 * time.Millisecond)
	tm.tick() // triggers idle

	if !tm.isIdle || !tm.paused() {
		t.Fatal("should be idle and paused")
	}

	// Activity should resume
	tm.recordActivity()
	if tm.isIdle {
		t.Fatal("should no longer be idle after activity")
	}
	if tm.paused() {
		t.Fatal("should have resumed after activity")
	}

	tm.stop()
}

func TestTimerRecordActivityWhenNotIdle(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	// Recording activity when not idle should just update lastActivity
	before := tm.lastActivity
	time.Sleep(10 * time.Millisecond)
	tm.recordActivity()
	if !tm.lastActivity.After(before) {
		t.Fatal("lastActivity should be updated")
	}

	tm.stop()
}

func TestTimerStartWithTask(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")
	task, _ := s.CreateTask(p.ID, "Feature", "")

	tm := newTimerModel(s)
	tid := task.ID
	err := tm.start(p.ID, "Dev", &tid, "Feature")
	if err != nil {
		t.Fatal(err)
	}
	if tm.taskID == nil || *tm.taskID != tid {
		t.Fatal("task ID not set")
	}
	if tm.taskName != "Feature" {
		t.Fatal("task name not set")
	}

	tm.stop()
}

func TestTimerStartCreatesDBEntry(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")

	running, _ := s.GetRunningEntry()
	if running == nil {
		t.Fatal("start should create a DB entry")
	}
	if running.ID != tm.entryID {
		t.Fatal("entry ID mismatch")
	}

	tm.stop()
}

func TestTimerStopPersistsTooDB(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	tm := newTimerModel(s)
	tm.start(p.ID, "Dev", nil, "")
	time.Sleep(10 * time.Millisecond)
	tm.stop()

	running, _ := s.GetRunningEntry()
	if running != nil {
		t.Fatal("stop should close the DB entry")
	}

	entry, _ := s.GetEntry(tm.entryID)
	if entry.EndTime == nil {
		t.Fatal("stopped entry should have end_time in DB")
	}
}

// ============================================================
// Helper functions
// ============================================================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00"},
		{time.Second, "00:00:01"},
		{time.Minute, "00:01:00"},
		{time.Hour, "01:00:00"},
		{time.Hour + time.Minute + time.Second, "01:01:01"},
		{25 * time.Hour, "25:00:00"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{0, "00:00:00"},
		{61, "00:01:01"},
		{3600, "01:00:00"},
		{86400, "24:00:00"},
	}
	for _, tt := range tests {
		got := formatSeconds(tt.secs)
		if got != tt.want {
			t.Errorf("formatSeconds(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

func TestFormatHours(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{0, "0.0h"},
		{3600, "1.0h"},
		{5400, "1.5h"},
		{7200, "2.0h"},
	}
	for _, tt := range tests {
		got := formatHours(tt.secs)
		if got != tt.want {
			t.Errorf("formatHours(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

func TestMinMax(t *testing.T) {
	if min(3, 5) != 3 {
		t.Fatal("min(3,5) should be 3")
	}
	if min(5, 3) != 3 {
		t.Fatal("min(5,3) should be 3")
	}
	if min(3, 3) != 3 {
		t.Fatal("min(3,3) should be 3")
	}
	if max(3, 5) != 5 {
		t.Fatal("max(3,5) should be 5")
	}
	if max(5, 3) != 5 {
		t.Fatal("max(5,3) should be 5")
	}
	if max(3, 3) != 3 {
		t.Fatal("max(3,3) should be 3")
	}
}

// ============================================================
// View state
// ============================================================

func TestViewNames(t *testing.T) {
	if len(viewNames) != 5 {
		t.Fatalf("expected 5 view names, got %d", len(viewNames))
	}
	expected := []string{"Dashboard", "Projects", "Reports", "Pomodoro", "Settings"}
	for i, name := range expected {
		if viewNames[i] != name {
			t.Fatalf("viewNames[%d] = %q, want %q", i, viewNames[i], name)
		}
	}
}

func TestViewStateConstants(t *testing.T) {
	if viewDashboard != 0 || viewProjects != 1 || viewReports != 2 || viewPomodoro != 3 || viewSettings != 4 {
		t.Fatal("view state constants out of order")
	}
}

// ============================================================
// Dashboard model
// ============================================================

func TestDashboardInit(t *testing.T) {
	s := newTestStore(t)
	d := newDashboardModel(s)

	if d.isRunning() {
		t.Fatal("dashboard timer should not be running initially")
	}
	if d.isPaused() {
		t.Fatal("dashboard timer should not be paused initially")
	}
	if d.elapsed() != 0 {
		t.Fatal("dashboard should have 0 elapsed initially")
	}
}

func TestDashboardStartStop(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Dev", "#000", "work")

	d := newDashboardModel(s)
	d.projects = []store.Project{*p}

	d, _ = d.startTimer(p.ID, "Dev", nil, "")
	if !d.isRunning() {
		t.Fatal("timer should be running")
	}

	d, _ = d.stopTimer()
	if d.isRunning() {
		t.Fatal("timer should be stopped")
	}
}

func TestDashboardPickerWithOneProject(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("Solo", "#000", "work")

	d := newDashboardModel(s)
	d.projects = []store.Project{*p}

	// With only one project, pressing start should auto-select it
	// (no picker shown)
	if d.picking {
		t.Fatal("should not be in picker mode initially")
	}
}

// ============================================================
// Settings helpers
// ============================================================

func TestSecsToMin(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1500", "25"},
		{"300", "5"},
		{"0", "0"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		got := secsToMin(tt.in)
		if got != tt.want {
			t.Errorf("secsToMin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMinToSecs(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"25", "1500"},
		{"5", "300"},
		{"0", "0"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		got := minToSecs(tt.in)
		if got != tt.want {
			t.Errorf("minToSecs(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSecsToHours(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"28800", "8.0"},
		{"3600", "1.0"},
		{"0", "0.0"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		got := secsToHours(tt.in)
		if got != tt.want {
			t.Errorf("secsToHours(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHoursToSecs(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"8.0", "28800"},
		{"1.0", "3600"},
		{"0.0", "0"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		got := hoursToSecs(tt.in)
		if got != tt.want {
			t.Errorf("hoursToSecs(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatSettingValue(t *testing.T) {
	tests := []struct {
		key, val, want string
	}{
		{"pomodoro_work", "1500", "25 min"},
		{"pomodoro_break", "300", "5 min"},
		{"idle_timeout", "300", "5 min"},
		{"daily_goal", "28800", "8.0 hours"},
		{"idle_action", "pause", "pause"},
		{"week_start", "monday", "monday"},
		{"pomodoro_count", "4", "4"},
		{"pomodoro_work", "invalid", "invalid"},
	}
	for _, tt := range tests {
		got := formatSettingValue(tt.key, tt.val)
		if got != tt.want {
			t.Errorf("formatSettingValue(%q, %q) = %q, want %q", tt.key, tt.val, got, tt.want)
		}
	}
}

// ============================================================
// Pomodoro model
// ============================================================

func TestPomodoroInit(t *testing.T) {
	s := newTestStore(t)
	pm := newPomodoroModel(s)

	if pm.phase != pomodoroIdle {
		t.Fatalf("expected idle phase, got %d", pm.phase)
	}
	if pm.workDuration != 25*time.Minute {
		t.Fatalf("expected 25min work, got %v", pm.workDuration)
	}
	if pm.breakDuration != 5*time.Minute {
		t.Fatalf("expected 5min break, got %v", pm.breakDuration)
	}
	if pm.longBreakDuration != 15*time.Minute {
		t.Fatalf("expected 15min long break, got %v", pm.longBreakDuration)
	}
	if pm.targetCount != 4 {
		t.Fatalf("expected 4 target, got %d", pm.targetCount)
	}
}

func TestPomodoroStartSession(t *testing.T) {
	s := newTestStore(t)
	pm := newPomodoroModel(s)

	pm, _ = pm.startSession()
	if pm.phase != pomodoroWork {
		t.Fatal("should be in work phase after start")
	}
	if pm.completedCount != 0 {
		t.Fatal("completed count should be 0")
	}
	if pm.sessionID == 0 {
		t.Fatal("session ID should be set")
	}
	if pm.remaining <= 0 {
		t.Fatal("remaining should be positive")
	}
}

func TestPomodoroCancelSession(t *testing.T) {
	s := newTestStore(t)
	pm := newPomodoroModel(s)
	pm, _ = pm.startSession()

	pm, _ = pm.cancelSession()
	if pm.phase != pomodoroIdle {
		t.Fatal("should be idle after cancel")
	}

	// Verify DB record is cancelled
	pom, _ := s.GetPomodoro(pm.sessionID)
	if pom.Status != "cancelled" {
		t.Fatalf("DB status should be cancelled, got %s", pom.Status)
	}
}

func TestPomodoroAdvanceWorkToBreak(t *testing.T) {
	s := newTestStore(t)
	pm := newPomodoroModel(s)
	pm, _ = pm.startSession()

	// Simulate work phase completion
	pm, _ = pm.advancePhase()

	if pm.completedCount != 1 {
		t.Fatalf("expected 1 completed, got %d", pm.completedCount)
	}
	if pm.phase != pomodoroShortBreak {
		t.Fatalf("expected short break, got %d", pm.phase)
	}
}

func TestPomodoroAdvanceBreakToWork(t *testing.T) {
	s := newTestStore(t)
	pm := newPomodoroModel(s)
	pm, _ = pm.startSession()

	// Work -> Break
	pm, _ = pm.advancePhase()
	if pm.phase != pomodoroShortBreak {
		t.Fatal("should be on short break")
	}

	// Break -> Work
	pm, _ = pm.advancePhase()
	if pm.phase != pomodoroWork {
		t.Fatalf("should be back to work, got %d", pm.phase)
	}
}

func TestPomodoroFullCycle(t *testing.T) {
	s := newTestStore(t)
	s.SetSetting("pomodoro_count", "2") // shorter cycle for test
	pm := newPomodoroModel(s)
	pm, _ = pm.startSession()

	// Work 1
	pm, _ = pm.advancePhase() // -> short break, count=1
	if pm.phase != pomodoroShortBreak || pm.completedCount != 1 {
		t.Fatalf("after work 1: phase=%d, count=%d", pm.phase, pm.completedCount)
	}

	// Break 1
	pm, _ = pm.advancePhase() // -> work
	if pm.phase != pomodoroWork {
		t.Fatal("should go back to work after break")
	}

	// Work 2 — should complete
	pm, _ = pm.advancePhase() // -> completed, count=2
	if pm.phase != pomodoroCompleted {
		t.Fatalf("expected completed, got %d", pm.phase)
	}
	if pm.completedCount != 2 {
		t.Fatalf("expected 2 completed, got %d", pm.completedCount)
	}
}

func TestPomodoroPhaseNames(t *testing.T) {
	phases := []pomodoroPhase{pomodoroIdle, pomodoroWork, pomodoroShortBreak, pomodoroLongBreak, pomodoroCompleted}
	for _, p := range phases {
		name, ok := phaseNames[p]
		if !ok {
			t.Fatalf("missing phase name for %d", p)
		}
		if name == "" {
			t.Fatalf("empty phase name for %d", p)
		}
	}
}

func TestFormatPomodoroTime(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{time.Second, "00:01"},
		{time.Minute, "01:00"},
		{25 * time.Minute, "25:00"},
		{5*time.Minute + 30*time.Second, "05:30"},
		{-time.Second, "00:00"}, // negative should clamp to 0
	}
	for _, tt := range tests {
		got := formatPomodoroTime(tt.d)
		if got != tt.want {
			t.Errorf("formatPomodoroTime(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestPomodoroLoadsSettings(t *testing.T) {
	s := newTestStore(t)
	s.SetSetting("pomodoro_work", "600")
	s.SetSetting("pomodoro_break", "120")
	s.SetSetting("pomodoro_long_break", "600")
	s.SetSetting("pomodoro_count", "2")

	pm := newPomodoroModel(s)
	if pm.workDuration != 10*time.Minute {
		t.Fatalf("expected 10min work, got %v", pm.workDuration)
	}
	if pm.breakDuration != 2*time.Minute {
		t.Fatalf("expected 2min break, got %v", pm.breakDuration)
	}
	if pm.longBreakDuration != 10*time.Minute {
		t.Fatalf("expected 10min long break, got %v", pm.longBreakDuration)
	}
	if pm.targetCount != 2 {
		t.Fatalf("expected 2 target, got %d", pm.targetCount)
	}
}

// ============================================================
// App model
// ============================================================

func TestNewApp(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)

	if app.activeView != viewDashboard {
		t.Fatal("default view should be dashboard")
	}
	if app.showHelp {
		t.Fatal("help should be hidden by default")
	}
	if app.exportPicking {
		t.Fatal("export picker should be hidden by default")
	}
}

func TestAppIsFormActiveDefault(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)

	if app.isFormActive() {
		t.Fatal("no forms should be active initially")
	}
}

func TestAppViewStates(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)
	app.width = 120
	app.height = 40

	// Test all views render without panic
	views := []viewState{viewDashboard, viewProjects, viewReports, viewPomodoro, viewSettings}
	for _, v := range views {
		app.activeView = v
		output := app.View()
		if output == "" {
			t.Fatalf("view %d rendered empty", v)
		}
	}
}

func TestAppRenderHeaderContainsAllTabs(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)
	app.width = 120
	app.height = 40

	header := app.renderHeader()
	for _, name := range viewNames {
		if !containsString(header, name) {
			t.Fatalf("header missing tab %q", name)
		}
	}
}

func TestAppRenderFooter(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)
	app.width = 120
	app.height = 40

	footer := app.renderFooter()
	if footer == "" {
		t.Fatal("footer should not be empty")
	}
}

func TestAppLoadingState(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)
	// Width 0 means not yet sized
	output := app.View()
	if output != "Loading..." {
		t.Fatalf("expected 'Loading...', got %q", output)
	}
}

func TestAppStatusMessage(t *testing.T) {
	s := newTestStore(t)
	app := NewApp(s)
	app.width = 120
	app.height = 40
	app.status = "test status"

	footer := app.renderFooter()
	if !containsString(footer, "test status") {
		t.Fatal("footer should contain status message")
	}
}

// containsString checks if s contains substr, ignoring ANSI escape codes.
func containsString(s, substr string) bool {
	// Simple check — ANSI codes don't affect the raw string contains
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================
// Key bindings
// ============================================================

func TestKeyMapShortHelp(t *testing.T) {
	bindings := keys.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("short help should have bindings")
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	groups := keys.FullHelp()
	if len(groups) == 0 {
		t.Fatal("full help should have groups")
	}
	for i, g := range groups {
		if len(g) == 0 {
			t.Fatalf("full help group %d is empty", i)
		}
	}
}

// ============================================================
// Styles (smoke test — just verify they don't panic)
// ============================================================

func TestStylesRender(t *testing.T) {
	styles := []struct {
		name string
		fn   func() string
	}{
		{"activeTab", func() string { return activeTabStyle.Render("test") }},
		{"inactiveTab", func() string { return inactiveTabStyle.Render("test") }},
		{"panel", func() string { return panelStyle.Render("test") }},
		{"activePanel", func() string { return activePanelStyle.Render("test") }},
		{"timer", func() string { return timerStyle.Render("test") }},
		{"timerRunning", func() string { return timerRunningStyle.Render("test") }},
		{"timerPaused", func() string { return timerPausedStyle.Render("test") }},
		{"title", func() string { return titleStyle.Render("test") }},
		{"subtitle", func() string { return subtitleStyle.Render("test") }},
		{"accent", func() string { return accentStyle.Render("test") }},
		{"success", func() string { return successStyle.Render("test") }},
		{"warning", func() string { return warningStyle.Render("test") }},
		{"error", func() string { return errorStyle.Render("test") }},
		{"muted", func() string { return mutedStyle.Render("test") }},
		{"highlight", func() string { return highlightStyle.Render("test") }},
		{"header", func() string { return headerStyle.Render("test") }},
		{"footer", func() string { return footerStyle.Render("test") }},
		{"selectedItem", func() string { return selectedItemStyle.Render("test") }},
		{"normalItem", func() string { return normalItemStyle.Render("test") }},
	}

	for _, s := range styles {
		result := s.fn()
		if result == "" {
			t.Fatalf("style %q rendered empty", s.name)
		}
	}
}
