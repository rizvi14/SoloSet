// Package docker locates the Docker CLI, checks whether its daemon is running,
// and (in later milestones) drives docker compose to run Superset.
package docker

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// State describes what we found when probing for Docker.
type State int

const (
	StateMissing State = iota // docker executable not found on PATH
	StateStopped              // docker present, but the daemon isn't responding
	StateRunning              // daemon is up and responding
)

// Result is the outcome of a detection probe.
type Result struct {
	State   State
	Version string // Docker server version, when running
	Detail  string // raw output/error, surfaced in the activity log
}

// infoTimeout bounds `docker info`, which can hang for a while when Docker
// Desktop is mid-startup.
const infoTimeout = 20 * time.Second

// Detect probes for Docker and reports whether it is missing, stopped, or
// running. It never returns an error: every outcome maps to a State the UI
// can act on.
func Detect(ctx context.Context) Result {
	path, err := exec.LookPath("docker")
	if err != nil {
		return Result{State: StateMissing, Detail: "docker executable not found on PATH"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()

	// Asking for the server version in one shot both confirms the daemon is up
	// and gives us something to show. If the daemon is down, docker exits
	// non-zero and prints the reason, which we keep for the log.
	out, err := exec.CommandContext(probeCtx, path, "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		return Result{State: StateStopped, Detail: firstLine(trimmed)}
	}
	return Result{State: StateRunning, Version: trimmed}
}

// firstLine keeps log entries tidy when docker prints a multi-line error.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
