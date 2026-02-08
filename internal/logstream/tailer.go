package logstream

import (
	"bufio"
	"context"
	"os"
	"time"
)

// Tailer polls a log file for new lines and sends them on a channel.
type Tailer struct {
	path     string
	backlog  int
	interval time.Duration
}

// NewTailer creates a tailer for the given log file path.
// backlog is the number of trailing lines to send as initial context.
func NewTailer(path string, backlog int) *Tailer {
	return &Tailer{
		path:     path,
		backlog:  backlog,
		interval: 500 * time.Millisecond,
	}
}

// Start begins tailing the file. It sends the last N backlog lines first,
// then polls for new content. The returned channel is closed when the
// context is cancelled or an unrecoverable error occurs.
func (t *Tailer) Start(ctx context.Context) <-chan string {
	ch := make(chan string, 64)
	go t.run(ctx, ch)
	return ch
}

func (t *Tailer) run(ctx context.Context, ch chan<- string) {
	defer close(ch)

	// Wait for the file to exist (it may not be created yet)
	var offset int64
	for {
		info, err := os.Stat(t.path)
		if err == nil {
			offset = t.sendBacklog(ctx, ch, info.Size())
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(t.interval):
		}
	}

	// Poll for new content
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(t.path)
			if err != nil {
				continue
			}
			newSize := info.Size()
			if newSize <= offset {
				if newSize < offset {
					// File was truncated — reset
					offset = 0
				}
				continue
			}
			offset = t.sendNewLines(ctx, ch, offset)
		}
	}
}

// sendBacklog reads the file and sends the last N lines. Returns the file offset after reading.
func (t *Tailer) sendBacklog(ctx context.Context, ch chan<- string, fileSize int64) int64 {
	f, err := os.Open(t.path)
	if err != nil {
		return 0
	}
	defer f.Close()

	// Read all lines to get the tail
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > t.backlog {
		start = len(lines) - t.backlog
	}
	for _, line := range lines[start:] {
		select {
		case ch <- line:
		case <-ctx.Done():
			return fileSize
		}
	}

	return fileSize
}

// sendNewLines reads new content from offset and sends complete lines. Returns the new offset.
func (t *Tailer) sendNewLines(ctx context.Context, ch chan<- string, offset int64) int64 {
	f, err := os.Open(t.path)
	if err != nil {
		return offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return offset
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		select {
		case ch <- scanner.Text():
		case <-ctx.Done():
			break
		}
	}

	pos, err := f.Seek(0, 1) // current position
	if err != nil {
		return offset
	}
	return pos
}
