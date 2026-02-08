package logging

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type DeployLogger struct {
	appID   string
	lines   []string
	mu      sync.Mutex
	onLine  func(appID, line string)
}

func NewDeployLogger(appID string, onLine func(appID, line string)) *DeployLogger {
	return &DeployLogger{
		appID:  appID,
		onLine: onLine,
	}
}

func (l *DeployLogger) Log(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	full := fmt.Sprintf("[%s] %s", ts, line)

	l.mu.Lock()
	l.lines = append(l.lines, full)
	l.mu.Unlock()

	log.Printf("[%s] %s", l.appID, line)

	if l.onLine != nil {
		l.onLine(l.appID, full)
	}
}

func (l *DeployLogger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.lines))
	copy(cp, l.lines)
	return cp
}
