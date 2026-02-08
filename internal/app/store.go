package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu        sync.RWMutex
	apps      map[string]*AppState
	statePath string // path to state.json for persistence
}

func NewStore(statePath string) *Store {
	s := &Store{
		apps:      make(map[string]*AppState),
		statePath: statePath,
	}
	if statePath != "" {
		s.load()
	}
	return s
}

func (s *Store) Put(state *AppState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.UpdatedAt = time.Now()
	s.apps[state.AppID] = state
	s.persistLocked()
}

func (s *Store) Get(appID string) (*AppState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.apps[appID]
	if !ok {
		return nil, fmt.Errorf("app %q not found", appID)
	}
	return state, nil
}

func (s *Store) Delete(appID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.apps[appID]; !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	delete(s.apps, appID)
	s.persistLocked()
	return nil
}

func (s *Store) List() []*AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AppState, 0, len(s.apps))
	for _, state := range s.apps {
		result = append(result, state)
	}
	return result
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.apps)
}

func (s *Store) UpdateStatus(appID string, status Status, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.apps[appID]
	if !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	state.Status = status
	state.Error = errMsg
	state.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

func (s *Store) SetPort(appID string, port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.apps[appID]
	if !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	state.Port = port
	state.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

func (s *Store) SetPID(appID string, pid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.apps[appID]
	if !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	state.PID = pid
	state.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

func (s *Store) SetProcess(appID string, proc ProcessInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.apps[appID]
	if !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	if state.Processes == nil {
		state.Processes = make(map[string]ProcessInfo)
	}
	state.Processes[proc.Name] = proc
	// Keep PID pointing at the web process for backward compat
	if proc.Name == "web" {
		state.PID = proc.PID
	}
	state.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

func (s *Store) ClearProcesses(appID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.apps[appID]
	if !ok {
		return fmt.Errorf("app %q not found", appID)
	}
	state.Processes = make(map[string]ProcessInfo)
	state.PID = 0
	state.UpdatedAt = time.Now()
	s.persistLocked()
	return nil
}

func (s *Store) AppendLog(appID string, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.apps[appID]; ok {
		state.BuildLog = append(state.BuildLog, line)
	}
	// Don't persist on every log line — too noisy. Build logs are ephemeral.
}

// load reads persisted state from disk. Called once at startup.
func (s *Store) load() {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("store: load %s: %v", s.statePath, err)
		}
		return
	}

	var apps map[string]*AppState
	if err := json.Unmarshal(data, &apps); err != nil {
		log.Printf("store: parse %s: %v", s.statePath, err)
		return
	}

	s.apps = apps
	log.Printf("store: loaded %d app(s) from %s", len(apps), s.statePath)
}

// persistLocked writes current state to disk. Must be called with mu held.
func (s *Store) persistLocked() {
	if s.statePath == "" {
		return
	}

	data, err := json.MarshalIndent(s.apps, "", "  ")
	if err != nil {
		log.Printf("store: marshal: %v", err)
		return
	}

	// Write atomically via temp file
	dir := filepath.Dir(s.statePath)
	tmp, err := os.CreateTemp(dir, "state-*.json")
	if err != nil {
		log.Printf("store: create temp: %v", err)
		return
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		log.Printf("store: write temp: %v", err)
		return
	}
	tmp.Close()

	if err := os.Rename(tmp.Name(), s.statePath); err != nil {
		os.Remove(tmp.Name())
		log.Printf("store: rename: %v", err)
	}
}
