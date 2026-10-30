package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
)

func TestWeekdayAndWeekendBoundary(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	saturday := time.Date(2026, time.September, 5, 12, 0, 0, 0, loc)
	if isWeekday(saturday) {
		t.Fatal("Saturday must not be a weekday")
	}
	if got := untilMonday(saturday); got != 36*time.Hour {
		t.Fatalf("until Monday = %s", got)
	}
}

func TestClassAndActiveWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	s := &Scheduler{loc: loc, cfg: config.Config{Schedule: config.ScheduleConfig{ActiveStart: "08:00", ActiveEnd: "18:00"}}, schedules: []ethol.ClassTime{{Course: 10, Day: 1, Start: "10:00", End: "12:00"}}}
	monday := time.Date(2026, time.September, 7, 10, 30, 0, 0, loc)
	if !s.inClass(monday) || !s.inActiveWindow(monday) {
		t.Fatal("expected Monday class and active window")
	}
	if s.inClass(monday.Add(3 * time.Hour)) {
		t.Fatal("class should have ended")
	}
	if s.inActiveWindow(monday.Add(9 * time.Hour)) {
		t.Fatal("active window should have ended")
	}
	if _, ok := s.classCourses(monday)["10"]; !ok {
		t.Fatal("active course not found")
	}
}

func TestModeChangesToOutsideAfterActiveClassAttendance(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, time.September, 7, 10, 30, 0, 0, loc)
	s := &Scheduler{
		loc:       loc,
		schedules: []ethol.ClassTime{{Course: 10, Day: 1, Start: "10:00", End: "12:00"}},
		courses:   []ethol.Course{{Number: 10, Schema: 1}},
		attended:  map[string]time.Time{},
	}

	if got := s.mode(now); got != "inside-class" {
		t.Fatalf("mode before attendance = %q, want inside-class", got)
	}
	s.attended["10:1:2026-09-07"] = now
	if got := s.mode(now); got != "outside-class" {
		t.Fatalf("mode after attendance = %q, want outside-class", got)
	}
	if got := s.mode(now.AddDate(0, 0, 1)); got != "outside-class" {
		t.Fatalf("mode outside active class = %q, want outside-class", got)
	}
}

func TestRandomIntervalWithinConfiguredBounds(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	s := &Scheduler{loc: loc, cfg: config.Config{Schedule: config.ScheduleConfig{OutsideClassInterval: config.IntervalRange{Min: "1m", Max: "2m"}, ActiveStart: "08:00", ActiveEnd: "18:00"}}}
	now := time.Date(2026, time.September, 7, 14, 0, 0, 0, loc)
	// interval() uses time.Now internally; validate the range implementation
	// directly here and keep scheduler boundary behavior covered separately.
	for i := 0; i < 20; i++ {
		d, err := s.cfg.Schedule.OutsideClassInterval.RandomDuration()
		if err != nil || d < time.Minute || d > 2*time.Minute {
			t.Fatalf("duration=%s err=%v", d, err)
		}
	}
	_ = now
}

