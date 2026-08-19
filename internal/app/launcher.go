// Package app orchestrates SoloSet's launch flow: probe Docker, then (M3) bring
// up Superset, driving the shared status Store that the web UI renders.
package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rizvi14/SoloSet/internal/docker"
	"github.com/rizvi14/SoloSet/internal/superset"
	"github.com/rizvi14/SoloSet/internal/system"
	"github.com/rizvi14/SoloSet/internal/web"
)

// Launcher runs the end-to-end flow and exposes the actions the status page can
// trigger (retry, install Docker). Its methods are safe to call from HTTP
// handlers; long-running work is run on its own goroutine so handlers return
// immediately.
type Launcher struct {
	store *web.Store
	ctx   context.Context
	quit  func() // asks the process to shut down (Quit button)

	mu      sync.Mutex // guards running and compose
	running bool
	compose *docker.Compose // set once Superset launch begins; used by Stop/Quit
}

// NewLauncher builds a Launcher bound to a base context used for all spawned
// work (cancelled on shutdown). quit is invoked when the user asks to close
// SoloSet.
func NewLauncher(ctx context.Context, store *web.Store, quit func()) *Launcher {
	return &Launcher{store: store, ctx: ctx, quit: quit}
}

// getCompose returns the current compose handle (nil before launch begins).
func (l *Launcher) getCompose() *docker.Compose {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.compose
}

// setCompose stores the compose handle for later Stop/Quit use.
func (l *Launcher) setCompose(c *docker.Compose) {
	l.mu.Lock()
	l.compose = c
	l.mu.Unlock()
}

// Start kicks off the flow in the background.
func (l *Launcher) Start() { l.run() }

// Retry re-runs the whole flow. It doubles as the "Start" action from the
// stopped state: re-detecting Docker and running compose up brings a stopped
// container back (init is skipped) and reaches ready.
func (l *Launcher) Retry() { l.run() }

// Stop stops the Superset container, keeping its data, and moves to the stopped
// state. No-op if Superset was never launched or another operation is running.
func (l *Launcher) Stop() {
	comp := l.getCompose()
	if comp == nil {
		return
	}
	if !l.tryBegin() {
		return // another operation (start/stop/retry) is in flight
	}
	go func() {
		defer l.end()
		l.store.Set(web.PhaseStopping, "Stopping Superset…", "Shutting down the container. Your data is kept.")
		l.store.AppendLog("Stopping Superset…")
		if err := comp.Stop(l.ctx, func(s string) { l.store.AppendLog(s) }); err != nil {
			l.fail("Couldn’t stop Superset.", err)
			return
		}
		l.store.Set(web.PhaseStopped, "Superset is stopped.",
			"Your dashboards and data are saved. Start it again whenever you like.")
	}()
}

// Quit closes SoloSet but deliberately leaves Superset running in Docker, so the
// user keeps their session; a later launch reconnects to it instantly.
func (l *Launcher) Quit() {
	l.store.Set(web.PhaseStopped, "SoloSet closed.",
		"Superset is still running in Docker — you can close this tab. Reopen SoloSet anytime to manage it.")
	l.store.AppendLog("Closing SoloSet (Superset keeps running).")
	if l.quit != nil {
		// Give the HTTP response and a final SSE frame a moment to flush.
		go func() {
			time.Sleep(300 * time.Millisecond)
			l.quit()
		}()
	}
}

// InstallDocker is wired up in M4. For now it tells the user to install Docker
// manually rather than leaving the button silently dead.
func (l *Launcher) InstallDocker() {
	l.store.AppendLog("Automatic Docker install isn't available yet in this build.")
	l.store.Set(web.PhaseDockerMissing, "Install Docker Desktop, then retry.",
		"Automatic install is coming soon. For now, install Docker Desktop from docker.com, start it, then click retry.")
}

// tryBegin marks the launcher busy, returning false if an operation (run/stop)
// is already in flight. This serializes all compose operations so they can't
// race each other.
func (l *Launcher) tryBegin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return false
	}
	l.running = true
	return true
}

// end clears the busy flag.
func (l *Launcher) end() {
	l.mu.Lock()
	l.running = false
	l.mu.Unlock()
}

