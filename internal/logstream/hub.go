package logstream

import "sync"

// Hub is a pub/sub hub for build log lines during deploys.
// Subscribers receive lines on a buffered channel; slow consumers
// have lines dropped rather than blocking the deploy pipeline.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan string]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan string]struct{}),
	}
}

// Subscribe returns a channel that receives build log lines for the given app,
// and an unsubscribe function. The channel is buffered (64 lines).
func (h *Hub) Subscribe(appID string) (<-chan string, func()) {
	ch := make(chan string, 64)
	h.mu.Lock()
	if h.subs[appID] == nil {
		h.subs[appID] = make(map[chan string]struct{})
	}
	h.subs[appID][ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.subs[appID], ch)
		if len(h.subs[appID]) == 0 {
			delete(h.subs, appID)
		}
		h.mu.Unlock()
	}
	return ch, unsub
}

// Publish sends a line to all subscribers for the given app.
// Non-blocking: drops lines for slow consumers.
func (h *Hub) Publish(appID, line string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[appID] {
		select {
		case ch <- line:
		default:
		}
	}
}

// Close closes all subscriber channels for the given app,
// signaling that the deploy has finished.
func (h *Hub) Close(appID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[appID] {
		close(ch)
	}
	delete(h.subs, appID)
}
