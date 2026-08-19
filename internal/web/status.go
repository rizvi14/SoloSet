package web

import (
	"sync"
	"time"
)

// Phase represents the current stage of the SoloSet launch lifecycle.
// Later milestones fill in the Docker and Superset phases; the status page
// renders differently for each.
type Phase string

const (
	PhaseInit           Phase = "init"             // just started up
	PhaseCheckingDocker Phase = "checking_docker"  // probing for Docker
	PhaseDockerMissing  Phase = "docker_missing"   // Docker not installed
	PhaseInstallingDocker Phase = "installing_docker" // installing Docker Desktop
	PhaseDockerStopped  Phase = "docker_stopped"   // installed but daemon not running
	PhaseStarting       Phase = "starting_superset" // docker compose up in progress
	PhaseReady          Phase = "ready"            // Superset is up and reachable
	PhaseStopping       Phase = "stopping"         // stop in progress
	PhaseStopped        Phase = "stopped"          // user stopped Superset
	PhaseError          Phase = "error"            // something went wrong
)

// Status is the full snapshot of what SoloSet is doing, serialized to the
// status page as JSON.
type Status struct {
	Phase       Phase     `json:"phase"`
	Message     string    `json:"message"`
	Detail      string    `json:"detail,omitempty"`
	Logs        []string  `json:"logs"`
	SupersetURL string    `json:"supersetUrl,omitempty"`
	Username    string    `json:"username,omitempty"`
	Password    string    `json:"password,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Store holds the current Status and fans out changes to any connected
// Server-Sent Events subscribers. It is safe for concurrent use.
type Store struct {
	mu          sync.RWMutex
	status      Status
	subscribers map[chan Status]struct{}
}

// NewStore returns a Store seeded with the initial phase.
func NewStore() *Store {
	return &Store{
		status: Status{
			Phase:     PhaseInit,
			Message:   "Starting SoloSet…",
			Logs:      []string{},
			UpdatedAt: time.Now(),
		},
		subscribers: make(map[chan Status]struct{}),
	}
}

// Snapshot returns a copy of the current status.
func (s *Store) Snapshot() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.copyLocked()
}

// copyLocked returns a defensive copy of the status. Caller must hold the lock.
func (s *Store) copyLocked() Status {
	cp := s.status
	cp.Logs = append([]string(nil), s.status.Logs...)
	return cp
}

// Set replaces the phase/message/detail (leaving logs and Superset info intact)
// and broadcasts the change.
func (s *Store) Set(phase Phase, message, detail string) {
	s.mu.Lock()
	s.status.Phase = phase
	s.status.Message = message
	s.status.Detail = detail
	s.status.UpdatedAt = time.Now()
	snap := s.copyLocked()
	s.mu.Unlock()
	s.broadcast(snap)
}

// SetReady marks Superset as reachable and records how to reach it.
func (s *Store) SetReady(url, username, password string) {
	s.mu.Lock()
	s.status.Phase = PhaseReady
	s.status.Message = "Superset is ready."
	s.status.Detail = ""
	s.status.SupersetURL = url
	s.status.Username = username
	s.status.Password = password
	s.status.UpdatedAt = time.Now()
	snap := s.copyLocked()
	s.mu.Unlock()
	s.broadcast(snap)
}

// AppendLog adds a line to the running log and broadcasts the change.
func (s *Store) AppendLog(line string) {
	s.mu.Lock()
	s.status.Logs = append(s.status.Logs, line)
	// Keep the log bounded so a long-running compose pull can't grow unbounded.
	const maxLogLines = 500
	if len(s.status.Logs) > maxLogLines {
		s.status.Logs = s.status.Logs[len(s.status.Logs)-maxLogLines:]
	}
	s.status.UpdatedAt = time.Now()
	snap := s.copyLocked()
	s.mu.Unlock()
	s.broadcast(snap)
}

// Subscribe registers a channel that receives every subsequent status update.
// The returned function unsubscribes and closes the channel.
func (s *Store) Subscribe() (<-chan Status, func()) {
	ch := make(chan Status, 8)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe
}

// broadcast sends the snapshot to every subscriber without blocking. A slow
// subscriber simply misses an intermediate update; it will still receive later
// ones and can re-fetch the full snapshot via /api/status.
func (s *Store) broadcast(snap Status) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subscribers {
		select {
		case ch <- snap:
		default:
		}
	}
}
