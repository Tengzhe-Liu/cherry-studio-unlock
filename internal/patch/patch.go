// Package patch 实现「补丁」后端：改动 app.asar，让 Cherry Studio 自己的
// 新建 Agent 界面重新列出被隐藏的运行时。
//
// 设计要点：
//
//  1. **等长替换**。asar 的目录树里存着每个文件的偏移量，文件长度一变，后面所有
//     偏移都要重算，等于要重写整个 300MB 归档。这里改为把过滤条件替换成等长的
//     恒真表达式，文件长度不变，只需原地覆盖那几个字节。
//  2. **只备份被改的那几个字节**，而不是复制整个 asar。撤销时写回去即可。
//  3. **撤销前校验上下文**。Cherry Studio 升级后 asar 会变，偏移量随之失效；
//     我们比对改动位置周围的字节，不一致就拒绝撤销，而不是把文件改坏。
package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cherry-tools/cherry-agent-manager/internal/asar"
	"github.com/cherry-tools/cherry-agent-manager/internal/cherry"

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
)

// markerToken 是替换文本里用来表明「此处由本工具解锁」的标记。
// 只有在长度放得下时才会出现。
const markerToken = "unlocked"

// rePredicate 匹配过滤条件里的 `runtime !== "dsh"`。
var rePredicate = regexp.MustCompile(`runtime\s*!==\s*"([^"]+)"`)

// State 描述补丁当前的状况。
type State struct {
	// Supported 表示找到了可用的补丁点。
	Supported bool `json:"supported"`
	// Applied 表示当前已经打过补丁。
	Applied bool `json:"applied"`
	// AsarPath 是目标归档。
	AsarPath string `json:"asarPath"`
	// TargetFile 是过滤语句所在的文件（asar 内路径）。
	TargetFile string `json:"targetFile"`
	// Statement 是原始过滤语句。
	Statement string `json:"statement"`
	// Replaced 是被替换掉的片段。
	Replaced []string `json:"replaced"`
	// Reason 是不能打补丁时的原因。
	Reason string `json:"reason"`
	// PatchID 是已应用补丁的记录 ID，供撤销使用。
	PatchID string `json:"patchId,omitempty"`
}

// Record 是一次补丁的记录，用于撤销。
type Record struct {
	ID          string `json:"id"`
	AsarPath    string `json:"asarPath"`
	TargetFile  string `json:"targetFile"`
	AbsOffset   int64  `json:"absOffset"`
	Patched     string `json:"patched"`
	OriginalHex string `json:"originalHex"`
	ContextHash string `json:"contextHash"`
	// BeforeLen / AfterLen 记录取样区间的实际长度（文件首尾处可能不足 contextSpan）。
	BeforeLen int       `json:"beforeLen"`
	AfterLen  int       `json:"afterLen"`
	CreatedAt time.Time `json:"createdAt"`
}

// RecordsDir 返回补丁记录的存放目录。
func RecordsDir(dataDir string) string {
	return filepath.Join(cherry.BackupRoot(dataDir), "asar-patches")
}

// Inspect 判断当前 asar 是否支持打补丁、以及是否已经打过。
func Inspect(asarPath string, lang i18n.Lang) (State, error) {
	st := State{AsarPath: asarPath}
	if asarPath == "" {
		st.Reason = i18n.T(lang, "patch.reason.noAsar")
		return st, nil
	}

	a, err := asar.Open(asarPath)
	if err != nil {
		st.Reason = i18n.T(lang, "err.withDetail", i18n.T(lang, "patch.reason.openFailed"), err)
		return st, nil
	}
	defer a.Close()

	res, err := cherry.ScanRuntimes(a)
	if err != nil {
		st.Reason = i18n.T(lang, "err.withDetail", i18n.T(lang, "patch.reason.scanFailed"), err)
		return st, nil
	}
	st.TargetFile = res.Markers["RUNTIMES"]
	st.Statement = res.CreateFilterStmt

	if len(res.HiddenRuntimes()) == 0 && st.TargetFile == "" {
		st.Reason = i18n.T(lang, "patch.reason.noHidden")
		return st, nil
	}

	if st.Statement == "" || st.TargetFile == "" {
		st.Reason = i18n.T(lang, "patch.reason.noStatement")
		return st, nil
	}

	matches := rePredicate.FindAllString(st.Statement, -1)
	if len(matches) == 0 {
		st.Supported = true
		// 没有过滤条件，可能是我们改过的，也可能是官方自己就没藏东西。
		// 靠替换文本里的标记来区分。
		st.Applied = strings.Contains(st.Statement, markerToken)
		if !st.Applied {
			st.Reason = i18n.T(lang, "patch.reason.noHidden")
			st.Supported = false
		}
		return st, nil
	}

	st.Supported = true
	st.Replaced = matches
	return st, nil
}

