package store

import (
	"os"
	"path/filepath"
	"testing"
)

// copyRealDB 把真实数据库复制到临时目录，测试全程只碰副本。
func copyRealDB(t *testing.T) string {
	t.Helper()
	src := os.Getenv("CHERRY_DB")
	if src == "" {
		t.Skip("未设置 CHERRY_DB，跳过真实数据库测试")
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "cherrystudio.sqlite")
	for _, suffix := range []string{"", "-wal"} {
		raw, err := os.ReadFile(src + suffix)
		if err != nil {
			if suffix == "" {
				t.Fatalf("读取 %s 失败: %v", src, err)
			}
			continue // -wal 可能不存在
		}
		if err := os.WriteFile(dst+suffix, raw, 0o644); err != nil {
			t.Fatalf("写入副本失败: %v", err)
		}
	}
	return dst
}

// TestCRUDAgainstCopy 在真实数据库的副本上跑完整的增删改查，
// 确认写进去的行在结构上和 Cherry Studio 自己写出来的完全一致。
func TestCRUDAgainstCopy(t *testing.T) {
	dbPath := copyRealDB(t)
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("打开副本失败: %v", err)
	}
	defer db.Close()

	// --- 读 ---
	before, err := db.ListAgents(false)
	if err != nil {
		t.Fatalf("列出 Agent 失败: %v", err)
	}
	t.Logf("副本里有 %d 个未删除的 Agent", len(before))
	for _, a := range before {
		t.Logf("  %-12s %-16s model=%s order=%s", a.Type, a.Name, a.Model, a.OrderKey)
	}
	if len(before) == 0 {
		t.Fatal("副本里没有 Agent，测试前提不成立")
	}

	prevMax, err := db.MaxOrderKey()
	if err != nil {
		t.Fatalf("读最大排序键失败: %v", err)
	}

	// 挑一个真实存在的模型来建
	models, err := db.Models()
	if err != nil {
		t.Fatalf("读模型失败: %v", err)
	}
	var pick string
	for _, m := range models {
		if m.ID == "deepseek::deepseek-flash" {
			pick = m.ID
		}
	}
	if pick == "" {
		for _, m := range models {
			if m.Enabled {
				pick = m.ID
				break
			}
		}
	}
	if pick == "" {
		t.Fatal("副本里没有可用模型")
	}

	// --- 建 ---
	created, err := db.CreateAgent(CreateInput{
		Type:            "dsh",
		Name:            "集成测试助手",
		Instructions:    "这是一条集成测试用的系统提示词。",
		Model:           pick,
		Avatar:          "🧪",
		PermissionMode:  "acceptEdits",
		ReasoningEffort: "max",
	})
	if err != nil {
		t.Fatalf("新建 Agent 失败: %v", err)
	}
	t.Logf("新建成功: id=%s order=%s", created.ID, created.OrderKey)

	if created.Type != "dsh" {
		t.Errorf("type = %q, 期望 dsh", created.Type)
	}
	if created.Model != pick {
		t.Errorf("model = %q, 期望 %q", created.Model, pick)
	}
	if created.ConfigString("permission_mode") != "acceptEdits" {
		t.Errorf("permission_mode = %q", created.ConfigString("permission_mode"))
	}
	if created.ConfigString("avatar") != "🧪" {
		t.Errorf("avatar = %q", created.ConfigString("avatar"))
	}
	if err := validateOrderKey(created.OrderKey, base62Digits); err != nil {
		t.Errorf("生成的排序键非法: %v", err)
	}
	if prevMax != nil && created.OrderKey <= *prevMax {
		t.Errorf("排序键 %q 没有排在原最大值 %q 之后", created.OrderKey, *prevMax)
	}
	if len(created.ID) != 36 {
		t.Errorf("id %q 不是合法的 UUID 形状", created.ID)
	}

	// --- 复制 ---
	dup, err := db.DuplicateAgent(created.ID, "集成测试助手 副本")
	if err != nil {
		t.Fatalf("复制失败: %v", err)
	}
	if dup.Type != created.Type || dup.Model != created.Model {
		t.Errorf("副本的 type/model 与被复制者不一致: %+v", dup)
	}
	if dup.OrderKey <= created.OrderKey {
		t.Errorf("副本排序键 %q 未排在原件 %q 之后", dup.OrderKey, created.OrderKey)
	}

	// --- 改 ---
	newName := "集成测试助手（改名后）"
	updated, err := db.UpdateAgent(created.ID, UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("改名未生效: %q", updated.Name)
	}
	// 确认更新没有把别的配置项写丢
	if updated.ConfigString("avatar") != "🧪" {
		t.Errorf("更新后 avatar 丢了: %q", updated.ConfigString("avatar"))
	}
	if updated.Type != "dsh" {
		t.Errorf("更新后 type 被改了: %q", updated.Type)
	}

	// --- 软删除 + 恢复 ---
	if err := db.SetDeleted(dup.ID, true); err != nil {
		t.Fatalf("软删除失败: %v", err)
	}
	live, _ := db.ListAgents(false)
	for _, a := range live {
		if a.ID == dup.ID {
			t.Error("软删除后仍出现在未删除列表里")
		}
	}
	if err := db.SetDeleted(dup.ID, false); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// --- 永久删除 ---
	if err := db.DeleteAgent(dup.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := db.GetAgent(dup.ID); err == nil {
		t.Error("删除后仍能读到该 Agent")
	}

	// --- 外键校验：不存在的模型必须被拒绝 ---
	if _, err := db.CreateAgent(CreateInput{
		Type: "dsh", Name: "坏模型", Model: "does-not-exist::nope",
	}); err == nil {
		t.Error("用不存在的模型建档居然成功了，外键校验失效")
	}
}

// TestSchemaGuard 确认表结构不符时会被拒绝，而不是将错就错地写。
func TestSchemaGuard(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.sqlite")

	db, err := Open(p)
	// 全新空库没有 agent 表，必须报错
	if err == nil {
		db.Close()
		t.Fatal("对没有 agent 表的库居然打开成功了")
	}
	if _, ok := err.(*SchemaError); !ok {
		t.Fatalf("期望 SchemaError，得到 %T: %v", err, err)
	}
	t.Logf("如期拒绝: %v", err)
}
