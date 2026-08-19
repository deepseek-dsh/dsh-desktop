//go:build manual

package harness

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPreinstallManual 真实安装全部默认插件到全新目录并验证事件序列与幂等。
// 需要网络与 pnpm; 平时由 CI 跳过, 仅手动验证用。
func TestPreinstallManual(t *testing.T) {
	dir := t.TempDir()

	// skip 开关在全局有效, 临时禁用以便真实安装。
	os.Unsetenv("DSH_DESKTOP_SKIP_PLUGINS")
	type ev struct{ stage, name string }
	var events []ev
	EnsurePreinstalled(dir, func(stage, name string, ok bool) {
		events = append(events, ev{stage, name})
	})

	if len(events) == 0 {
		t.Fatal("应有预装事件产生")
	}
	for _, p := range DefaultPlugins {
		if !PluginInstalled(dir, p.Name) {
			t.Fatalf("插件 %s 安装后应已被探测到", p.Name)
		}
	}

	// 二次调用应幂等(已装即标记 ok, 不重复安装)。
	events = nil
	EnsurePreinstalled(dir, func(stage, name string, ok bool) {
		events = append(events, ev{stage, name})
	})
	for _, e := range events {
		if e.stage == "install" {
			t.Fatalf("已装插件不应再次安装, 出现 install: %+v", e)
		}
	}

	// skip 开关应跳过安装。
	os.Setenv("DSH_DESKTOP_SKIP_PLUGINS", "1")
	defer os.Unsetenv("DSH_DESKTOP_SKIP_PLUGINS")
	dir2 := filepath.Join(dir, "skip-dir")
	events = nil
	EnsurePreinstalled(dir2, func(stage, name string, ok bool) {
		events = append(events, ev{stage, name})
	})
	if len(events) == 0 {
		t.Fatal("skip 模式下也应有 skip 事件")
	}
}
