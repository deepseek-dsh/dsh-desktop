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

func TestNodeAvailable(t *testing.T) {
	isolatedHome(t)
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

// isolatedHome 把 HOME 指向临时目录, 使 commonBinDirs 不会命中宿主机的真实
// dsh/npm 安装目录, 保证 npx 回退测试可重复、不受开发机环境影响。
func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeBin 在 binDir 里写入一个可执行占位脚本, 返回其路径。
func writeBin(t *testing.T, binDir, name string) string {
	t.Helper()
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewDshCommandFallsBackToNpx(t *testing.T) {
	isolatedHome(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	// 只有 npx, 没有 dsh: 应回退到 npx --yes dsh。
	writeBin(t, binDir, "npx")

	if !IsInstalled() {
		t.Fatal("want IsInstalled true when npx can pull dsh")
	}
	cmd, err := newDshCommand("web", "--port", "1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Path != filepath.Join(binDir, "npx") {
		t.Fatalf("cmd.Path=%q, want npx %q", cmd.Path, filepath.Join(binDir, "npx"))
	}
	wantArgs := []string{"--yes", "dsh", "web", "--port", "1234"}
	if !equalStrings(cmd.Args[1:], wantArgs) {
		t.Fatalf("cmd.Args=%v, want prefix %v", cmd.Args, wantArgs)
	}
}

func TestNewDshCommandErrorsWithoutDshAndNpx(t *testing.T) {
	isolatedHome(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)

	if IsInstalled() {
		t.Fatal("want IsInstalled false when neither dsh nor npx is available")
	}
	if _, err := newDshCommand("web"); err == nil {
		t.Fatal("want error when neither dsh nor npx is available")
	}
}

func TestEnsurePATHAddsCommonDirs(t *testing.T) {
	isolatedHome(t)
	t.Setenv("PATH", "/usr/bin")
	EnsurePATH()
	current := os.Getenv("PATH")
	// 探测 commonBinDirs 是否会命中本机真实存在的常见安装目录;
	// 若命中, 该目录必须已并入 PATH(确保 EnsurePATH 生效)。
	for _, dir := range commonBinDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() &&
			!inPath(current, dir) {
			t.Fatalf("EnsurePATH result %q missing existing common bin dir %q", current, dir)
		}
	}
}

func TestInPathExactMatch(t *testing.T) {
	sep := string(os.PathListSeparator)
	// 精确命中
	if !inPath("/usr/local/bin"+sep+"/usr/bin", "/usr/local/bin") {
		t.Fatal("want true for exact match segment")
	}
	// 前缀不匹配: /usr/local/bin 不应命中 /usr/local/bin2
	if inPath("/usr/local/bin2"+sep+"/usr/bin", "/usr/local/bin") {
		t.Fatal("want false for substring/prefix-only match")
	}
	// 空目录返回 false
	if inPath("/usr/bin", "") {
		t.Fatal("want false for empty dir")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
