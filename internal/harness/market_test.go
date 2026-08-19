package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureBuildAllowlist(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "profiles", "web")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wsFile := filepath.Join(wsDir, "pnpm-workspace.yaml")
	orig := "packages:\n  - .\nnodeLinker: hoisted\nautoInstallPeers: false\n"
	if err := os.WriteFile(wsFile, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	ensureBuildAllowlist(dir)

	got, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	// 原内容保留, 不以截断破坏 dsh 配置。
	if !strings.HasPrefix(s, "packages:") {
		t.Fatalf("原配置被打乱: %s", s)
	}
	// allowBuilds 块与白名单键已写入。
	for _, pkg := range AllowlistedBuildScripts {
		if !strings.Contains(s, pkg+": true") {
			t.Fatalf("白名单缺少 %s: \n%s", pkg, s)
		}
	}

	// 幂等: 再次调用不重复追加。
	ensureBuildAllowlist(dir)
	got2, _ := os.ReadFile(wsFile)
	if strings.Count(string(got2), "allowBuilds:") != 1 {
		t.Fatalf("allowBuilds 应只出现一次, 出现 %d 次", strings.Count(string(got2), "allowBuilds:"))
	}
}

func TestPreinstalledPluginsIncludesConfiguredUsagePlugin(t *testing.T) {
	t.Setenv(UsagePluginSourceEnv, "/tmp/dsh-plugins/packages/usage")

	plugins := preinstalledPlugins()
	got := plugins[len(plugins)-1]
	if got.Name != "@dsh-plugins/usage" || got.Source != "/tmp/dsh-plugins/packages/usage" {
		t.Fatalf("本地用量插件配置不正确: %+v", got)
	}
}

func TestPreinstalledPluginsOmitsUnconfiguredUsagePlugin(t *testing.T) {
	t.Setenv(UsagePluginSourceEnv, "   ")

	plugins := preinstalledPlugins()
	if len(plugins) != len(DefaultPlugins) {
		t.Fatalf("未配置时不应增加用量插件: %+v", plugins)
	}
}
