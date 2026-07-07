package sidecar

import (
	"os"
	"testing"
)

// TestDaemonExeIdentity pins the stale-dev-daemon fix (2026-07-06 live
// incident: two different "dev" builds passed the version handshake, and a
// 3-hour-old daemon silently served a fresh client). A daemon reporting a
// different executable is unhealthy → restart; empty (pre-field) and
// symlink-skewed paths are compatible.
func TestDaemonExeIdentity(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := DaemonClient{}
	if !c.exeMatches(&DaemonPing{ExePath: ""}) {
		t.Error("pre-field daemon (empty exePath) must be accepted")
	}
	if !c.exeMatches(&DaemonPing{ExePath: self}) {
		t.Error("same executable must match")
	}
	if c.exeMatches(&DaemonPing{ExePath: "/tmp/other-ghx-binary"}) {
		t.Error("different executable must NOT match")
	}
	pinned := DaemonClient{ExePath: "/tmp/pinned-ghx"}
	if pinned.exeMatches(&DaemonPing{ExePath: self}) {
		t.Error("pinned client exe vs different daemon exe must NOT match")
	}
}
