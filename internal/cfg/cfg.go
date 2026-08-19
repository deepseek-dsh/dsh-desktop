package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPort 是 harness 固定监听的端口, 复用已运行的本地服务。
const DefaultPort = 3080

// EnvDshHome 是 DSH CLI 与桌面端共同使用的数据根目录环境变量。
const EnvDshHome = "DSH_HOME"

// envDataDir 是允许覆盖应用数据根目录的环境变量名。
const envDataDir = "DSH_DESKTOP_DATA_DIR"

// Config 汇总启动所需的全部路径与端口配置。
type Config struct {
	DataDir string // 桌面应用数据根目录
	DshHome string // 与系统 DSH 共用的数据根目录(profiles/sessions/plugins 所在)
	LogDir  string // 桌面应用日志目录
	Port    int    // Harness 监听端口(固定 DefaultPort)
}

// Load 从环境变量加载配置, 数据目录遵循 XDG 规范。
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("读取用户主目录失败: %w", err)
	}

	// 数据根目录: 环境变量 DSH_DESKTOP_DATA_DIR 覆盖, 默认遵循 XDG 规范。
	dataRoot := os.Getenv(envDataDir)
	if len(dataRoot) == 0 {
		dataRoot = filepath.Join(home, ".local", "share", "dsh-desktop")
	}

	// 与命令行 DSH 遵循同一套目录规则：显式 DSH_HOME 优先，否则 ~/.dsh。
	dshHome, err := resolveDshHome(home, os.Getenv(EnvDshHome))
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(dataRoot, "logs")

	cfg := &Config{
		DataDir: dataRoot,
		DshHome: filepath.Clean(dshHome),
		LogDir:  logDir,
		Port:    DefaultPort,
	}

	return cfg, nil
}

func resolveDshHome(home, configured string) (string, error) {
	dshHome := strings.TrimSpace(configured)
	if dshHome == "" {
		return filepath.Join(home, ".dsh"), nil
	}
	dshHome = expandHome(dshHome, home)
	if !filepath.IsAbs(dshHome) {
		absolute, err := filepath.Abs(dshHome)
		if err != nil {
			return "", fmt.Errorf("解析环境变量 %s 失败: %w", EnvDshHome, err)
		}
		dshHome = absolute
	}
	return filepath.Clean(dshHome), nil
}

// EnsureDirs 创建工作所需的目录(数据根/工作区/日志)。
// 独立于 Load 以便 wails 绑定生成阶段(只读环境空跑)也能通过。
func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.DataDir, c.DshHome, c.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}
	return nil
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}
