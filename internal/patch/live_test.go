package patch

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"

	"github.com/cherry-tools/cherry-agent-manager/internal/asar"
	"github.com/cherry-tools/cherry-agent-manager/internal/cherry"
)

func fileHash(t *testing.T, p string) string {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", p, err)
	}
	sum := sha256.Sum256(raw)
	return string(sum[:])
}

// TestPatchRoundTrip 在真实的 app.asar 副本上跑完整的「打补丁 → 验证生效 → 撤销 → 验证还原」。
//
//	CHERRY_ASAR="/c/app/AI/Cherry Studio/resources/app.asar" go test ./internal/patch/ -run RoundTrip -v
func TestPatchRoundTrip(t *testing.T) {
	src := os.Getenv("CHERRY_ASAR")
	if src == "" {
		t.Skip("未设置 CHERRY_ASAR，跳过补丁测试")
	}

	dir := t.TempDir()
	asarPath := filepath.Join(dir, "app.asar")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读取源 asar 失败: %v", err)
	}
	if err := os.WriteFile(asarPath, raw, 0o644); err != nil {
		t.Fatalf("写副本失败: %v", err)
	}
	before := fileHash(t, asarPath)
	t.Logf("副本大小 %.1f MB", float64(len(raw))/1024/1024)

	dataDir := filepath.Join(dir, "Data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// --- 1. 打补丁前 ---
	st, err := Inspect(asarPath, i18n.Zh)
	if err != nil {
		t.Fatalf("Inspect 失败: %v", err)
	}
	t.Logf("打补丁前: supported=%v applied=%v target=%s", st.Supported, st.Applied, st.TargetFile)
	t.Logf("原始语句: %s", st.Statement)
	t.Logf("将被替换: %v", st.Replaced)
	if !st.Supported || st.Applied {
		t.Fatalf("预期「可打补丁且未打过」，实际 supported=%v applied=%v reason=%s", st.Supported, st.Applied, st.Reason)
	}

	hiddenBefore := hiddenRuntimes(t, asarPath)
	if len(hiddenBefore) == 0 {
		t.Fatal("打补丁前应当存在被隐藏的运行时")
	}
	t.Logf("打补丁前被隐藏: %v", hiddenBefore)

	// --- 2. 打补丁 ---
	after, err := Apply(asarPath, dataDir, i18n.Zh)
	if err != nil {
		t.Fatalf("打补丁失败: %v", err)
	}
	t.Logf("打补丁后: applied=%v patchID=%s", after.Applied, after.PatchID)

	if got := fileHash(t, asarPath); got == before {
		t.Error("文件内容没有变化，补丁可能没写进去")
	}
	// 大小必须不变——这正是等长替换的意义
	fi, _ := os.Stat(asarPath)
	if fi.Size() != int64(len(raw)) {
		t.Errorf("文件大小变了: %d -> %d（等长替换失败）", len(raw), fi.Size())
	}

	hiddenAfter := hiddenRuntimes(t, asarPath)
	t.Logf("打补丁后被隐藏: %v", hiddenAfter)
	if len(hiddenAfter) != 0 {
		t.Errorf("补丁后仍然有被隐藏的运行时: %v", hiddenAfter)
	}

	// --- 3. 幂等 ---
	if _, err := Apply(asarPath, dataDir, i18n.Zh); err != nil {
		t.Errorf("重复打补丁应当幂等，却报错: %v", err)
	}

	// --- 4. 撤销 ---
	if err := Revert(asarPath, dataDir); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	restored := fileHash(t, asarPath)
	if restored != before {
		t.Errorf("撤销后文件未还原到原始内容")
	} else {
		t.Log("撤销成功，app.asar 与原始字节完全一致")
	}

	hiddenRestored := hiddenRuntimes(t, asarPath)
	t.Logf("撤销后被隐藏: %v", hiddenRestored)
	if len(hiddenRestored) != len(hiddenBefore) {
		t.Errorf("撤销后隐藏列表未复原: %v vs %v", hiddenRestored, hiddenBefore)
	}
}

func hiddenRuntimes(t *testing.T, asarPath string) []string {
	t.Helper()
	a, err := asar.Open(asarPath)
	if err != nil {
		t.Fatalf("打开 asar 失败: %v", err)
	}
	defer a.Close()
	res, err := cherry.ScanRuntimes(a)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	var out []string
	for _, h := range res.HiddenRuntimes() {
		out = append(out, h.Type)
	}
	return out
}
