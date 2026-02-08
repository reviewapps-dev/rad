package port

import (
	"fmt"
	"net"
	"sync"
)

const (
	minPort = 3001
	maxPort = 9000
)

type Allocator struct {
	mu       sync.Mutex
	assigned map[int]string // port -> app_id
}

func NewAllocator() *Allocator {
	return &Allocator{
		assigned: make(map[int]string),
	}
}

func (a *Allocator) Allocate(appID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if app already has a port
	for port, id := range a.assigned {
		if id == appID {
			return port, nil
		}
	}

	for port := minPort; port <= maxPort; port++ {
		if _, taken := a.assigned[port]; taken {
			continue
		}
		if !isPortAvailable(port) {
			continue
		}
		a.assigned[port] = appID
		return port, nil
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", minPort, maxPort)
}

func (a *Allocator) Release(appID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for port, id := range a.assigned {
		if id == appID {
			delete(a.assigned, port)
			return
		}
	}
}

// Reserve marks a port as assigned to an app. Used on startup to restore
// port assignments from persisted state.
func (a *Allocator) Reserve(appID string, port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assigned[port] = appID
}

func (a *Allocator) GetPort(appID string) (int, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for port, id := range a.assigned {
		if id == appID {
			return port, true
		}
	}
	return 0, false
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
