package web

import (
	"testing"
	"time"
)

func TestStore_SetUpdatesSnapshot(t *testing.T) {
	s := NewStore()
	s.Set(PhaseCheckingDocker, "Checking Docker…", "detail")

	snap := s.Snapshot()
	if snap.Phase != PhaseCheckingDocker {
		t.Errorf("phase = %q, want %q", snap.Phase, PhaseCheckingDocker)
	}
	if snap.Message != "Checking Docker…" {
		t.Errorf("message = %q", snap.Message)
	}
}

func TestStore_SetReadyRecordsCredentials(t *testing.T) {
	s := NewStore()
	s.SetReady("http://localhost:8088", "admin", "admin")

	snap := s.Snapshot()
	if snap.Phase != PhaseReady {
		t.Errorf("phase = %q, want ready", snap.Phase)
	}
	if snap.SupersetURL != "http://localhost:8088" || snap.Username != "admin" || snap.Password != "admin" {
		t.Errorf("credentials not recorded: %+v", snap)
	}
}

func TestStore_AppendLogIsBounded(t *testing.T) {
	s := NewStore()
	for i := 0; i < 600; i++ {
		s.AppendLog("line")
	}
	if got := len(s.Snapshot().Logs); got > 500 {
		t.Errorf("log length = %d, want <= 500", got)
	}
}

func TestStore_SnapshotIsDefensiveCopy(t *testing.T) {
	s := NewStore()
	s.AppendLog("one")
	snap := s.Snapshot()
	snap.Logs[0] = "mutated"

	if s.Snapshot().Logs[0] != "one" {
		t.Error("mutating a snapshot's logs leaked back into the store")
	}
}

func TestStore_SubscribeReceivesUpdates(t *testing.T) {
	s := NewStore()
	ch, unsubscribe := s.Subscribe()
	defer unsubscribe()

	s.Set(PhaseStarting, "Starting…", "")

	select {
	case snap := <-ch:
		if snap.Phase != PhaseStarting {
			t.Errorf("received phase %q, want starting", snap.Phase)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive an update")
	}
}

func TestStore_UnsubscribeStopsDelivery(t *testing.T) {
	s := NewStore()
	ch, unsubscribe := s.Subscribe()
	unsubscribe()

	// Channel should be closed; a further Set must not panic on a closed chan.
	s.Set(PhaseError, "boom", "")
	if _, open := <-ch; open {
		t.Error("expected channel to be closed after unsubscribe")
	}
}