// run executes the flow once, guarding against concurrent/overlapping runs.
func (l *Launcher) run() {
	if !l.tryBegin() {
		return
	}
	go func() {
		defer l.end()
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

// launchSuperset brings up the Superset container, runs one-time init on first
// launch, waits for it to respond, and marks the app ready.
func (l *Launcher) launchSuperset() {
	l.store.Set(web.PhaseStarting, "Starting Superset…",
		"Preparing the container. The first run downloads the Superset image and example data — this can take several minutes.")

	key, err := system.SecretKey()
	if err != nil {
		l.fail("Could not prepare SoloSet's configuration.", err)
		return
	}
	comp, err := docker.NewCompose(key)
	if err != nil {
		l.fail("Could not prepare the Superset configuration.", err)
		return
	}
	l.setCompose(comp)

	// If our container isn't already the one on 8088, make sure nothing else is
	// holding the port before we try to publish it.
	if !comp.IsRunning(l.ctx) && system.PortInUse("127.0.0.1:8088") {
		l.store.Set(web.PhaseError, "Port 8088 is already in use.",
			"Another program on your computer is using port 8088, which Superset needs. Close it, then retry.")
		return
	}

	logLine := func(s string) {
		if s != "" {
			l.store.AppendLog(s)
		}
	}

	// The pull prints hundreds of per-layer progress lines; keep only the
	// meaningful ones in the activity log.
	pullLog := func(s string) {
		if keepComposeLine(s) {
			l.store.AppendLog(strings.TrimSpace(s))
		}
	}
	l.store.AppendLog("Pulling the Superset image and starting the container…")
	if err := comp.Up(l.ctx, pullLog); err != nil {
		l.fail("Failed to start the Superset container.", err)
		return
	}

	if l.needsInit() {
		if !l.initialize(comp, logLine) {
			return // initialize() already reported the failure
		}
	} else {
		l.store.AppendLog("Superset is already set up — skipping first-time initialization.")
	}

	l.store.Set(web.PhaseStarting, "Almost there…", "Waiting for Superset to respond.")
	if err := superset.WaitHealthy(l.ctx, 3*time.Minute); err != nil {
		l.fail("Superset started but isn’t responding yet.", err)
		return
	}

	l.store.AppendLog("Superset is up.")
	l.store.SetReady(superset.BaseURL, superset.Username, superset.Password)
}

// needsInit reports whether first-time initialization is required, by checking
// for the sentinel file on the data volume.
func (l *Launcher) needsInit() bool {
	// A zero exit means the marker exists (already initialized).
	err := l.compose.Exec(l.ctx, nil, "test", "-f", superset.InitMarker)
	return err != nil
}

// initialize runs the one-time setup: migrate the DB, create the admin user,
// initialize roles, and load example dashboards. Returns false (after reporting)
// if a required step fails.
func (l *Launcher) initialize(comp *docker.Compose, logLine docker.LineFunc) bool {
	l.store.Set(web.PhaseStarting, "Setting up Superset…",
		"One-time setup: creating the database, admin user, and loading example dashboards.")

	step := func(args []string, fatal bool) bool {
		l.store.AppendLog("$ " + strings.Join(args, " "))
		if err := comp.Exec(l.ctx, logLine, args...); err != nil {
			if fatal {
				l.fail("Superset setup failed while running: "+strings.Join(args, " "), err)
				return false
			}
			l.store.AppendLog("(continuing) step reported: " + err.Error())
		}
		return true
	}

	if !step(superset.DBUpgradeArgs(), true) {
		return false
	}
	// create-admin is non-fatal: a re-run after a partial init would find the
	// user already present.
	step(superset.CreateAdminArgs(), false)
	if !step(superset.InitArgs(), true) {
		return false
	}
	// Examples are a nice-to-have; Superset is fully usable without them.
	l.store.AppendLog("Loading example dashboards (downloads sample data)…")
	step(superset.LoadExamplesArgs(), false)

	// Record that init is done so future launches skip it.
	if err := comp.Exec(l.ctx, nil, "touch", superset.InitMarker); err != nil {
		l.store.AppendLog("Note: could not write the setup marker; examples may reload next time.")
	}
	return true
}

// fail records an error phase with the underlying detail in the log.
func (l *Launcher) fail(message string, err error) {
	if err != nil {
		l.store.AppendLog("Error: " + err.Error())
	}
	l.store.Set(web.PhaseError, message, "See the activity log for details, then retry.")
}
