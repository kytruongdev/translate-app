package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger is the business logger interface injected into controllers.
type Logger interface {
	Info(event string, fields ...any)
	Warn(event string, fields ...any)
	Error(event string, fields ...any)
	Close()
}

type fileLogger struct {
	mu   sync.Mutex
	file *os.File
}

// New opens ~/.config/TranslateApp/app.log for appending.
// Rotates to app.log.1 when the file exceeds 10 MiB.
func New() (Logger, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(dir, "TranslateApp")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, "app.log")
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 10<<20 {
		_ = os.Rename(logPath, logPath+".1")
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &fileLogger{file: f}, nil
}

func formatVal(v any) string {
	s := fmt.Sprintf("%v", v)
	if strings.ContainsAny(s, " \t\n\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func (l *fileLogger) write(level, event string, fields []any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s] [%-5s] %s", ts, level, event)
	if len(fields) >= 2 {
		sb.WriteString(" |")
		for i := 0; i+1 < len(fields); i += 2 {
			fmt.Fprintf(&sb, " %v=%s", fields[i], formatVal(fields[i+1]))
		}
	}
	sb.WriteByte('\n')
	l.mu.Lock()
	_, _ = l.file.WriteString(sb.String())
	l.mu.Unlock()
}

func (l *fileLogger) Info(event string, fields ...any)  { l.write("INFO", event, fields) }
func (l *fileLogger) Warn(event string, fields ...any)  { l.write("WARN", event, fields) }
func (l *fileLogger) Error(event string, fields ...any) { l.write("ERROR", event, fields) }
func (l *fileLogger) Close()                            { _ = l.file.Close() }
