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

	cmd, err := newDshCommand("web", "--port", "1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Path != dshPath {
		t.Fatalf("launcher=%q, want system dsh %q", cmd.Path, dshPath)
	}
	if len(cmd.Args) != 4 || cmd.Args[1] != "web" {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}
}

func TestNewDshCommandErrorsWhenNotInstalled(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)

	if _, err := newDshCommand("web"); err == nil {
		t.Fatal("want error when dsh is not installed")
	}
}

func TestNodeAvailable(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	if NodeAvailable() {
		t.Fatal("want false when npm is not installed")
	}

	npmPath := filepath.Join(binDir, "npm")
	if err := os.WriteFile(npmPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !NodeAvailable() {
		t.Fatal("want true when npm is installed")
	}
}
