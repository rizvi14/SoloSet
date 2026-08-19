// Package system holds cross-platform helpers for talking to the host OS:
// opening the browser now, and (in later milestones) locating and installing
// Docker.
package system

import (
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the user's default browser. It returns any error
// from launching the opener; a nil error does not guarantee the browser
// actually appeared, only that the command started.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// "start" is a cmd builtin; the empty first argument is the window
		// title, which "start" would otherwise consume from a quoted URL.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux and friends
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
