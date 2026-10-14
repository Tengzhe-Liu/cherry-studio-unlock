package cherry

import (
	"os/exec"
	"runtime"
	"strings"
)

// 进程名在不同平台/安装方式下略有差异，全部覆盖。
var processNames = []string{"Cherry Studio", "CherryStudio", "cherry-studio"}

// cherryStudioPIDs 返回正在运行的 Cherry Studio 进程号。
// 拿不到时返回 nil（不报错）——「是否在运行」只是一个安全提示，
// 探测失败不应该让整个工具不可用。
func cherryStudioPIDs() []string {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
		raw, err := cmd.Output()
		if err != nil {
			return nil
		}
		return parseTasklistCSV(string(raw))
	default:
		var pids []string
		for _, name := range processNames {
			cmd := exec.Command("pgrep", "-f", name)
			raw, err := cmd.Output()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(raw), "\n") {
				if pid := strings.TrimSpace(line); pid != "" {
					pids = append(pids, pid)
				}
			}
		}
		return pids
	}
}

// parseTasklistCSV 从 `tasklist /FO CSV /NH` 的输出里挑出 Cherry Studio 的 PID。
// 形如："Cherry Studio.exe","14212","Console","1","379,448 K"
func parseTasklistCSV(out string) []string {
	var pids []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, `"`) {
			continue
		}
		fields := strings.Split(line, `","`)
		if len(fields) < 2 {
			continue
		}
		image := strings.Trim(fields[0], `"`)
		if !isCherryStudioImage(image) {
			continue
		}
		pids = append(pids, strings.Trim(fields[1], `"`))
	}
	return pids
}

func isCherryStudioImage(image string) bool {
	base := strings.TrimSuffix(image, ".exe")
	for _, name := range processNames {
		if strings.EqualFold(base, name) {
			return true
		}
	}
	return false
}

// IsRunning 报告 Cherry Studio 当前是否在运行。
func IsRunning() bool { return len(cherryStudioPIDs()) > 0 }
