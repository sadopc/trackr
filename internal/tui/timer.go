package tui

import (
	"time"

	"github.com/sadopc/trackr/internal/store"
)

// timerState tracks the current state of the timer.
type timerState int

const (
	timerStopped timerState = iota
	timerRunning
	timerPaused
)

// timerModel manages the timing logic separate from display.
type timerModel struct {
	store *store.Store

	state     timerState
	startTime time.Time
	elapsed   time.Duration
	pausedAt  time.Time // when paused, to compute pause gap
	pauseGap  time.Duration

	projectID   int64
	projectName string
	taskID      *int64
	taskName    string
	entryID     int64

	// Idle detection
	lastActivity time.Time
	idleTimeout  time.Duration
	isIdle       bool
}

func newTimerModel(s *store.Store) timerModel {
	return timerModel{
		store:        s,
		state:        timerStopped,
		lastActivity: time.Now(),
		idleTimeout:  5 * time.Minute,
	}
}

func (t *timerModel) start(projectID int64, projectName string, taskID *int64, taskName string) error {
	entry, err := t.store.StartEntry(projectID, taskID)
	if err != nil {
		return err
	}
	t.state = timerRunning
	t.startTime = time.Now()
	t.elapsed = 0
	t.pauseGap = 0
	t.projectID = projectID
	t.projectName = projectName
	t.taskID = taskID
	t.taskName = taskName
	t.entryID = entry.ID
	t.lastActivity = time.Now()
	t.isIdle = false
	return nil
}

func (t *timerModel) stop() (*store.TimeEntry, error) {
	if t.state == timerStopped {
		return nil, nil
	}
	entry, err := t.store.StopEntry(t.entryID)
	if err != nil {
		return nil, err
	}
	t.state = timerStopped
	t.elapsed = 0
	return entry, nil
}

func (t *timerModel) pause() {
	if t.state != timerRunning {
		return
	}
	t.state = timerPaused
	t.pausedAt = time.Now()
}

func (t *timerModel) resume() {
	if t.state != timerPaused {
		return
	}
	t.pauseGap += time.Since(t.pausedAt)
	t.state = timerRunning
	t.isIdle = false
	t.lastActivity = time.Now()
}

func (t *timerModel) toggle() {
	switch t.state {
	case timerRunning:
		t.pause()
	case timerPaused:
		t.resume()
	}
}

func (t *timerModel) tick() {
	if t.state == timerRunning {
		t.elapsed = time.Since(t.startTime) - t.pauseGap

		// Idle detection
		if time.Since(t.lastActivity) > t.idleTimeout && !t.isIdle {
			t.isIdle = true
			t.pause()
		}
	}
}

func (t *timerModel) recordActivity() {
	t.lastActivity = time.Now()
	if t.isIdle && t.state == timerPaused {
		t.resume()
		t.isIdle = false
	}
}

func (t timerModel) running() bool {
	return t.state != timerStopped
}

func (t timerModel) paused() bool {
	return t.state == timerPaused
}

func (t timerModel) currentElapsed() time.Duration {
	if t.state == timerStopped {
		return 0
	}
	if t.state == timerPaused {
		return time.Since(t.startTime) - t.pauseGap - time.Since(t.pausedAt)
	}
	return time.Since(t.startTime) - t.pauseGap
}
