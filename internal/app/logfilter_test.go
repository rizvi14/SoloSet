package app

import "testing"

func TestKeepComposeLine(t *testing.T) {
	drop := []string{
		" 41a5d067467d Downloading 23.07MB",
		" 57fc3cc49dc3 Download complete 0B",
		" 4f4fb700ef54 Pulling fs layer 0B",
		" 9bac17bb69cc Extracting 5.243MB",
		" d63a43b4783d Already exists",
		"   ",
	}
	for _, line := range drop {
		if keepComposeLine(line) {
			t.Errorf("expected to drop noisy line: %q", line)
		}
	}

	keep := []string{
		" Image apache/superset:6.0.0 Pulling",
		" Container soloset-superset Created",
		" Container soloset-superset Started",
		"failed to pull image: network error",
		"Error response from daemon: something",
	}
	for _, line := range keep {
		if !keepComposeLine(line) {
			t.Errorf("expected to keep meaningful line: %q", line)
		}
	}
}
