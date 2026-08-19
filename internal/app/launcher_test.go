package app

import "testing"

// TestTryBeginSerializes verifies the busy guard prevents overlapping compose
// operations (the bug where a fast Stop→Start raced two docker commands).
func TestTryBeginSerializes(t *testing.T) {
	l := &Launcher{}

	if !l.tryBegin() {
		t.Fatal("first tryBegin should succeed")
	}
	if l.tryBegin() {
		t.Fatal("second tryBegin should fail while an operation is in flight")
	}

	l.end()
	if !l.tryBegin() {
		t.Fatal("tryBegin should succeed again after end()")
	}
}
