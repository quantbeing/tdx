package main

import "testing"

func TestProbeCommandsSelectsNamedOperations(t *testing.T) {
	cmds, unknown := probeCommands("security-count,quote")
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v", unknown)
	}
	if len(cmds) != 2 {
		t.Fatalf("len(cmds) = %d, want 2", len(cmds))
	}
	if cmds[0].Operation() != "security_count" || cmds[1].Operation() != "security_quotes" {
		t.Fatalf("operations = %s/%s", cmds[0].Operation(), cmds[1].Operation())
	}
}

func TestProbeCommandsReportsUnknownOperations(t *testing.T) {
	cmds, unknown := probeCommands("security-count,nope")
	if len(cmds) != 1 || len(unknown) != 1 || unknown[0] != "nope" {
		t.Fatalf("cmds=%d unknown=%v", len(cmds), unknown)
	}
}