// Apply 打补丁。返回打完之后的 State。
func Apply(asarPath, dataDir string, lang i18n.Lang) (State, error) {
	st, err := Inspect(asarPath, lang)
	if err != nil {
		return st, err
	}
	if !st.Supported {
		return st, i18n.E("patch.err.notSupported", st.Reason)
	}
	if st.Applied {
		return st, nil // 幂等
	}

	content, absBase, err := readTargetFile(asarPath, st.TargetFile)
	if err != nil {
		return st, err
	}

	stmtStart := strings.Index(string(content), st.Statement)
	if stmtStart < 0 {
		return st, i18n.E("patch.err.stmtMissing", st.TargetFile)
	}

	type edit struct {
		start, end int
		original   []byte
		patched    []byte
	}
	var edits []edit
	for _, loc := range rePredicate.FindAllIndex(content, -1) {
		if loc[0] < stmtStart || loc[1] > stmtStart+len(st.Statement) {
			continue
		}
		original := content[loc[0]:loc[1]]
		patched := sameLengthTrue(len(original))
		if len(patched) != len(original) {
			return st, i18n.E("patch.err.lenMismatch", len(patched), len(original))
		}
		edits = append(edits, edit{start: loc[0], end: loc[1], original: original, patched: []byte(patched)})
	}
	if len(edits) == 0 {
		return st, i18n.E("patch.err.noPredicate")
	}

	f, err := os.OpenFile(asarPath, os.O_RDWR, 0o644)
	if err != nil {
		return st, i18n.Wrap("patch.err.openRW", err)
	}
	defer f.Close()

	rec := Record{
		ID:         time.Now().Format("20060102-150405"),
		AsarPath:   asarPath,
		TargetFile: st.TargetFile,
		CreatedAt:  time.Now(),
	}
	var patchedParts []string

	for _, e := range edits {
		abs := absBase + int64(e.start)
		if _, err := f.WriteAt(e.patched, abs); err != nil {
			return st, i18n.Wrap("patch.err.writeFailed", err)
		}
		patchedParts = append(patchedParts, string(e.patched))
		if rec.AbsOffset == 0 {
			before, after := spansAround(content, e.start, e.end)
			rec.AbsOffset = abs
			rec.OriginalHex = hex.EncodeToString(e.original)
			rec.BeforeLen = len(before)
			rec.AfterLen = len(after)
			// 记录改动位置两侧的上下文哈希，撤销前比对
			rec.ContextHash = contextHash(before, after)
		}
	}
	rec.Patched = strings.Join(patchedParts, "|")
	if err := f.Sync(); err != nil {
		return st, i18n.Wrap("patch.err.syncFailed", err)
	}

	if err := saveRecord(dataDir, rec); err != nil {
		// 补丁已经打上了，但记录没存下来——撤销会受影响，必须让用户知道
		return st, i18n.Wrap("patch.err.recordFailed", err)
	}

	// 复查：重新扫描应当不再有隐藏运行时
	after, err := Inspect(asarPath, lang)
	if err == nil && !after.Applied {
		return st, i18n.E("patch.err.verifyFailed")
	}
	st.Applied = true
	st.PatchID = rec.ID
	return st, nil
}

