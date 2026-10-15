package logging

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[1;31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"

	colorEnabled = true
)

type logEntry struct {
	timestamp time.Time
	level     string
	message   string
}

// LogStore manages SQLite persistence for logs with automatic retention.
type LogStore struct {
	db            *sql.DB
	loc           *time.Location
	retentionDays int
	msgCh         chan logEntry
	closeCh       chan struct{}
	doneCh        chan struct{}
	mu            sync.RWMutex
}

var globalStore *LogStore

// Configure sets up standard logging with timezone formatting and an optional SQLite log store.
func Configure(timezone, dbPath string, retentionDays int) (*LogStore, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}

	var store *LogStore
	if dbPath != "" {
		s, err := newLogStore(dbPath, retentionDays, loc)
		if err != nil {
			return nil, fmt.Errorf("buka sqlite log: %w", err)
		}
		store = s
		globalStore = s
	}

	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&timezoneWriter{
		out:   os.Stderr,
		loc:   loc,
		color: colorEnabledFor(os.Stderr),
		store: store,
	})

	return store, nil
}

// GetGlobalStore returns the active SQLite log store if configured.
func GetGlobalStore() *LogStore {
	return globalStore
}

func newLogStore(dbPath string, retentionDays int, loc *time.Location) (*LogStore, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("buat folder log: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS app_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		created_at INTEGER NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_logs_created_at ON app_logs(created_at);

	CREATE TABLE IF NOT EXISTS task_reminders (
		task_key TEXT NOT NULL,
		stage TEXT NOT NULL,
		sent_at INTEGER NOT NULL,
		PRIMARY KEY (task_key, stage)
	);
	CREATE INDEX IF NOT EXISTS idx_task_reminders_sent_at ON task_reminders(sent_at);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("inisialisasi tabel log: %w", err)
	}

	s := &LogStore{
		db:            db,
		loc:           loc,
		retentionDays: retentionDays,
		msgCh:         make(chan logEntry, 1024),
		closeCh:       make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go s.worker()

	// Initial pruning on startup
	if retentionDays > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.Prune(ctx)
		cancel()
	}

	return s, nil
}

func (s *LogStore) worker() {
	defer close(s.doneCh)
	pruneTicker := time.NewTicker(6 * time.Hour)
	defer pruneTicker.Stop()

	for {
		select {
		case <-s.closeCh:
			// Drain remaining messages
			for {
				select {
				case entry := <-s.msgCh:
					s.insertDirect(entry)
				default:
					return
				}
			}
		case <-pruneTicker.C:
			if s.retentionDays > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, _ = s.Prune(ctx)
				cancel()
			}
		case entry := <-s.msgCh:
			s.insertDirect(entry)
		}
	}
}

func (s *LogStore) insertDirect(entry logEntry) {
	createdAt := entry.timestamp.Unix()
	timestampStr := entry.timestamp.Format("2006-01-02 15:04:05 MST")
	_, _ = s.db.Exec("INSERT INTO app_logs (timestamp, created_at, level, message) VALUES (?, ?, ?, ?)",
		timestampStr, createdAt, entry.level, entry.message)
}

// Prune deletes logs older than retentionDays.
func (s *LogStore) Prune(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.retentionDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays).Unix()
	_, _ = s.db.ExecContext(ctx, "DELETE FROM task_reminders WHERE sent_at < ?", cutoff)
	res, err := s.db.ExecContext(ctx, "DELETE FROM app_logs WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// HasTaskReminder checks whether a reminder for (taskKey, stage) has already been sent.
func (s *LogStore) HasTaskReminder(taskKey, stage string) bool {
	if s == nil || s.db == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists int
	err := s.db.QueryRow("SELECT 1 FROM task_reminders WHERE task_key = ? AND stage = ?", taskKey, stage).Scan(&exists)
	return err == nil && exists == 1
}

// RecordTaskReminder stores a sent reminder for (taskKey, stage).
func (s *LogStore) RecordTaskReminder(taskKey, stage string) error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("INSERT OR REPLACE INTO task_reminders (task_key, stage, sent_at) VALUES (?, ?, ?)",
		taskKey, stage, time.Now().Unix())
	return err
}

