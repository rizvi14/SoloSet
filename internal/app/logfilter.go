package app

import "strings"

// noisyComposeMarkers are the per-layer progress phrases `docker compose up`
// prints by the hundreds while pulling an image. We drop these from the
// activity log and keep the higher-level lines (image pulling/pulled, container
// created/started, and anything that looks like an error).
var noisyComposeMarkers = []string{
	"Pulling fs layer",
	"Waiting",
	"Downloading",
	"Verifying Checksum",
	"Download complete",
	"Extracting",
	"Pull complete",
	"Already exists",
}

// keepComposeLine reports whether a line of compose output is worth logging.
func keepComposeLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// Always keep anything that smells like a failure, even if it also contains
	// a noisy word.
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
		return true
	}
	for _, m := range noisyComposeMarkers {
		if strings.Contains(trimmed, m) {
			return false
		}
	}
	return true
}
