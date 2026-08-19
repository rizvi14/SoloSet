// Package app orchestrates SoloSet's launch flow: probe Docker, then (M3) bring
// up Superset, driving the shared status Store that the web UI renders.
package app

import (
	"context"
	"sync"

	"github.com/rizvi14/SoloSet/internal/docker"
	"github.com/rizvi14/SoloSet/internal/web"
)

// Launcher runs the end-to-end flow and exposes the actions the status page can
// trigger (retry, install Docker). Its methods are safe to call from HTTP
// handlers; long-running work is run on its own goroutine so handlers return
// immediately.
type Launcher struct {
	store *web.Store
	ctx   context.Context

	mu      sync.Mutex // serializes flow runs so a retry can't overlap one in progress
	running bool
}

// NewLauncher builds a Launcher bound to a base context used for all spawned
// work (cancelled on shutdown).
func NewLauncher(ctx context.Context, store *web.Store) *Launcher {
	return &Launcher{store: store, ctx: ctx}
}

// Start kicks off the flow in the background.
func (l *Launcher) Start() { l.run() }

// Retry re-runs the flow, used by the "retry" buttons after the user starts
// Docker or installs it.
func (l *Launcher) Retry() { l.run() }

// InstallDocker is wired up in M4. For now it tells the user to install Docker
// manually rather than leaving the button silently dead.
func (l *Launcher) InstallDocker() {
	l.store.AppendLog("Automatic Docker install isn't available yet in this build.")
	l.store.Set(web.PhaseDockerMissing, "Install Docker Desktop, then retry.",
		"Automatic install is coming soon. For now, install Docker Desktop from docker.com, start it, then click retry.")
}

// run executes the flow once, guarding against concurrent/overlapping runs.
func (l *Launcher) run() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.mu.Unlock()

	go func() {
		defer func() {
			l.mu.Lock()
			l.running = false
			l.mu.Unlock()
		}()
		l.detectDocker()
	}()
}

// detectDocker probes Docker and routes to the right phase. When Docker is
// running it proceeds to launch Superset (a stub until M3).
func (l *Launcher) detectDocker() {
	l.store.Set(web.PhaseCheckingDocker, "Checking Docker…", "Looking for Docker and its daemon.")
	l.store.AppendLog("Checking Docker…")

	res := docker.Detect(l.ctx)
	switch res.State {
	case docker.StateRunning:
		l.store.AppendLog("Docker is running (server " + res.Version + ").")
		l.launchSuperset()
	case docker.StateStopped:
		if res.Detail != "" {
			l.store.AppendLog("Docker daemon not responding: " + res.Detail)
		}
		l.store.Set(web.PhaseDockerStopped, "Docker isn’t running.",
			"Docker is installed, but its daemon isn’t responding. Start Docker Desktop and wait for it to say “running,” then retry.")
	case docker.StateMissing:
		l.store.AppendLog("Docker was not found on this system.")
		l.store.Set(web.PhaseDockerMissing, "Docker isn’t installed.",
			"SoloSet uses Docker to run Superset. Install Docker Desktop, then retry.")
	}
}

// launchSuperset is filled in by M3. For now it confirms Docker is ready.
func (l *Launcher) launchSuperset() {
	l.store.Set(web.PhaseStarting, "Docker is ready.",
		"Superset launch lands in the next build — Docker detection is working.")
}