// Revert 撤销补丁。
func Revert(asarPath, dataDir string) error {
	recs, err := ListRecords(dataDir)
	if err != nil {
		return err
	}
	var rec *Record
	for i := range recs {
		if recs[i].AsarPath == asarPath {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		return i18n.E("patch.err.noRecord")
	}

	original, err := hex.DecodeString(rec.OriginalHex)
	if err != nil {
		return i18n.Wrap("patch.err.badRecord", err)
	}

	f, err := os.OpenFile(asarPath, os.O_RDWR, 0o644)
	if err != nil {
		return i18n.Wrap("patch.err.openRW", err)
	}
	defer f.Close()

	// 撤销前先核对上下文：Cherry Studio 升级后 asar 变了，偏移量就不可信了
	before := make([]byte, rec.BeforeLen)
	after := make([]byte, rec.AfterLen)
	_, errBefore := f.ReadAt(before, rec.AbsOffset-int64(rec.BeforeLen))
	_, errAfter := f.ReadAt(after, rec.AbsOffset+int64(len(original)))
	if errBefore != nil || errAfter != nil || contextHash(before, after) != rec.ContextHash {
		return i18n.E("patch.err.changed")
	}

	if _, err := f.WriteAt(original, rec.AbsOffset); err != nil {
		return i18n.Wrap("patch.err.writeFailed", err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return deleteRecord(dataDir, rec.ID)
}

// sameLengthTrue 生成一个与 predicate 等长、且恒为真的表达式。
// 长度放得下时带上 unlocked 标记，方便人工确认。
func sameLengthTrue(n int) string {
	// "true /*" + token + "*/"
	commented := "true /*" + markerToken + "*/"
	if len(commented) == n {
		return commented
	}
	if n >= len(commented) {
		return commented + strings.Repeat(" ", n-len(commented))
	}
	// 长度不够，退化为 "true" + 空格
	if n >= 4 {
		return "true" + strings.Repeat(" ", n-4)
	}
	return strings.Repeat(" ", n) // 理论上不会发生
}

func readTargetFile(asarPath, target string) ([]byte, int64, error) {
	a, err := asar.Open(asarPath)
	if err != nil {
		return nil, 0, err
	}
	defer a.Close()

	entry, ok := a.Stat(target)
	if !ok {
		return nil, 0, i18n.E("err.asarEntryMissing", target)
	}
	if entry.Unpacked {
		return nil, 0, i18n.E("err.asarUnpackedPatch", target)
	}
	off, err := parseInt64(entry.Offset)
	if err != nil {
		return nil, 0, err
	}
	content, err := a.ReadFile(target)
	if err != nil {
		return nil, 0, err
	}
	return content, a.BaseOffset() + off, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, i18n.E("err.badOffsetValue", s)
		}
		n = n*10 + int64(r-'0')
	}
	return n, nil
}

// contextSpan 是从改动位置两侧取样的字节数。
const contextSpan = 32

// contextHash 对「改动位置之前 contextSpan 字节」与「之后 contextSpan 字节」取哈希，
// 刻意跳过被替换的字节本身——打补丁前后这两段是不变的，所以撤销时才能对上。
func contextHash(before, after []byte) string {
	sum := sha256.New()
	sum.Write(before)
	sum.Write(after)
	return hex.EncodeToString(sum.Sum(nil))
}

// spansAround 从内容里取出改动区间两侧的字节。
func spansAround(content []byte, start, end int) (before, after []byte) {
	lo := start - contextSpan
	if lo < 0 {
		lo = 0
	}
	hi := end + contextSpan
	if hi > len(content) {
		hi = len(content)
	}
	return content[lo:start], content[end:hi]
}

func saveRecord(dataDir string, rec Record) error {
	dir := RecordsDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, rec.ID+".json"), raw, 0o644)
}

// ListRecords 列出全部补丁记录，新的在前。
func ListRecords(dataDir string) ([]Record, error) {
	dir := RecordsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func deleteRecord(dataDir, id string) error {
	return os.Remove(filepath.Join(RecordsDir(dataDir), id+".json"))
}
