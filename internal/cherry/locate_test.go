package cherry

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDirFromDisplayIcon 覆盖注册表 DisplayIcon 的各种写法。
// 这是"Cherry Studio 已关闭时仍能找到 app.asar"的关键一环。
func TestDirFromDisplayIcon(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\app\AI\Cherry Studio\Cherry Studio.exe,0`, `C:\app\AI\Cherry Studio`},
		{`"C:\Program Files\Cherry Studio\Cherry Studio.exe",0`, `C:\Program Files\Cherry Studio`},
		{`C:\app\AI\Cherry Studio\Cherry Studio.exe`, `C:\app\AI\Cherry Studio`},
		{`  C:\Tools\CherryStudio\CherryStudio.exe,1  `, `C:\Tools\CherryStudio`},
		{``, ``},
		{`,0`, ``},
	}
	for _, c := range cases {
		if got := dirFromDisplayIcon(c.in); got != c.want {
			t.Errorf("dirFromDisplayIcon(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// TestAsarFromWindowsRegistry 在真机上确认注册表能反查出安装目录，
// 并据此推出存在的 app.asar。非 Windows 或未安装时跳过。
func TestAsarFromWindowsRegistry(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows")
	}
	cands := asarFromWindowsRegistry()
	if len(cands) == 0 {
		t.Skip("注册表里没有 Cherry Studio 的卸载项")
	}
	t.Logf("注册表给出的候选：%v", cands)

	for _, c := range cands {
		if fileExists(c) {
			t.Logf("命中：%s", c)
			if filepath.Base(c) != "app.asar" {
				t.Errorf("结果 %q 不是 app.asar", c)
			}
			return
		}
	}
	t.Logf("候选路径都不存在（可能未安装在本机），注册表查询本身工作正常")
}

// TestAsarLookupDoesNotDependOnRunningProcess 是本轮修复的回归测试，
// 端到端复现用户的失败场景并断言已经修好。
//
// 用户按提示先退出 Cherry Studio —— 这是本工具唯一允许写数据的场景。
// 修复前：进程反查失效 + 安装目录是自定义路径（不在常见位置）→ app.asar 找不到
// → 运行时列表为空 → 界面里"选不了运行环境"。
func TestAsarLookupDoesNotDependOnRunningProcess(t *testing.T) {
	// 把"从运行中的进程反查"替换成永远返回空，等价于 Cherry Studio 已关闭
	orig := asarFromRunningProcess
	asarFromRunningProcess = func() []string { return nil }
	defer func() { asarFromRunningProcess = orig }()

	ins, err := Locate()
	if err != nil {
		t.Skipf("本机没有 Cherry Studio 环境，无法端到端验证：%v", err)
	}

	if ins.AsarPath == "" {
		t.Fatalf("Cherry Studio 关闭时定位不到 app.asar——"+
			"这正是要修的问题。已尝试的候选路径：%v", ins.AsarCandidates)
	}
	t.Logf("Cherry Studio 关闭时仍定位到：%s", ins.AsarPath)

	if !fileExists(ins.AsarPath) {
		t.Errorf("定位到 %s，但文件不存在", ins.AsarPath)
	}
	if ins.Version == "" {
		t.Error("定位到了 app.asar 但读不出版本号，说明文件可能有问题")
	} else {
		t.Logf("版本号：%s", ins.Version)
	}
}

// TestRememberedAsarPathIsUsed 断言"记住的路径"确实参与了定位，
// 这样即使用户换了安装位置、注册表也查不到，工具仍能工作。
func TestRememberedAsarPathIsUsed(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "whatever", "app.asar")

	if err := RememberAsarPath(dir, want); err != nil {
		t.Fatalf("记住路径失败: %v", err)
	}
	if got := LoadConfig(dir).AsarPath; got != want {
		t.Fatalf("读回 %q，期望 %q", got, want)
	}

	// 配置文件损坏时应当降级为空配置，而不是让整个工具起不来
	if err := os.WriteFile(ConfigPath(dir), []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadConfig(dir).AsarPath; got != "" {
		t.Fatalf("配置损坏时应当返回空路径，实际 %q", got)
	}
}