func TestParseMinutes(t *testing.T) {
	cases := []struct {
		input    string
		expected int
		hasErr   bool
	}{
		{"08:30", 510, false},
		{"08.30", 510, false},
		{"08:30:00", 510, false},
		{"08.30.00", 510, false},
		{"08:30 WIB", 510, false},
		{" 09:15 ", 555, false},
		{"00:00", 0, false},
		{"23:59", 1439, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"24:00", 0, true},
		{"08:60", 0, true},
		{"-01:00", 0, true},
	}

	for _, tc := range cases {
		got, err := parseMinutes(tc.input)
		if tc.hasErr && err == nil {
			t.Errorf("parseMinutes(%q) expected error, got %d", tc.input, got)
		}
		if !tc.hasErr && err != nil {
			t.Errorf("parseMinutes(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.hasErr && got != tc.expected {
			t.Errorf("parseMinutes(%q) = %d, expected %d", tc.input, got, tc.expected)
		}
	}
}

func TestSchedulerDryRun(t *testing.T) {
	s := &Scheduler{}
	if s.dryRun {
		t.Fatal("expected default dryRun to be false")
	}
	s.SetDryRun(true)
	if !s.dryRun {
		t.Fatal("expected dryRun to be true after SetDryRun(true)")
	}
}

type testNotifier struct {
	reminders []string
}

func (n *testNotifier) NotifyPresenceSuccess(ctx context.Context, courseName, dosen, key string) {}
func (n *testNotifier) NotifyContractApproved(ctx context.Context, courseName string)           {}
func (n *testNotifier) NotifyPresenceFailed(ctx context.Context, courseName string, err error)    {}
func (n *testNotifier) NotifyTaskReminder(ctx context.Context, task ethol.TaskItem, stage string, remaining time.Duration) {
	n.reminders = append(n.reminders, fmt.Sprintf("%s:%s", task.TaskTitle(), stage))
}

func TestScheduler_TaskRemindersDeduplication(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	n := &testNotifier{}
	s := &Scheduler{
		loc:           loc,
		sentReminders: make(map[string]bool),
		notifier:      n,
	}

	ctx := context.Background()

	// Task with 20h remaining (within H-1 / 24h)
	now := time.Now().In(loc)
	deadline20h := now.Add(20 * time.Hour).Format("02-01-2006 15:04:05")

	task := ethol.TaskItem{
		KuliahID: 101,
		Title:    "Laporan Akhir",
		Deadline: deadline20h,
	}

	s.SetTasks([]ethol.TaskItem{task})

	// First check: should trigger "Deadline H-1"
	s.CheckTaskReminders(ctx)
	if len(n.reminders) != 1 || n.reminders[0] != "Laporan Akhir:Deadline H-1" {
		t.Fatalf("expected 1 reminder 'Deadline H-1', got: %v", n.reminders)
	}

	// Second check with same deadline: should be deduplicated (no new reminder)
	s.CheckTaskReminders(ctx)
	if len(n.reminders) != 1 {
		t.Fatalf("expected reminder to be deduplicated, got: %v", n.reminders)
	}

	// Update task to 4h remaining (within 5h)
	deadline4h := now.Add(4 * time.Hour).Format("02-01-2006 15:04:05")
	task.Deadline = deadline4h
	s.SetTasks([]ethol.TaskItem{task})

	s.CheckTaskReminders(ctx)
	if len(n.reminders) != 2 || n.reminders[1] != "Laporan Akhir:Kurang 5 Jam" {
		t.Fatalf("expected second reminder 'Kurang 5 Jam', got: %v", n.reminders)
	}

	// Update task to 2h remaining (within 3h)
	deadline2h := now.Add(2 * time.Hour).Format("02-01-2006 15:04:05")
	task.Deadline = deadline2h
	s.SetTasks([]ethol.TaskItem{task})

	s.CheckTaskReminders(ctx)
	if len(n.reminders) != 3 || n.reminders[2] != "Laporan Akhir:Kurang 3 Jam" {
		t.Fatalf("expected third reminder 'Kurang 3 Jam', got: %v", n.reminders)
	}

	// Update task to 40m remaining (within 1h)
	deadline40m := now.Add(40 * time.Minute).Format("02-01-2006 15:04:05")
	task.Deadline = deadline40m
	s.SetTasks([]ethol.TaskItem{task})

	s.CheckTaskReminders(ctx)
	if len(n.reminders) != 4 || n.reminders[3] != "Laporan Akhir:Kurang 1 Jam" {
		t.Fatalf("expected fourth reminder 'Kurang 1 Jam', got: %v", n.reminders)
	}
}

func TestScheduler_StatusDeadlockDuringActiveClass(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	s := &Scheduler{
		loc: loc,
		schedules: []ethol.ClassTime{
			{Course: 10, Day: weekday, Start: "00:00", End: "23:59"},
		},
		courses: []ethol.Course{
			{Number: 10, Schema: 1},
		},
		attended: map[string]time.Time{},
	}

	done := make(chan struct{})
	go func() {
		status := s.Status()
		if status.Mode != "inside-class" {
			t.Errorf("expected inside-class, got %s", status.Mode)
		}
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Status() hung because of double mutex lock when class is active!")
	}
}

