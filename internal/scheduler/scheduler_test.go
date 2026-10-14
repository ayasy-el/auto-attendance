package scheduler

import (
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
