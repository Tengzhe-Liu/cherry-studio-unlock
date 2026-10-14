package store

import (
	"encoding/json"
	"os"
	"testing"
)

type orderKeyVector struct {
	A    *string `json:"a"`
	B    *string `json:"b"`
	Want string  `json:"want"`
	Note string  `json:"note"`
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func loadVectors(t *testing.T) []orderKeyVector {
	t.Helper()
	raw, err := os.ReadFile("testdata/orderkey_vectors.json")
	if err != nil {
		t.Fatalf("读取测试向量失败: %v", err)
	}
	var vectors []orderKeyVector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("解析测试向量失败: %v", err)
	}
	if len(vectors) == 0 {
		t.Fatal("测试向量为空")
	}
	return vectors
}

// TestGenerateKeyBetweenMatchesReference 逐条比对 Cherry Studio 内置的参考实现。
// 向量由 tools/orderkey-vectors/gen.mjs 生成，真值来自 App 实际打包的那份
// fractional-indexing@3.2.0 —— 也就是说，这个测试断言的是"和 Cherry Studio 生成的键一模一样"。
func TestGenerateKeyBetweenMatchesReference(t *testing.T) {
	for i, v := range loadVectors(t) {
		got, err := GenerateKeyBetween(v.A, v.B)
		if err != nil {
			t.Errorf("case %d (%s): GenerateKeyBetween(%s, %s) 报错: %v",
				i, v.Note, deref(v.A), deref(v.B), err)
			continue
		}
		if got != v.Want {
			t.Errorf("case %d (%s): GenerateKeyBetween(%s, %s) = %q, 期望 %q",
				i, v.Note, deref(v.A), deref(v.B), got, v.Want)
		}
	}
}

// TestAppendedKeysAreStrictlyIncreasing 模拟连续新建 Agent：
// 每一步追加出来的键都必须合法，且严格大于前一个。
func TestAppendedKeysAreStrictlyIncreasing(t *testing.T) {
	var max *string
	prev := ""
	for i := 0; i < 500; i++ {
		key, err := NextOrderKey(max)
		if err != nil {
			t.Fatalf("第 %d 次追加失败: %v", i+1, err)
		}
		if err := validateOrderKey(key, base62Digits); err != nil {
			t.Fatalf("第 %d 次追加产生了非法键 %q: %v", i+1, key, err)
		}
		if i > 0 && key <= prev {
			t.Fatalf("第 %d 次追加剧了 %q，未大于前一个 %q", i+1, key, prev)
		}
		prev = key
		max = &key
	}
}

// TestNextOrderKeyTreatsEmptyAsNil 空串与 nil 等价（表的 order_key 可能读出空串）。
func TestNextOrderKeyTreatsEmptyAsNil(t *testing.T) {
	empty := ""
	got, err := NextOrderKey(&empty)
	if err != nil {
		t.Fatalf("NextOrderKey(空串) 报错: %v", err)
	}
	if got != "a0" {
		t.Fatalf("NextOrderKey(空串) = %q, 期望 \"a0\"", got)
	}
}

func TestGenerateKeyBetweenRejectsOutOfOrder(t *testing.T) {
	a, b := "a5", "a1"
	if _, err := GenerateKeyBetween(&a, &b); err == nil {
		t.Fatal("期望 a >= b 时报错，但没有")
	}
}

func TestGenerateKeyBetweenRejectsInvalidKey(t *testing.T) {
	bad := "0abc" // 首字符不是字母，不是合法整数部分
	if _, err := GenerateKeyBetween(&bad, nil); err == nil {
		t.Fatal("期望非法键报错，但没有")
	}
}
