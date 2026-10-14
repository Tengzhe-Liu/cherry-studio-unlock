package cherry

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/asar"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/i18n"
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
	// 内容由 warningMsgs 按当前语言渲染而来，见 RenderWarnings。
	Warnings []string `json:"warnings"`

	// warningMsgs 保存的是文案 key 而不是成品句子，
	// 这样切换界面语言后不必重新探测就能换一种语言显示。
	warningMsgs []i18n.Msg
}

// errNotInstalled 表示本机没找到 Cherry Studio。
var errNotInstalled = i18n.E("err.dataDirMissing")

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
		return "", i18n.E("err.dataDirTriedPaths", strings.Join(candidates, ", "))
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
//
// 声明成变量而不是普通函数，是为了让测试能把它替换成"永远返回空"，
// 从而真实复现「Cherry Studio 已关闭」这个场景——
// 那恰恰是本工具唯一允许写入数据的场景，也是最容易定位不到安装目录的场景。
var asarFromRunningProcess = func() []string {
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

// asarFromWindowsRegistry 从注册表的卸载项反查安装目录。
//
// 这是"Cherry Studio 已关闭"时唯一可靠的办法：卸载项的 DisplayIcon 里存着
// "<安装目录>\Cherry Studio.exe,0"，而 InstallLocation 常常是空的。
// 用户把程序装在自定义目录（如 C:\app\AI\Cherry Studio）时，别的招都不管用。
func asarFromWindowsRegistry() []string {
	script := `$ErrorActionPreference='SilentlyContinue'; ` +
		`$p=@('HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',` +
		`'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',` +
		`'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'); ` +
		`Get-ItemProperty $p | Where-Object { $_.DisplayName -like '*Cherry*' } | ` +
		`ForEach-Object { "$($_.InstallLocation)|$($_.DisplayIcon)" }`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}

	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		var dir string
		// InstallLocation 优先
		if loc := strings.TrimSpace(parts[0]); loc != "" && isDir(loc) {
			dir = loc
		} else if len(parts) == 2 {
			dir = dirFromDisplayIcon(parts[1])
		}
		if dir == "" {
			continue
		}
		out = append(out, filepath.Join(dir, "resources", "app.asar"))
	}
	return out
}

// dirFromDisplayIcon 把 `C:\path\Cherry Studio.exe,0` 解析成 `C:\path`。
func dirFromDisplayIcon(icon string) string {
	icon = strings.TrimSpace(icon)
	// 去掉尾部引号与 ",0" 这样的图标序号
	icon = strings.Trim(icon, `"`)
	if i := strings.LastIndex(icon, ","); i > 0 {
		icon = icon[:i]
	}
	icon = strings.Trim(icon, `" `)
	if icon == "" {
		return ""
	}
	if strings.EqualFold(filepath.Ext(icon), ".exe") {
		return filepath.Dir(icon)
	}
	if isDir(icon) {
		return icon
	}
	return ""
}

// asarFromAppBundles 在 macOS 上扫 /Applications 下的 .app，
// 在 Linux 上读 .desktop 文件。都用 glob，避免写死目录名。
func asarFromAppBundles() []string {
	var out []string
	switch runtime.GOOS {
	case "darwin":
		for _, pattern := range []string{"/Applications/*.app", "/Applications/*/*.app"} {
			matches, _ := filepath.Glob(pattern)
			for _, m := range matches {
				if !strings.Contains(strings.ToLower(filepath.Base(m)), "cherry") {
					continue
				}
				out = append(out, filepath.Join(m, "Contents", "Resources", "app.asar"))
			}
		}
	default:
		for _, pattern := range []string{"/usr/share/applications/*.desktop", "/usr/local/share/applications/*.desktop"} {
			matches, _ := filepath.Glob(pattern)
			for _, m := range matches {
				if !strings.Contains(strings.ToLower(filepath.Base(m)), "cherry") {
					continue
				}
				raw, err := os.ReadFile(m)
				if err != nil {
					continue
				}
				for _, line := range strings.Split(string(raw), "\n") {
					if !strings.HasPrefix(line, "Exec=") {
						continue
					}
					exe := strings.Fields(strings.TrimPrefix(line, "Exec="))
					if len(exe) == 0 {
						continue
					}
					// 可执行文件通常在 <安装目录>/cherrystudio 或类似位置
					out = append(out,
						filepath.Join(filepath.Dir(exe[0]), "resources", "app.asar"),
						filepath.Join(filepath.Dir(filepath.Dir(exe[0])), "resources", "app.asar"))
				}
			}
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
		return nil, i18n.E("err.dbMissing", ins.DBPath)
	}

	// app.asar 的查找顺序，按可靠性从高到低：
	//   1. 上次记住的路径 —— Cherry Studio 关闭后仍然有效，这是最关键的兜底
	//   2. 正在运行的进程 —— 需要 App 开着，但能拿到真实安装目录
	//   3. 注册表卸载项     —— App 关着时的可靠来源，能覆盖自定义安装目录
	//   4. 常见安装位置     —— 覆盖标准安装
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
	add(LoadConfig(dataDir).AsarPath)
	for _, p := range asarFromRunningProcess() {
		add(p)
	}
	if runtime.GOOS == "windows" {
		for _, p := range asarFromWindowsRegistry() {
			add(p)
		}
	}
	for _, p := range asarFromAppBundles() {
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
		ins.AddWarning("warn.asarNotFound")
	} else {
		ins.Version = readAsarVersion(ins.AsarPath)
	}

	// 注意：这里不写任何文件。Locate 只负责"探测"，
	// 持久化由调用方在确定最终数据目录之后执行——否则 --data-dir 覆盖会被忽略。
	return ins, nil
}

// DescribeAsar 根据 app.asar 路径推导安装根目录与版本号。
// 用户用 --asar 手动指定路径时，用它把信息补齐。
func DescribeAsar(asarPath string) (appRoot, version string) {
	return filepath.Dir(filepath.Dir(asarPath)), readAsarVersion(asarPath)
}

// AddWarning 记录一条提示（存 key，不存成品句子）。
func (i *Install) AddWarning(key string, args ...any) {
	i.warningMsgs = append(i.warningMsgs, i18n.Msg{Key: key, Args: args})
}

// RenderWarnings 按给定语言生成 Warnings 字段，供界面直接展示。
func (i *Install) RenderWarnings(l i18n.Lang) {
	i.Warnings = i.Warnings[:0]
	for _, m := range i.warningMsgs {
		i.Warnings = append(i.Warnings, m.Text(l))
	}
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
