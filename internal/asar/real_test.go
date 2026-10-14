package asar

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestOpenRealArchive 对着真实的 app.asar 做集成测试。
// 未设置 CHERRY_ASAR 时跳过，所以在 CI 上是空操作；本地验证时：
//
//	CHERRY_ASAR="/c/app/AI/Cherry Studio/resources/app.asar" go test ./internal/asar/ -run Real -v
func TestOpenRealArchive(t *testing.T) {
	p := os.Getenv("CHERRY_ASAR")
	if p == "" {
		t.Skip("未设置 CHERRY_ASAR，跳过真实归档测试")
	}

	a, err := Open(p)
	if err != nil {
		t.Fatalf("打开 %s 失败: %v", p, err)
	}
	defer a.Close()

	if a.HeaderSize() <= 0 {
		t.Fatalf("头部长度异常: %d", a.HeaderSize())
	}

	// 统计文件数，确认目录树确实被解析出来了
	files := 0
	var jsFiles int
	err = a.Walk(func(path string, e *Entry) error {
		files++
		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".cjs") || strings.HasSuffix(path, ".mjs") {
			jsFiles++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历失败: %v", err)
	}
	t.Logf("头部长度=%d 内容起始=%d 文件数=%d 其中 JS=%d", a.HeaderSize(), a.BaseOffset(), files, jsFiles)
	if files < 100 {
		t.Fatalf("文件数太少（%d），目录树可能没解析对", files)
	}

	// 读一个已知存在的 JSON，确认按偏移取内容是对的
	raw, err := a.ReadFile("package.json")
	if err != nil {
		t.Fatalf("读取 package.json 失败: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("package.json 不是合法 JSON（偏移算错了？）: %v", err)
	}
	if _, ok := meta["name"]; !ok {
		t.Fatalf("package.json 里没有 name 字段: %v", meta)
	}
	t.Logf("package.json → name=%v version=%v", meta["name"], meta["version"])
}
