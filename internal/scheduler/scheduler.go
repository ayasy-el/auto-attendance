package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
)

type Notifier interface {
	NotifyPresenceSuccess(ctx context.Context, courseName, key string)
	NotifyContractApproved(ctx context.Context, courseName string)
	NotifyPresenceFailed(ctx context.Context, courseName string, err error)
}

type SchedulerStatus struct {
	StartTime time.Time
	LastTick  time.Time
	Mode      string
	IsPaused  bool
	IsDryRun  bool
	Courses   int
	Student   ethol.Student
}

type Scheduler struct {
	client    *ethol.Client
	cfg       config.Config
	student   ethol.Student
	loc       *time.Location
	schedules []ethol.ClassTime
	courses   []ethol.Course
	attended  map[string]time.Time
	mu        sync.Mutex
	dryRun    bool
	paused    bool
	notifier  Notifier
	startTime time.Time
	lastTick  time.Time
}

func (s *Scheduler) SetNotifier(n Notifier) {
	s.mu.Lock()
	s.notifier = n
	s.mu.Unlock()
}

func (s *Scheduler) Pause() {
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
}

func (s *Scheduler) Resume() {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
}

func (s *Scheduler) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

func (s *Scheduler) Status() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().In(s.loc)
	return SchedulerStatus{
		StartTime: s.startTime,
		LastTick:  s.lastTick,
		Mode:      s.mode(now),
		IsPaused:  s.paused,
		IsDryRun:  s.dryRun,
		Courses:   len(s.courses),
		Student:   s.student,
	}
}

func (s *Scheduler) AttendedToday() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().In(s.loc)
	todayPrefix := now.Format("2006-01-02")
	var result []string
	for k := range s.attended {
		if strings.HasSuffix(k, todayPrefix) {
			result = append(result, k)
		}
	}
	return result
}

func (s *Scheduler) TriggerCheck(ctx context.Context) (int, error) {
	if err := s.refresh(ctx); err != nil {
		return 0, fmt.Errorf("gagal memuat jadwal: %w", err)
	}
	now := time.Now().In(s.loc)
	log.Printf("trigger-check: mode=%s time=%s courses=%d", s.mode(now), now.Format("2006-01-02 15:04:05 MST"), len(s.courses))
	s.scanCourses(ctx, now)
	return len(s.courses), nil
}

func (s *Scheduler) SetDryRun(dryRun bool) {
	s.mu.Lock()
	s.dryRun = dryRun
	s.mu.Unlock()
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	if err := s.refresh(ctx); err != nil {
		return fmt.Errorf("gagal memuat jadwal: %w", err)
	}
	now := time.Now().In(s.loc)
	log.Printf("scan-once: mode=%s time=%s courses=%d", s.mode(now), now.Format("2006-01-02 15:04:05 MST"), len(s.courses))
	s.scanCourses(ctx, now)
	return nil
}

func New(client *ethol.Client, cfg config.Config, student ethol.Student) *Scheduler {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.Local
	}
	return &Scheduler{
		client:    client,
		cfg:       cfg,
		student:   student,
		loc:       loc,
		attended:  map[string]time.Time{},
		startTime: time.Now().In(loc),
	}
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
		if s.IsPaused() {
			log.Printf("scheduler: mode=paused next_tick_in=1m")
			timer := time.NewTimer(1 * time.Minute)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}

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
	s.client.ResetActivePeriod()
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
	s := strings.TrimSpace(value)
	if s == "" {
		return 0, errors.New("waktu kosong")
	}
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	s = strings.ReplaceAll(s, ".", ":")
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("format waktu tidak valid: %q", value)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("jam tidak valid: %q", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("menit tidak valid: %q", parts[1])
	}
	return h*60 + m, nil
}
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().In(s.loc)
	inClass := s.inClass(now)
	if !isWeekday(now) || (!inClass && !s.inActiveWindow(now)) {
		return
	}
	log.Printf("tick: mode=%s time=%s courses=%d", s.mode(now), now.Format("2006-01-02 15:04:05 MST"), len(s.courses))
	s.scanCourses(ctx, now)
}

func (s *Scheduler) scanCourses(ctx context.Context, now time.Time) {
	s.mu.Lock()
	isDryRun := s.dryRun
	notifier := s.notifier
	s.lastTick = now
	s.mu.Unlock()

	for _, course := range s.courses {
		key := fmt.Sprintf("%d:%d:%s", course.Number, course.Schema, now.Format("2006-01-02"))
		if isDryRun {
			presenceKey, err := s.client.CheckPresence(ctx, course, s.student)
			if err != nil {
				if errors.Is(err, ethol.ErrNoActivePresence) {
					log.Printf("[DRY-RUN] kuliah=%d nama=%q status=tidak_aktif", course.Number, course.Subject.Name)
					continue
				}
				if errors.Is(err, ethol.ErrAlreadyAttended) {
					log.Printf("[DRY-RUN] kuliah=%d nama=%q status=sudah_presensi", course.Number, course.Subject.Name)
					continue
				}
				log.Printf("[DRY-RUN] kuliah=%d nama=%q status=gagal error=%v", course.Number, course.Subject.Name, err)
				continue
			}
			log.Printf("[DRY-RUN] kuliah=%d nama=%q presensi=aktif key=%s (submit dilewati)", course.Number, course.Subject.Name, presenceKey)
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
			if notifier != nil {
				notifier.NotifyPresenceFailed(ctx, course.Subject.Name, err)
			}
			continue
		}
		s.mu.Lock()
		s.attended[key] = time.Now()
		s.mu.Unlock()
		log.Printf("kuliah=%d nama=%q presensi=sukses key=%s", course.Number, course.Subject.Name, presenceKey)
		if notifier != nil {
			notifier.NotifyPresenceSuccess(ctx, course.Subject.Name, presenceKey)
		}
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
