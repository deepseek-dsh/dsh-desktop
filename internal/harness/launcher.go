package harness

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// commonBinDirs 返回桌面启动器常缺 PATH 的 dsh/node 安装目录。
// 从终端启动 PATH 完整, 但从桌面快捷方式启动时需显式补充。
// 按平台给出常见安装目录: 除用户级 npm/nvm 目录外, 也纳入系统级、
// Homebrew 与 Windows 全局安装目录, 覆盖非 nvm 场景下 dsh 落位路径。
func commonBinDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	patterns := []string{
		filepath.Join(home, ".cache", "dsh-global", "bin"),
		filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".nvm", "versions", "node", "*", "bin"),
	}
	if runtime.GOOS == "windows" {
		// Windows 全局 npm bin 目录, %APPDATA% 由环境变量注入。
		patterns = append(patterns, filepath.Join(os.Getenv("APPDATA"), "npm"))
		patterns = append(patterns, filepath.Join(os.Getenv("ProgramFiles"), "nodejs"))
		patterns = append(patterns, filepath.Join(os.Getenv("ProgramFiles(x86)"), "nodejs"))
	} else {
		patterns = append(patterns,
			"/usr/local/bin",
			"/opt/homebrew/bin",
			"/opt/homebrew/Cellar/node/*/bin",
			"/usr/local/nvm/versions/node/*/bin",
		)
	}
	var dirs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			dirs = append(dirs, matches...)
		}
	}
	dirs = orderNvmNewestFirst(dirs)
	return dedupe(dirs)
}

// orderNvmNewestFirst 把 nvm 版本目录(v{整})按版本降序排到最前,
// 保证并入 PATH 时新版 node 优先, 避免 dsh 误用旧版 Node 而无法启动。
// 非 nvm 形态的目录保持原顺序。
func orderNvmNewestFirst(dirs []string) []string {
	nvm := make([]string, 0, len(dirs))
	rest := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		base := filepath.Base(dir)
		if filepath.Base(filepath.Dir(filepath.Dir(dir))) == "node" && len(base) > 1 && base[0] == 'v' {
			nvm = append(nvm, dir)
			continue
		}
		rest = append(rest, dir)
	}
	sort.Slice(nvm, func(i, j int) bool {
		return versionGreater(filepath.Base(nvm[i]), filepath.Base(nvm[j]))
	})
	return append(nvm, rest...)
}

// versionGreater 按 主.次.修订 数值比较两个 v 开头的版本, 返回 a>b。
func versionGreater(a, b string) bool {
	pa := splitVersion(a)
	pb := splitVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

// splitVersion 把 v24.15.0 解析为 [24,15,0]。
func splitVersion(v string) [3]int {
	var out [3]int
	rest := strings.TrimPrefix(v, "v")
	for i := 0; i < 3; i++ {
		num := rest
		if j := strings.IndexByte(rest, '.'); j >= 0 {
			num = rest[:j]
			rest = rest[j+1:]
		}
		n, _ := strconv.Atoi(num)
		out[i] = n
	}
	return out
}

// dedupe 保持顺序去除重复目录, 避免同一安装目录被重复探测。
func dedupe(dirs []string) []string {
	seen := make(map[string]bool, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

// EnsurePATH 把 dsh/node 的常见安装目录并入进程 PATH, 保证检测与子进程可用。
// 新目录追加到末尾而不覆盖已存在的目录, 避免把旧版 nvm 的 node 误排到新版之前,
// 否则 dsh 会用错 Node 版本而无法启动。
func EnsurePATH() {
	current := os.Getenv("PATH")
	for _, dir := range commonBinDirs() {
		if inPath(current, dir) {
			continue
		}
		current = current + string(os.PathListSeparator) + dir
	}
	os.Setenv("PATH", current)
}

// inPath 判断 dir 是否已精确出现在以 PathListSeparator 分隔的 PATH 里。
// 逐段精确比较, 避免子串误判(如 /usr/local/bin 命中 /usr/local/bin2);
// Windows 下路径不区分大小写, 故统一小写比较。
func inPath(pathEnv, dir string) bool {
	if dir == "" {
		return false
	}
	sep := string(os.PathListSeparator)
	lowerDir := strings.ToLower(filepath.Clean(dir))
	for _, part := range strings.Split(pathEnv, sep) {
		if strings.ToLower(filepath.Clean(part)) == lowerDir {
			return true
		}
	}
	return false
}

// lookupInPaths 在 PATH 与常见安装目录中查找可执行文件, 返回完整路径。
func lookupInPaths(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range commonBinDirs() {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		// Unix 需校验可执行位; Windows 下 os.Stat 不设执行位, 存在即视为可用。
		if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
			continue
		}
		return path
	}
	return ""
}

// dshLauncher 返回调用 dsh 的命令名与参数前缀。
// 优先使用系统安装的 dsh; 未安装但存在 npm 时回退到 npx 按需拉取,
// 以保证在未预装 dsh 的干净机器上也能运行(对应 README 的 npx 回退策略)。
// 两者都不可用返回 "", 由上层提示用户安装 Node.js 与 dsh。
func dshLauncher() (binary string, args []string) {
	if path := lookupInPaths("dsh"); path != "" {
		return path, nil
	}
	if npx := lookupInPaths("npx"); npx != "" {
		return npx, []string{"--yes", "dsh"}
	}
	return "", nil
}

// IsInstalled 检测本机是否具备运行 dsh 的能力(系统 dsh 或可经 npx 拉取)。
func IsInstalled() bool {
	bin, _ := dshLauncher()
	return bin != ""
}

// LaunchMode 返回用于运行 dsh 的方式: "dsh" 表示系统已安装, "npx" 表示经
// npx 按需拉取, 空串表示不可用。供启动页在文案上区分真实安装与 npx 拉起。
func LaunchMode() string {
	bin, prefix := dshLauncher()
	switch {
	case bin == "":
		return ""
	case len(prefix) > 0:
		return "npx"
	default:
		return "dsh"
	}
}

// NodeAvailable 检测系统是否已安装 npm(Node.js), 用于未装 harness 时给出安装指引。
func NodeAvailable() bool {
	return lookupInPaths("npm") != ""
}

// newDshCommand 返回执行 dsh 命令的 exec.Cmd。优先系统 dsh, 缺失时经 npx 回退
// (首次会自动下载), 仍不可用则返回错误, 由上层提示用户自行安装。
func newDshCommand(args ...string) (*exec.Cmd, error) {
	bin, prefix := dshLauncher()
	if bin == "" {
		return nil, errors.New("dsh 未安装(且未找到 npm/npx, 请先安装 Node.js)")
	}
	// exec.LookPath 已解析 bin 为绝对路径, 但 npx 场景下需显式带回 prefix。
	return exec.Command(bin, append(prefix, args...)...), nil
}
