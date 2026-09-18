package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestAddTaskAndGetTasks(t *testing.T) {
	s := New()
	s.AddTask("task1", time.Hour, func() error { return nil })

	tasks := s.GetTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Name != "task1" {
		t.Errorf("expected task name task1, got %q", tasks[0].Name)
	}
	if !tasks[0].Enabled {
		t.Error("expected task to be enabled by default")
	}
}

func TestRemoveTask(t *testing.T) {
	s := New()
	s.AddTask("task1", time.Hour, func() error { return nil })
	s.RemoveTask("task1")

	if len(s.GetTasks()) != 0 {
		t.Errorf("expected 0 tasks after removal, got %d", len(s.GetTasks()))
	}
}

func TestEnableDisableTask(t *testing.T) {
	s := New()
	s.AddTask("task1", time.Hour, func() error { return nil })

	s.DisableTask("task1")
	tasks := s.GetTasks()
	if tasks[0].Enabled {
		t.Error("expected task to be disabled")
	}

	s.EnableTask("task1")
	tasks = s.GetTasks()
	if !tasks[0].Enabled {
		t.Error("expected task to be enabled")
	}
}

func TestEnableDisableUnknownTask(t *testing.T) {
	s := New()
	// Unknown task names must be silently ignored, not panic.
	s.EnableTask("missing")
	s.DisableTask("missing")
}

func TestRunNow(t *testing.T) {
	s := New()
	ran := false
	s.AddTask("task1", time.Hour, func() error {
		ran = true
		return nil
	})

	if err := s.RunNow("task1"); err != nil {
		t.Fatalf("RunNow returned error: %v", err)
	}
	if !ran {
		t.Error("expected task function to run")
	}

	tasks := s.GetTasks()
	if tasks[0].LastRun.IsZero() {
		t.Error("expected LastRun to be set")
	}
}

func TestRunNowError(t *testing.T) {
	s := New()
	wantErr := errors.New("boom")
	s.AddTask("task1", time.Hour, func() error {
		return wantErr
	})

	if err := s.RunNow("task1"); !errors.Is(err, wantErr) {
		t.Errorf("expected error %v, got %v", wantErr, err)
	}
}

func TestRunNowUnknownTask(t *testing.T) {
	s := New()
	if err := s.RunNow("missing"); err != nil {
		t.Errorf("expected nil error for unknown task, got %v", err)
	}
}

func TestStartStop(t *testing.T) {
	s := New()
	s.AddTask("task1", time.Hour, func() error { return nil })

	s.Start()
	// Starting again while already running must be a no-op.
	s.Start()
	s.Stop()
	// Stopping again while already stopped must be a no-op.
	s.Stop()
}

func TestRunDueTasks(t *testing.T) {
	s := New()
	ran := make(chan struct{}, 1)
	s.AddTask("due-task", time.Hour, func() error {
		ran <- struct{}{}
		return nil
	})
	s.AddTask("not-due-task", time.Hour, func() error {
		t.Error("not-due task should not run")
		return nil
	})

	s.mu.Lock()
	s.tasks["due-task"].NextRun = time.Now().Add(-time.Minute)
	s.tasks["not-due-task"].NextRun = time.Now().Add(time.Hour)
	s.mu.Unlock()

	s.runDueTasks()

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("expected due task to run")
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"minutely", time.Minute},
		{"hourly", time.Hour},
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
		{"monthly", 30 * 24 * time.Hour},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"not-a-duration", 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseInterval(tt.input); got != tt.want {
				t.Errorf("ParseInterval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