// GetRecentLogs returns the last n log lines formatted with timestamps.
func (s *LogStore) GetRecentLogs(ctx context.Context, n int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 {
		n = 5
	}
	if n > 50 {
		n = 50
	}

	rows, err := s.db.QueryContext(ctx, "SELECT timestamp, message FROM app_logs ORDER BY id DESC LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []string
	for rows.Next() {
		var ts, msg string
		if err := rows.Scan(&ts, &msg); err != nil {
			return nil, err
		}
		records = append(records, fmt.Sprintf("[%s] %s", ts, msg))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse so chronological order (oldest first, newest last)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// Close gracefully flushes pending logs and closes the database.
func (s *LogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.closeCh:
		return nil
	default:
		close(s.closeCh)
	}

	<-s.doneCh
	return s.db.Close()
}

type timezoneWriter struct {
	out   io.Writer
	loc   *time.Location
	color bool
	store *LogStore
	mu    sync.Mutex
}

func (w *timezoneWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	rawMessage := strings.TrimRight(string(p), "\r\n")
	now := time.Now().In(w.loc)
	level := logLevelForMessage(rawMessage)

	if w.store != nil {
		select {
		case w.store.msgCh <- logEntry{
			timestamp: now,
			level:     level,
			message:   rawMessage,
		}:
		default:
			// Non-blocking drop if channel full under extreme load
		}
	}

	message := rawMessage
	if w.color {
		message = colorize(message)
	}

	_, err := fmt.Fprintf(
		w.out,
		"[%s] %s\n",
		now.Format("2006-01-02 15:04:05 MST"),
		message,
	)

	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func colorEnabledFor(out io.Writer) bool {
	if !colorEnabled || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if forceColor() {
		return true
	}

	f, ok := out.(*os.File)
	if !ok {
		return false
	}

	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func forceColor() bool {
	switch strings.ToLower(os.Getenv("FORCE_COLOR")) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func colorize(message string) string {
	color := colorForMessage(message)
	if color == "" {
		return message
	}
	return color + message + colorReset
}

func logLevelForMessage(message string) string {
	lower := strings.ToLower(message)
	if containsAny(lower, "response_error", "read_error", "error", "gagal", "failed", "err=") {
		return "ERROR"
	}
	if containsAny(lower, "peringatan", "warning", "tidak_aktif") {
		return "WARN"
	}
	if containsAny(lower, "sukses", "berhasil", "success") {
		return "SUCCESS"
	}
	return "INFO"
}

func colorForMessage(message string) string {
	lower := strings.ToLower(message)
	if containsAny(lower, "response_error", "read_error") {
		return colorRed
	}

	if strings.Contains(lower, "http request:") {
		return colorCyan
	}

	if strings.Contains(lower, "status=") {
		if status, ok := statusCode(lower); ok {
			switch {
			case status >= 500:
				return colorRed
			case status >= 400:
				return colorYellow
			case status >= 300:
				return colorCyan
			default:
				return colorGreen
			}
		}

		if strings.Contains(lower, "status=client_error") ||
			strings.Contains(lower, "status=network_error") {
			return colorRed
		}

		return colorBlue
	}

	if containsAny(lower, "read_error", "error", "gagal", "failed", "err=") {
		return colorRed
	}

	if containsAny(lower, "peringatan", "warning", "tidak_aktif") {
		return colorYellow
	}

	if containsAny(lower, "sukses", "berhasil", "success") {
		return colorGreen
	}

	return colorGray
}

func statusCode(message string) (int, bool) {
	index := strings.Index(message, "status=")
	if index < 0 {
		return 0, false
	}

	value := message[index+len("status="):]
	end := strings.IndexAny(value, " \t\n\r")
	if end >= 0 {
		value = value[:end]
	}

	status, err := strconv.Atoi(value)
	return status, err == nil
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
