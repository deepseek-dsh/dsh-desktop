package cfg

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesSystemDshHomeByDefault(t *testing.T) {
	home := t.TempDir()

	got, err := resolveDshHome(home, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".dsh")
	if got != want {
		t.Fatalf("DSH_HOME=%q, want %q", got, want)
	}
}

func TestLoadRespectsDshHomeOverride(t *testing.T) {
	home := t.TempDir()

	got, err := resolveDshHome(home, "~/shared-dsh")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "shared-dsh")
	if got != want {
		t.Fatalf("DSH_HOME=%q, want %q", got, want)
	}
}
