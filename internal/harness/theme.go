package harness

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ThemePreference 读取 harness 外观设置(settings.yaml 的 ui-theme.preference)。
// 返回 "dark"/"system"/"light"; 配置缺失或无法解析时回退 "light"。
func ThemePreference(workDir string) string {
	data, err := os.ReadFile(filepath.Join(workDir, "settings.yaml"))
	if err != nil {
		return "light"
	}

	var s struct {
		UITheme struct {
			Preference string `yaml:"preference"`
		} `yaml:"ui-theme"`
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		return "light"
	}

	switch s.UITheme.Preference {
	case "dark", "system":
		return s.UITheme.Preference
	default:
		return "light"
	}
}