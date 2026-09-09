package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	colorReset = "\033[0m"

	// Log levels / categories
	colorRed    = "\033[1;31m" // Bold Red   - Error / HTTP 5xx
	colorGreen  = "\033[32m"   // Green       - Success / HTTP 2xx
	colorYellow = "\033[33m"   // Yellow      - Warning / HTTP 4xx
	colorBlue   = "\033[34m"   // Blue        - Non-HTTP status
	colorCyan   = "\033[36m"   // Cyan        - HTTP request / HTTP 3xx
	colorGray   = "\033[90m"   // Gray        - General info

	colorEnabled = true
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
	log.SetOutput(&timezoneWriter{
		out:   os.Stderr,
		loc:   loc,
		color: colorEnabledFor(os.Stderr),
	})
}

type timezoneWriter struct {
	out   io.Writer
	loc   *time.Location
	color bool
	mu    sync.Mutex
}

func (w *timezoneWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	message := string(p)

	if w.color {
		message = colorize(message)
	}

	_, err := fmt.Fprintf(
		w.out,
		"[%s] %s",
		time.Now().In(w.loc).Format("2006-01-02 15:04:05 MST"),
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

func colorForMessage(message string) string {
	lower := strings.ToLower(message)
	if containsAny(lower, "response_error", "read_error") {
		return colorRed
	}

	// HTTP request
	if strings.Contains(lower, "http request:") {
		return colorCyan
	}

	// HTTP status
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

		// Non-HTTP status
		if strings.Contains(lower, "status=client_error") ||
			strings.Contains(lower, "status=network_error") {
			return colorRed
		}

		return colorBlue
	}

	// General errors
	if containsAny(lower, "read_error", "error", "gagal", "failed", "err=") {
		return colorRed
	}

	// Warnings
	if containsAny(lower, "peringatan", "warning", "tidak_aktif") {
		return colorYellow
	}

	// Success
	if containsAny(lower, "sukses", "berhasil", "success") {
		return colorGreen
	}

	// General info
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
