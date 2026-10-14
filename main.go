// Cherry Agent Manager —— 解锁并管理 Cherry Studio 的 Agent 运行时。
//
// 用法：
//
//	cherry-agent-manager                 自动探测环境并打开界面
//	cherry-agent-manager --status        只在终端打印自检结果后退出
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
	"github.com/cherry-tools/cherry-agent-manager/internal/service"
	"github.com/cherry-tools/cherry-agent-manager/internal/web"
)

func main() {
	var (
		dataDir    = flag.String("data-dir", "", "手动指定 Cherry Studio 数据目录")
		asarPath   = flag.String("asar", "", "手动指定 app.asar 路径")
		port       = flag.Int("port", 0, "监听端口，0 表示自动选择")
		noOpen     = flag.Bool("no-open", false, "不自动打开浏览器")
		statusOnly = flag.Bool("status", false, "只打印自检结果后退出")
	)
	flag.Parse()

	if err := run(*dataDir, *asarPath, *port, *noOpen, *statusOnly); err != nil {
		fmt.Fprintln(os.Stderr, "\n出错了："+err.Error())
		os.Exit(1)
	}
}

func run(dataDir, asarPath string, port int, noOpen, statusOnly bool) error {
	install, err := cherry.Locate()
	if err != nil && dataDir == "" {
		return fmt.Errorf("%w\n\n可以用 --data-dir 手动指定，例如：\n"+
			"  cherry-agent-manager --data-dir \"%%APPDATA%%\\CherryStudio\\Data\"", err)
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
		return fmt.Errorf("没有找到数据库文件：%s", install.DBPath)
	}

	svc, err := service.New(install)
	if err != nil {
		return err
	}
	defer svc.Close()

	if statusOnly {
		fmt.Print("\n" + web.StatusText(svc))
		return nil
	}

	srv := web.New(svc)
	addr, err := srv.Serve(port)
	if err != nil {
		return fmt.Errorf("启动本地服务失败: %w", err)
	}

	fmt.Println()
	fmt.Println("  Cherry Agent Manager")
	fmt.Println("  ─────────────────────────────────────────────")
	fmt.Print(indent(web.StatusText(svc), "  "))
	fmt.Println()
	fmt.Println("  界面地址: " + addr)
	fmt.Println("  按 Ctrl+C 退出")
	fmt.Println()

	if !noOpen {
		if err := openBrowser(addr); err != nil {
			fmt.Println("  （没能自动打开浏览器，请手动访问上面的地址）")
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
