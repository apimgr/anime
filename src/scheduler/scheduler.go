package scheduler

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Task represents a scheduled task
type Task struct {
	Name     string
	Schedule string // Cron-like format: "minute hour day month weekday"
	Func     func() error
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	tasks   []*Task
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// New creates a new Scheduler
func New() *Scheduler {
	return &Scheduler{
		tasks:  make([]*Task, 0),
		stopCh: make(chan struct{}),
	}
}

// AddTask adds a task to the scheduler
func (s *Scheduler) AddTask(name, schedule string, fn func() error) error {
	// Validate cron schedule format
	if err := validateCronSchedule(schedule); err != nil {
		return fmt.Errorf("invalid schedule format: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := &Task{
		Name:     name,
		Schedule: schedule,
		Func:     fn,
	}

	s.tasks = append(s.tasks, task)
	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Println("Scheduler started")

	s.wg.Add(1)
	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	log.Println("Scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkTasks(now)
		}
	}
}

// checkTasks checks if any tasks should run at the current time
func (s *Scheduler) checkTasks(now time.Time) {
	s.mu.RLock()
	tasks := make([]*Task, len(s.tasks))
	copy(tasks, s.tasks)
	s.mu.RUnlock()

	for _, task := range tasks {
		if shouldRun(task.Schedule, now) {
			go s.runTask(task)
		}
	}
}

// runTask executes a single task
func (s *Scheduler) runTask(task *Task) {
	log.Printf("Running scheduled task: %s", task.Name)

	startTime := time.Now()
	err := task.Func()
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("Task %s failed after %v: %v", task.Name, duration, err)
	} else {
		log.Printf("Task %s completed successfully in %v", task.Name, duration)
	}
}

// shouldRun checks if a task should run at the given time
func shouldRun(schedule string, now time.Time) bool {
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return false
	}

	minute := parts[0]
	hour := parts[1]
	day := parts[2]
	month := parts[3]
	weekday := parts[4]

	// Check minute
	if !matchField(minute, now.Minute(), 0, 59) {
		return false
	}

	// Check hour
	if !matchField(hour, now.Hour(), 0, 23) {
		return false
	}

	// Check day of month
	if !matchField(day, now.Day(), 1, 31) {
		return false
	}

	// Check month
	if !matchField(month, int(now.Month()), 1, 12) {
		return false
	}

	// Check weekday (0 = Sunday, 6 = Saturday)
	if !matchField(weekday, int(now.Weekday()), 0, 6) {
		return false
	}

	return true
}

// matchField checks if a cron field matches the current value
func matchField(field string, value, min, max int) bool {
	// * matches any value
	if field == "*" {
		return true
	}

	// Check for range (e.g., 1-5)
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return value >= start && value <= end
			}
		}
		return false
	}

	// Check for list (e.g., 1,3,5)
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				if v == value {
					return true
				}
			}
		}
		return false
	}

	// Check for step values (e.g., */5)
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) == 2 {
			step, err := strconv.Atoi(parts[1])
			if err == nil && step > 0 {
				if parts[0] == "*" {
					return value%step == 0
				}
				start, err := strconv.Atoi(parts[0])
				if err == nil {
					return value >= start && (value-start)%step == 0
				}
			}
		}
		return false
	}

	// Exact match
	v, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	return v == value
}

// validateCronSchedule validates a cron schedule format
func validateCronSchedule(schedule string) error {
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return fmt.Errorf("schedule must have 5 fields (minute hour day month weekday), got %d", len(parts))
	}

	// Validate each field
	fields := []struct {
		name  string
		field string
		min   int
		max   int
	}{
		{"minute", parts[0], 0, 59},
		{"hour", parts[1], 0, 23},
		{"day", parts[2], 1, 31},
		{"month", parts[3], 1, 12},
		{"weekday", parts[4], 0, 6},
	}

	for _, f := range fields {
		if err := validateField(f.field, f.min, f.max); err != nil {
			return fmt.Errorf("invalid %s field: %w", f.name, err)
		}
	}

	return nil
}

// validateField validates a single cron field
func validateField(field string, min, max int) error {
	// * is always valid
	if field == "*" {
		return nil
	}

	// Check for range (e.g., 1-5)
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range format")
		}
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range values")
		}
		if start < min || start > max || end < min || end > max || start > end {
			return fmt.Errorf("range out of bounds (%d-%d)", min, max)
		}
		return nil
	}

	// Check for list (e.g., 1,3,5)
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return fmt.Errorf("invalid list value")
			}
			if v < min || v > max {
				return fmt.Errorf("list value out of bounds (%d-%d)", min, max)
			}
		}
		return nil
	}

	// Check for step values (e.g., */5)
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid step format")
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return fmt.Errorf("invalid step value")
		}
		if parts[0] != "*" {
			start, err := strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid step start value")
			}
			if start < min || start > max {
				return fmt.Errorf("step start out of bounds (%d-%d)", min, max)
			}
		}
		return nil
	}

	// Exact value
	v, err := strconv.Atoi(field)
	if err != nil {
		return fmt.Errorf("invalid value")
	}
	if v < min || v > max {
		return fmt.Errorf("value out of bounds (%d-%d)", min, max)
	}

	return nil
}
