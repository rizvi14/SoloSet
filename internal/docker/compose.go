package docker

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rizvi14/SoloSet/internal/system"
)

//go:embed assets/compose.yaml
var composeYAML []byte

// projectName namespaces every Docker resource SoloSet creates so we can bring
// the whole stack up and down as a unit and never collide with the user's other
// containers.
const projectName = "soloset"

// The one service and its container name, kept in sync with assets/compose.yaml.
const (
	serviceName   = "superset"
	containerName = "soloset-superset"
)

// Compose drives `docker compose` against SoloSet's embedded, self-contained
// stack. The embedded YAML is written to the config dir at construction so the
// docker CLI has a real file to read.
type Compose struct {
	filePath string
	env      []string
}

// NewCompose extracts the embedded compose file to disk and prepares the
// environment (inherited PATH etc. plus the Superset secret key) for every
// docker invocation.
func NewCompose(secretKey string) (*Compose, error) {
	dir, err := system.ConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, composeYAML, 0o644); err != nil {
		return nil, err
	}
	env := append(os.Environ(), "SUPERSET_SECRET_KEY="+secretKey)
	return &Compose{filePath: path, env: env}, nil
}

// LineFunc receives one line of command output at a time.
type LineFunc func(line string)

// command builds a `docker compose -p <project> -f <file> …` invocation.
func (c *Compose) command(ctx context.Context, args ...string) *exec.Cmd {
	full := append([]string{"compose", "-p", projectName, "-f", c.filePath}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	cmd.Env = c.env
	return cmd
}

// Up starts the stack in the background (detached), streaming progress.
func (c *Compose) Up(ctx context.Context, onLine LineFunc) error {
	return runStreaming(c.command(ctx, "up", "-d"), onLine)
}

// Exec runs a command inside the running Superset container (no TTY), streaming
// output. Used for the one-time init steps.
func (c *Compose) Exec(ctx context.Context, onLine LineFunc, args ...string) error {
	full := append([]string{"exec", "-T", serviceName}, args...)
	return runStreaming(c.command(ctx, full...), onLine)
}

// IsRunning reports whether SoloSet's Superset container is currently running.
func (c *Compose) IsRunning(ctx context.Context) bool {
	out, err := c.command(ctx, "ps", "--status", "running", "-q").Output()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(out)) > 0
}

// Stop stops the containers but keeps the data volume, so a later start is fast.
func (c *Compose) Stop(ctx context.Context, onLine LineFunc) error {
	return runStreaming(c.command(ctx, "stop"), onLine)
}

// Down removes the containers. When removeData is true the metadata volume
// (dashboards, examples, users) is deleted too — a full reset.
func (c *Compose) Down(ctx context.Context, onLine LineFunc, removeData bool) error {
	args := []string{"down"}
	if removeData {
		args = append(args, "-v")
	}
	return runStreaming(c.command(ctx, args...), onLine)
}

// runStreaming runs cmd, forwarding merged stdout+stderr to onLine line by line.
func runStreaming(cmd *exec.Cmd, onLine LineFunc) error {
	if onLine == nil {
		onLine = func(string) {}
	}
	lw := &lineWriter{emit: onLine}
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.flush()
	return err
}

// lineWriter accumulates bytes and emits one callback per complete line. It is
// safe for the concurrent writes that exec makes to stdout and stderr.
type lineWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	emit LineFunc
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// No newline yet; put the partial back and wait for more.
			w.buf.Reset()
			w.buf.WriteString(line)
			break
		}
		w.emit(trimEOL(line))
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	rest := w.buf.String()
	w.buf.Reset()
	if s := trimEOL(rest); s != "" {
		w.emit(s)
	}
}

func trimEOL(s string) string {
	return strings.TrimRight(s, "\r\n")
}
