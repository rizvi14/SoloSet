package system

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ErrManualInstall means SoloSet can't install Docker automatically on this
// system (no supported package manager); the user must install Docker Desktop
// by hand.
var ErrManualInstall = errors.New("automatic Docker install unavailable")

// DockerDownloadURL is where to get Docker Desktop manually.
const DockerDownloadURL = "https://www.docker.com/products/docker-desktop/"

// InstallDocker installs Docker Desktop using the platform's package manager,
// streaming output line by line. It returns ErrManualInstall when no supported
// installer is available, so the caller can fall back to guiding the user.
func InstallDocker(ctx context.Context, onLine func(string)) error {
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("winget"); err != nil {
			return ErrManualInstall
		}
		return runLines(ctx, onLine, "winget", "install", "--id", "Docker.DockerDesktop",
			"--accept-source-agreements", "--accept-package-agreements")
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return ErrManualInstall
		}
		return runLines(ctx, onLine, "brew", "install", "--cask", "docker")
	default:
		return ErrManualInstall
	}
}

// StartDockerDesktop launches the Docker Desktop app so its daemon starts. Best
// effort: a failure here just means the user starts it themselves.
func StartDockerDesktop() {
	switch runtime.GOOS {
	case "windows":
		const p = `C:\Program Files\Docker\Docker\Docker Desktop.exe`
		if _, err := os.Stat(p); err == nil {
			_ = exec.Command(p).Start()
		}
	case "darwin":
		_ = exec.Command("open", "-a", "Docker").Start()
	}
}

// runLines runs a command, forwarding merged stdout+stderr to onLine one line at
// a time.
func runLines(ctx context.Context, onLine func(string), name string, args ...string) error {
	if onLine == nil {
		onLine = func(string) {}
	}
	cmd := exec.CommandContext(ctx, name, args...)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			onLine(strings.TrimRight(sc.Text(), "\r"))
		}
		close(done)
	}()

	err := cmd.Run()
	_ = pw.Close()
	<-done
	return err
}
