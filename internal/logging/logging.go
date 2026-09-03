package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Configure makes the standard logger print timestamps in the configured
// timezone, independent of the machine/container timezone.
func Configure(timezone string) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&timezoneWriter{out: os.Stderr, loc: loc})
}

type timezoneWriter struct {
	out io.Writer
	loc *time.Location
	mu  sync.Mutex
}

func (w *timezoneWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprintf(w.out, "[%s] %s", time.Now().In(w.loc).Format("2006-01-02 15:04:05 MST"), p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
