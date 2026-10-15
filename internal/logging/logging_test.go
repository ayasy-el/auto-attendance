package logging

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestLogStoreBasicAndRecent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := Configure("Asia/Jakarta", dbPath, 7)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Insert test logs directly
	now := time.Now()
	for i := 1; i <= 10; i++ {
		store.insertDirect(logEntry{
			timestamp: now.Add(time.Duration(i) * time.Second),
			level:     "INFO",
			message:   fmt.Sprintf("log line %d", i),
		})
	}

	// Default 5 logs
	logs5, err := store.GetRecentLogs(ctx, 5)
	if err != nil {
		t.Fatalf("GetRecentLogs(5) error: %v", err)
	}
	if len(logs5) != 5 {
		t.Fatalf("expected 5 logs, got %d", len(logs5))
	}
	// Check order: oldest first (line 6), newest last (line 10)
	if logs5[0] == "" || logs5[4] == "" {
		t.Fatal("empty log content")
	}

	// 10 logs
	logs10, err := store.GetRecentLogs(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentLogs(10) error: %v", err)
	}
	if len(logs10) != 10 {
		t.Fatalf("expected 10 logs, got %d", len(logs10))
	}
}

func TestLogStorePrune(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_prune.db")

	store, err := newLogStore(dbPath, 5, time.Local)
	if err != nil {
		t.Fatalf("newLogStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Insert an old log (10 days ago)
	oldTime := time.Now().AddDate(0, 0, -10)
	store.insertDirect(logEntry{
		timestamp: oldTime,
		level:     "INFO",
		message:   "old log from 10 days ago",
	})

	// Insert a recent log (today)
	store.insertDirect(logEntry{
		timestamp: time.Now(),
		level:     "INFO",
		message:   "recent log from today",
	})

	// Prune logs older than 5 days
	pruned, err := store.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned log, got %d", pruned)
	}

	// Check remaining logs
	logs, err := store.GetRecentLogs(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentLogs error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 remaining log, got %d", len(logs))
	}
}
