package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
)

type Scheduler struct {
	client    *ethol.Client
	cfg       config.Config
	student   ethol.Student
	loc       *time.Location
	schedules []ethol.ClassTime
	courses   []ethol.Course
	attended  map[string]time.Time
	mu        sync.Mutex
}

func New(client *ethol.Client, cfg config.Config, student ethol.Student) *Scheduler {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.Local
	}
	return &Scheduler{client: client, cfg: cfg, student: student, loc: loc, attended: map[string]time.Time{}}
}
func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.refresh(ctx); err != nil {
		log.Printf("peringatan: gagal memuat jadwal: %v", err)
	}
	refresh, _ := time.ParseDuration(s.cfg.Schedule.RefreshScheduleInterval)
	nextRefresh := time.Now().Add(refresh)
	if s.cfg.Schedule.RunImmediately {
		s.tick(ctx)
	}
	for {
		now := time.Now().In(s.loc)
		interval := s.interval()
		nextTick := now.Add(interval)
		log.Printf("scheduler: mode=%s next_tick_at=%s delay=%s", s.mode(now), nextTick.Format("2006-01-02 15:04:05 MST"), interval)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case now := <-timer.C:
			if now.After(nextRefresh) {
				if err := s.refresh(ctx); err != nil {
					log.Printf("gagal refresh jadwal: %v", err)
				}
				nextRefresh = now.Add(refresh)
			}
			s.tick(ctx)
		}
	}
}
func (s *Scheduler) refresh(ctx context.Context) error {
	courses, err := s.client.Courses(ctx)
	if err != nil {
		return err
	}
	schedules, err := s.client.Schedules(ctx, courses)
	if err != nil {
		return err
	}
	s.schedules = schedules
	s.courses = courses
	log.Printf("jadwal dimuat: %d kuliah, %d slot jadwal", len(courses), len(schedules))
	return nil
}
func (s *Scheduler) interval() time.Duration {
	now := time.Now().In(s.loc)
	if !isWeekday(now) {
		return untilMonday(now)
	}
	if s.mode(now) == "inside-class" {
		d, err := s.cfg.Schedule.InsideClassInterval.RandomDuration()
		if err != nil {
			return time.Minute
		}
		return d
	}
	if !s.inActiveWindow(now) {
		return s.untilActiveStart(now)
	}
	d, err := s.cfg.Schedule.OutsideClassInterval.RandomDuration()
	if err != nil {
		return 15 * time.Minute
	}
	// Wake up at the next class boundary so a 15-minute tick cannot skip
	// the start of a class.
	if until := s.untilNextStart(now); until > 0 && until < d {
		return until
	}
	return d
}

func isWeekday(now time.Time) bool {
	return now.Weekday() >= time.Monday && now.Weekday() <= time.Friday
}

func untilMonday(now time.Time) time.Duration {
	daysUntilMonday := (int(time.Monday) - int(now.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, now.Location())
	return next.Sub(now)
}

func (s *Scheduler) inActiveWindow(now time.Time) bool {
	start, errStart := parseMinutes(s.cfg.Schedule.ActiveStart)
	end, errEnd := parseMinutes(s.cfg.Schedule.ActiveEnd)
	if errStart != nil || errEnd != nil {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	return current >= start && current < end
}

func (s *Scheduler) untilActiveStart(now time.Time) time.Duration {
	start, err := parseMinutes(s.cfg.Schedule.ActiveStart)
	if err != nil {
		return time.Hour
	}
	candidate := time.Date(now.Year(), now.Month(), now.Day(), start/60, start%60, 0, 0, s.loc)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate.Sub(now)
}

func (s *Scheduler) untilNextStart(now time.Time) time.Duration {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	best := 8 * 24 * time.Hour
	for _, item := range s.schedules {
		dayDelta := (item.Day - weekday + 7) % 7
		start, err := parseMinutes(item.Start)
		if err != nil {
			continue
		}
		candidate := time.Date(now.Year(), now.Month(), now.Day()+dayDelta, start/60, start%60, 0, 0, s.loc)
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		if wait := candidate.Sub(now); wait < best {
			best = wait
		}
	}
	return best
}
func (s *Scheduler) inClass(now time.Time) bool {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	current := now.Hour()*60 + now.Minute()
	for _, item := range s.schedules {
		if item.Day != weekday {
			continue
		}
		start, e1 := parseMinutes(item.Start)
		end, e2 := parseMinutes(item.End)
		if e1 == nil && e2 == nil && current >= start && current <= end {
			return true
		}
	}
	return false
}
func parseMinutes(value string) (int, error) {
	var h, m int
	_, err := fmt.Sscanf(value, "%d:%d", &h, &m)
	return h*60 + m, err
}
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().In(s.loc)
	inClass := s.inClass(now)
	if !isWeekday(now) || (!inClass && !s.inActiveWindow(now)) {
		return
	}
	log.Printf("tick: mode=%s time=%s courses=%d", s.mode(now), now.Format("2006-01-02 15:04:05 MST"), len(s.courses))
	for _, course := range s.courses {
		key := fmt.Sprintf("%d:%d:%s", course.Number, course.Schema, now.Format("2006-01-02"))
		s.mu.Lock()
		_, done := s.attended[key]
		s.mu.Unlock()
		if done {
			continue
		}
		presenceKey, err := s.client.AttendCourse(ctx, course, s.student)
		if err != nil {
			if errors.Is(err, ethol.ErrNoActivePresence) {
				log.Printf("kuliah=%d nama=%q status=tidak_aktif", course.Number, course.Subject.Name)
				continue
			}
			if errors.Is(err, ethol.ErrAlreadyAttended) {
				s.mu.Lock()
				s.attended[key] = time.Now()
				s.mu.Unlock()
				log.Printf("kuliah=%d nama=%q status=sudah_presensi", course.Number, course.Subject.Name)
				continue
			}
			log.Printf("kuliah=%d nama=%q presensi=gagal error=%v", course.Number, course.Subject.Name, err)
			continue
		}
		s.mu.Lock()
		s.attended[key] = time.Now()
		s.mu.Unlock()
		log.Printf("kuliah=%d nama=%q presensi=sukses key=%s", course.Number, course.Subject.Name, presenceKey)
	}
}

func (s *Scheduler) mode(now time.Time) string {
	if s.inClass(now) && !s.hasAttendedActiveClass(now) {
		return "inside-class"
	}
	return "outside-class"
}

func (s *Scheduler) hasAttendedActiveClass(now time.Time) bool {
	activeCourses := s.classCourses(now)
	if len(activeCourses) == 0 {
		return false
	}

	date := now.Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, course := range s.courses {
		if _, active := activeCourses[fmt.Sprintf("%d", course.Number)]; !active {
			continue
		}
		key := fmt.Sprintf("%d:%d:%s", course.Number, course.Schema, date)
		if _, attended := s.attended[key]; attended {
			return true
		}
	}
	return false
}

func (s *Scheduler) classCourses(now time.Time) map[string]struct{} {
	result := make(map[string]struct{})
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	current := now.Hour()*60 + now.Minute()
	for _, item := range s.schedules {
		if item.Day != weekday {
			continue
		}
		start, startErr := parseMinutes(item.Start)
		end, endErr := parseMinutes(item.End)
		if startErr == nil && endErr == nil && current >= start && current <= end {
			result[fmt.Sprintf("%d", item.Course)] = struct{}{}
		}
	}
	return result
}
