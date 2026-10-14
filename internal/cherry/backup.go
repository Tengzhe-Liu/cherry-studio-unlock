package cherry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
)

// backupRootName 是备份目录名。放在用户数据目录下，卸载 Cherry Studio 时不会被误删，
// 用户也能一眼看到备份在哪。
const backupRootName = "backups-manual"

// Backup 是一次备份的记录。
type Backup struct {
	// ID 是目录名，形如 20261014-101530-before-create-agent
	ID string `json:"id"`
	// Dir 是备份目录的绝对路径。
	Dir string `json:"dir"`
	// Reason 说明这次备份是为了什么操作。
	Reason string `json:"reason"`
	// CreatedAt 是备份时间。
	CreatedAt time.Time `json:"createdAt"`
	// Entries 记录每个被备份的文件及其原始位置。
	Entries []BackupEntry `json:"entries"`
}

// BackupEntry 是备份里的一个文件。
type BackupEntry struct {
	// Original 是文件原本的绝对路径。
	Original string `json:"original"`
	// Stored 是它在备份目录里的文件名。
	Stored string `json:"stored"`
	// Size 是备份时的字节数。
	Size int64 `json:"size"`
}

// BackupRoot 返回某个数据目录下的备份根目录。
func BackupRoot(dataDir string) string {
	return filepath.Join(dataDir, backupRootName, "cherry-agent-manager")
}

// CreateBackup 把给定文件复制到备份目录，并写入 manifest。
// 不存在的文件会被跳过（例如 SQLite 的 -wal 文件可能本来就没有）。
func CreateBackup(dataDir, reason string, paths ...string) (*Backup, error) {
	stamp := time.Now().Format("20060102-150405")
	slug := slugify(reason)
	if slug == "" {
		slug = "backup"
	}
	dir := filepath.Join(BackupRoot(dataDir), stamp+"-"+slug)

	b := &Backup{
		ID:        filepath.Base(dir),
		Dir:       dir,
		Reason:    reason,
		CreatedAt: time.Now(),
	}

	for _, src := range paths {
		st, err := os.Stat(src)
		if err != nil || st.IsDir() {
			continue // 不存在就跳过
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, i18n.Wrap("err.backupDir", err)
		}
		stored := filepath.Base(src)
		if err := copyFile(src, filepath.Join(dir, stored)); err != nil {
			return nil, i18n.Wrap("err.backupCopy", err, src)
		}
		b.Entries = append(b.Entries, BackupEntry{Original: src, Stored: stored, Size: st.Size()})
	}

	if len(b.Entries) == 0 {
		return nil, i18n.E("err.backupNothing")
	}

	manifest, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o644); err != nil {
		return nil, i18n.Wrap("err.backupManifest", err)
	}
	return b, nil
}

// ListBackups 列出备份根目录下的全部备份，新的在前。
func ListBackups(dataDir string) ([]Backup, error) {
	root := BackupRoot(dataDir)
	dirents, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Backup
	for _, de := range dirents {
		if !de.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, de.Name(), "manifest.json"))
		if err != nil {
			continue // 不是我们的备份，跳过
		}
		var b Backup
		if err := json.Unmarshal(raw, &b); err != nil {
			continue
		}
		b.Dir = filepath.Join(root, de.Name())
		if b.ID == "" {
			b.ID = de.Name()
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Restore 按备份清单把文件放回原位。
// 覆盖之前会先把当前状态另存一份，所以「撤销」本身也是可以再撤销的。
func Restore(dataDir string, b Backup) error {
	// 先把当前状态存一份，避免用户撤错了无法回头
	if _, err := CreateBackup(dataDir, "before-restore", originals(b)...); err != nil {
		// 当前文件可能都不存在（例如刚删过），这不算致命错误
		fmt.Fprintf(os.Stderr, i18n.T(i18n.Detect(), "err.skipCurrentBackup")+"\n", err)
	}

	for _, e := range b.Entries {
		src := filepath.Join(b.Dir, e.Stored)
		if err := copyFile(src, e.Original); err != nil {
			return i18n.Wrap("err.restoreFailed", err, e.Original)
		}
	}
	return nil
}

func originals(b Backup) []string {
	out := make([]string, 0, len(b.Entries))
	for _, e := range b.Entries {
		out = append(out, e.Original)
	}
	return out
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// 先写临时文件再改名，避免中途失败留下半个文件把原文件毁了
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func slugify(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-")
}
