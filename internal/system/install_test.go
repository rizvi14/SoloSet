package system

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

// TestRunLines verifies the streaming command runner captures each output line.
func TestRunLines(t *testing.T) {
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "echo line1& echo line2"}
	} else {
		name, args = "sh", []string{"-c", "printf 'line1\\nline2\\n'"}
	}

	var got []string
	err := runLines(context.Background(), func(s string) {
		got = append(got, strings.TrimSpace(s))
	}, name, args...)
	if err != nil {
		t.Fatalf("runLines returned error: %v", err)
	}

	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "line1") || !strings.Contains(joined, "line2") {
		t.Fatalf("expected both lines captured, got %q", got)
	}
}
