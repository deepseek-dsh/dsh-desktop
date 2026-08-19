package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDshCommandPrefersSystemCommand(t *testing.T) {
	binDir := t.TempDir()
	dshPath := filepath.Join(binDir, "dsh")
	if err := os.WriteFile(dshPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	cmd := newDshCommand("web", "--port", "1234")
	if cmd.Path != dshPath {
		t.Fatalf("launcher=%q, want system dsh %q", cmd.Path, dshPath)
	}
	if len(cmd.Args) != 4 || cmd.Args[1] != "web" {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}
}

func TestNewDshCommandFallsBackToNpx(t *testing.T) {
	binDir := t.TempDir()
	npxPath := filepath.Join(binDir, "npx")
	if err := os.WriteFile(npxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	cmd := newDshCommand("web")
	if cmd.Path != npxPath {
		t.Fatalf("launcher=%q, want npx %q", cmd.Path, npxPath)
	}
	if len(cmd.Args) != 4 || cmd.Args[1] != "--yes" || cmd.Args[2] != DshPackage || cmd.Args[3] != "web" {
		t.Fatalf("unexpected fallback args: %#v", cmd.Args)
	}
}
