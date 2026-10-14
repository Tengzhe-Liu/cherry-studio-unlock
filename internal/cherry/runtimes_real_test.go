package cherry

import (
	"os"
	"testing"
	"time"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/asar"
)

// TestScanRealArchive 对真实 app.asar 跑扫描，验证能自动识别出被隐藏的运行时。
//
//	CHERRY_ASAR="/c/app/AI/Cherry Studio/resources/app.asar" go test ./internal/cherry/ -run ScanReal -v
func TestScanRealArchive(t *testing.T) {
	p := os.Getenv("CHERRY_ASAR")
	if p == "" {
		t.Skip("未设置 CHERRY_ASAR，跳过真实归档测试")
	}

	a, err := asar.Open(p)
	if err != nil {
		t.Fatalf("打开 asar 失败: %v", err)
	}
	defer a.Close()

	start := time.Now()
	res, err := ScanRuntimes(a)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	t.Logf("扫描 %d 个 JS 文件（失败 %d），耗时 %s", res.FilesScanned, res.FilesFailed, time.Since(start).Round(time.Millisecond))
	t.Logf("注册的驱动: %v", res.Drivers)
	t.Logf("过滤语句: %s", res.CreateFilterStmt)
	t.Logf("被排除的运行时: %v", res.HiddenFilterNames)

	for _, r := range res.Runtimes {
		t.Logf("  运行时 %-12s label=%-18s supported=%v offeredInCreate=%v hidden=%v",
			r.Type, r.Label, r.Supported, r.OfferedInCreate, r.Hidden)
	}
	for k, v := range res.Markers {
		t.Logf("  标记 %-32s → %s", k, v)
	}

	if len(res.Runtimes) == 0 {
		t.Fatal("没有识别出任何运行时，扫描逻辑可能没匹配上")
	}

	// 这套断言针对 v1.13.8 的实测结果；换成别的版本时可能需要在 README 里说明。
	if _, ok := res.FindRuntime("dsh"); !ok {
		t.Error("未识别出 dsh 运行时")
	}

	hidden := res.HiddenRuntimes()
	if len(hidden) == 0 {
		t.Error("没有发现任何被隐藏的运行时，但当前版本确实隐藏了 dsh")
	}
	for _, h := range hidden {
		t.Logf("发现被隐藏的运行时: %s (%s)", h.Type, h.Label)
	}
}
