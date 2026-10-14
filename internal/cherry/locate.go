package cherry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cherry-tools/cherry-agent-manager/internal/asar"
)

// Install 描述一份本机 Cherry Studio 安装。
type Install struct {
	// DataDir 是用户数据目录，例如 %APPDATA%\CherryStudio\Data
	DataDir string `json:"dataDir"`
	// DBPath 是 agent 配置所在的 SQLite 数据库。
	DBPath string `json:"dbPath"`
	// AgentsDir 是各个 Agent 的私有数据目录（SOUL.md / USER.md / memory）。
	AgentsDir string `json:"agentsDir"`
	// AsarPath 是程序资源包，运行时注册表就在里面。
	AsarPath string `json:"asarPath"`
	// AppRoot 是安装根目录（app.asar 的上两级）。
	AppRoot string `json:"appRoot"`
	// Version 取自 app.asar 内的 package.json。
	Version string `json:"version"`
	// AsarCandidates 是所有被发现的 app.asar 路径，供用户确认。
	AsarCandidates []string `json:"asarCandidates"`
	// Warnings 记录探测过程中的不确定项，UI 上要如实展示。
	Warnings []string `json:"warnings"`
}

// errNotInstalled 表示本机没找到 Cherry Studio。
var errNotInstalled = errors.New("没有找到 Cherry Studio 的数据目录")

// LocateDataDir 按操作系统惯例定位用户数据目录。
func LocateDataDir() (string, error) {
	var candidates []string

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			candidates = append(candidates, filepath.Join(appData, "CherryStudio", "Data"))
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates,
				filepath.Join(home, "Library", "Application Support", "CherryStudio", "Data"))
		}
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			candidates = append(candidates, filepath.Join(xdg, "CherryStudio", "Data"))
		}
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates,
				filepath.Join(home, ".config", "CherryStudio", "Data"))
		}
	}

	for _, c := range candidates {
		if isDir(c) {
			return c, nil
		}
	}
	if len(candidates) > 0 {
		return "", fmt.Errorf("%w（已尝试：%s）", errNotInstalled, strings.Join(candidates, ", "))
	}
	return "", errNotInstalled
}

// asarCandidates 列出常见的 app.asar 位置。
func asarCandidates() []string {
	var out []string

	switch runtime.GOOS {
	case "windows":
		var roots []string
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			roots = append(roots,
				filepath.Join(local, "Programs", "CherryStudio"),
				filepath.Join(local, "CherryStudio"),
				filepath.Join(local, "Programs", "Cherry Studio"),
			)
		}
		roots = append(roots,
			`C:\Program Files\Cherry Studio`,
			`C:\Program Files\CherryStudio`,
			`C:\Program Files (x86)\Cherry Studio`,
		)
		for _, r := range roots {
			out = append(out, filepath.Join(r, "resources", "app.asar"))
		}
	case "darwin":
		out = append(out,
			"/Applications/Cherry Studio.app/Contents/Resources/app.asar",
			"/Applications/CherryStudio.app/Contents/Resources/app.asar",
		)
	default:
		out = append(out,
			"/opt/CherryStudio/resources/app.asar",
			"/opt/cherry-studio/resources/app.asar",
			"/usr/lib/cherry-studio/resources/app.asar",
		)
		if home, err := os.UserHomeDir(); err == nil {
			out = append(out,
				filepath.Join(home, ".local", "share", "CherryStudio", "resources", "app.asar"),
				filepath.Join(home, "Applications", "CherryStudio", "resources", "app.asar"),
			)
		}
	}
	return out
}

// asarFromRunningProcess 从正在运行的 Cherry Studio 进程反查安装路径。
// 这是最可靠的一招——用户可能装在任意目录（例如 C:\app\AI\Cherry Studio）。
func asarFromRunningProcess() []string {
	pids := cherryStudioPIDs()
	if len(pids) == 0 {
		return nil
	}
	var out []string

	switch runtime.GOOS {
	case "windows":
		// 用 PowerShell 取主模块路径，避免依赖 wmic（新版 Windows 已移除）
		script := `Get-Process -Name 'Cherry Studio','CherryStudio' -ErrorAction SilentlyContinue | ` +
			`Where-Object { $_.Path } | Select-Object -ExpandProperty Path -Unique`
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
		if raw, err := cmd.Output(); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				exe := strings.TrimSpace(line)
				if exe == "" {
					continue
				}
				out = append(out, filepath.Join(filepath.Dir(exe), "resources", "app.asar"))
			}
		}
	case "darwin", "linux":
		for _, pid := range pids {
			link, err := os.Readlink(fmt.Sprintf("/proc/%s/exe", pid))
			if err != nil {
				continue
			}
			// macOS 上可执行文件在 .app/Contents/MacOS/ 下
			out = append(out, filepath.Join(filepath.Dir(link), "..", "Resources", "app.asar"))
		}
	}
	return out
}

// Locate 自动定位本机安装。asar 找不到时不算失败（数据库那条路仍然可用），
// 只是把警告放进 Warnings 里。
func Locate() (*Install, error) {
	ins := &Install{}

	dataDir, err := LocateDataDir()
	if err != nil {
		return nil, err
	}
	ins.DataDir = dataDir
	ins.DBPath = filepath.Join(dataDir, "cherrystudio.sqlite")
	ins.AgentsDir = filepath.Join(dataDir, "Agents")

	if !fileExists(ins.DBPath) {
		return nil, fmt.Errorf("找到了数据目录但没有数据库文件：%s", ins.DBPath)
	}

	// app.asar：先看正在运行的进程，再看常见安装位置
	seen := map[string]bool{}
	var candidates []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		candidates = append(candidates, p)
	}
	for _, p := range asarFromRunningProcess() {
		add(p)
	}
	for _, p := range asarCandidates() {
		add(p)
	}

	ins.AsarCandidates = candidates
	for _, c := range candidates {
		if fileExists(c) {
			ins.AsarPath = c
			ins.AppRoot = filepath.Dir(filepath.Dir(c))
			break
		}
	}
	if ins.AsarPath == "" {
		ins.Warnings = append(ins.Warnings,
			"没有自动找到 app.asar。数据库相关的功能仍然可用，但「运行时扫描」和「打补丁」需要你在界面上手动指定路径。")
	} else if v := readAsarVersion(ins.AsarPath); v != "" {
		ins.Version = v
	}

	return ins, nil
}

// DescribeAsar 根据 app.asar 路径推导安装根目录与版本号。
// 用户用 --asar 手动指定路径时，用它把信息补齐。
func DescribeAsar(asarPath string) (appRoot, version string) {
	return filepath.Dir(filepath.Dir(asarPath)), readAsarVersion(asarPath)
}

func readAsarVersion(asarPath string) string {
	a, err := asar.Open(asarPath)
	if err != nil {
		return ""
	}
	defer a.Close()
	raw, err := a.ReadFile("package.json")
	if err != nil {
		return ""
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ""
	}
	return meta.Version
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
