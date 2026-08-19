package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"
)

//go:embed static
var staticFS embed.FS

// Actions are the callbacks the status page can trigger. Any nil callback is
// treated as a no-op, so the server works even before a milestone wires one up.
type Actions struct {
	Retry         func()
	InstallDocker func()
}

// Server exposes the SoloSet status UI and its backing API over local HTTP.
type Server struct {
	store   *Store
	actions Actions
	mux     *http.ServeMux
}

// NewServer wires up the routes around the given Store and action callbacks.
func NewServer(store *Store, actions Actions) *Server {
	s := &Server{store: store, actions: actions, mux: http.NewServeMux()}

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Embedded FS path is a compile-time constant; this cannot fail in a
		// correctly built binary.
		panic(err)
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/retry", s.action(actions.Retry))
	s.mux.HandleFunc("/api/install-docker", s.action(actions.InstallDocker))

	return s
}

// action returns a POST-only handler that invokes fn (a nil fn is a clean
// no-op) and replies 202.
func (s *Server) action(fn func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if fn != nil {
			fn()
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// Handler returns the root HTTP handler for embedding in an http.Server.
func (s *Server) Handler() http.Handler { return s.mux }

// handleStatus returns the current status snapshot as JSON. The status page
// fetches this on load and as an SSE fallback.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.store.Snapshot())
}

// handleEvents streams status updates to the page via Server-Sent Events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsubscribe := s.store.Subscribe()
	defer unsubscribe()

	// Send the current snapshot immediately so a fresh connection is in sync.
	writeEvent(w, flusher, s.store.Snapshot())

	// Periodic comments keep proxies and the browser from idling the stream out.
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case snap, open := <-ch:
			if !open {
				return
			}
			writeEvent(w, flusher, snap)
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, snap Status) {
	payload, err := json.Marshal(snap)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
