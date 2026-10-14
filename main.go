// Cherry Agent Manager —— 解锁并管理 Cherry Studio 的 Agent 运行时。
//
// 用法：
//
//	cherry-agent-manager                 自动探测环境并打开界面
//	cherry-agent-manager --status        只在终端打印自检结果后退出
//	cherry-agent-manager --lang en       以英文界面启动
//	cherry-agent-manager --port 8765     指定端口
//	cherry-agent-manager --no-open       不自动打开浏览器
//	cherry-agent-manager --data-dir DIR  手动指定 Cherry Studio 数据目录
//	cherry-agent-manager --asar FILE     手动指定 app.asar
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cherry-tools/cherry-agent-manager/internal/cherry"
	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
	"github.com/cherry-tools/cherry-agent-manager/internal/service"
	"github.com/cherry-tools/cherry-agent-manager/internal/web"
)

func main() {
	var (
		dataDir    = flag.String("data-dir", "", "手动指定数据目录 / data directory")
		asarPath   = flag.String("asar", "", "手动指定 app.asar / path to app.asar")
		port       = flag.Int("port", 0, "监听端口，0 为自动 / listen port, 0 = auto")
		noOpen     = flag.Bool("no-open", false, "不自动打开浏览器 / do not open a browser")
		statusOnly = flag.Bool("status", false, "只打印自检结果 / print self-check and exit")
		langFlag   = flag.String("lang", "", "界面语言 zh|en / UI language")
	)
	flag.Parse()

	lang := resolveLang(*langFlag, *dataDir)
	if err := run(*dataDir, *asarPath, *port, *noOpen, *statusOnly, lang); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.errPrefix", i18n.Text(lang, err)))
		os.Exit(1)
	}
}

// resolveLang 决定界面语言，优先级：命令行参数 > 配置文件 > 环境变量 > 中文。
func resolveLang(flagVal, dataDir string) i18n.Lang {
	if flagVal != "" {
		return i18n.Parse(flagVal)
	}
	dir := dataDir
	if dir == "" {
		if d, err := cherry.LocateDataDir(); err == nil {
			dir = d
		}
	}
	if dir != "" {
		if cfg := cherry.LoadConfig(dir); cfg.Lang != "" {
			return i18n.Parse(cfg.Lang)
		}
	}
	return i18n.Detect()
}

func run(dataDir, asarPath string, port int, noOpen, statusOnly bool, lang i18n.Lang) error {
	install, err := cherry.Locate()
	if err != nil && dataDir == "" {
		return i18n.E("cli.notInstalledHint")
	}
	if install == nil {
		install = &cherry.Install{}
	}

	// --data-dir 一旦显式指定，就覆盖自动探测的结果
	if dataDir != "" {
		install.DataDir = filepath.Clean(dataDir)
		install.DBPath = filepath.Join(install.DataDir, "cherrystudio.sqlite")
		install.AgentsDir = filepath.Join(install.DataDir, "Agents")
	}
	if asarPath != "" {
		install.AsarPath = filepath.Clean(asarPath)
		install.AppRoot, install.Version = cherry.DescribeAsar(install.AsarPath)
	}
	if !fileOK(install.DBPath) {
		return i18n.E("cli.noDataDir", install.DBPath)
	}

	// 记住这次定位到的 app.asar。Cherry Studio 必须关闭才能写数据，而关闭后就无法
	// 从进程反查安装目录了——装在自定义目录的用户下次就会找不到它。
	if install.AsarPath != "" {
		if err := cherry.RememberAsarPath(install.DataDir, install.AsarPath); err != nil {
			install.AddWarning("err.rememberFailed", err)
		}
	}

	svc, err := service.New(install, lang)
	if err != nil {
		return err
	}
	defer svc.Close()

	if statusOnly {
		fmt.Println()
		fmt.Print(web.StatusText(svc))
		return nil
	}

	srv := web.New(svc)
	addr, err := srv.Serve(port)
	if err != nil {
		return i18n.Wrap("cli.serveFailed", err)
	}

	fmt.Println()
	fmt.Println("  Cherry Agent Manager")
	fmt.Println("  ─────────────────────────────────────────────")
	fmt.Print(indent(web.StatusText(svc), "  "))
	fmt.Println()
	fmt.Println("  " + i18n.T(lang, "cli.uiAt", addr))
	fmt.Println("  " + i18n.T(lang, "cli.quit"))
	fmt.Println()

	if !noOpen {
		if err := openBrowser(addr); err != nil {
			fmt.Println("  " + i18n.T(lang, "cli.browserFailed"))
		}
	}

	select {}
}

func indent(s, prefix string) string {
	out := ""
	for _, line := range splitLines(s) {
		if line == "" {
			out += "\n"
			continue
		}
		out += prefix + line + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func fileOK(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
