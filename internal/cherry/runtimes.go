package cherry

import (
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/cherry-tools/cherry-agent-manager/internal/asar"
)

// 这些标记串来自 Cherry Studio 主进程与渲染进程的打包产物。
// 故意用「找标记」而不是「写死 dsh」：以后 Cherry Studio 藏了别的运行时，
// 工具也能自动发现，不需要改代码。
const (
	markerCapabilities = "AGENT_RUNTIME_CAPABILITIES = {"
	markerRuntimesVar  = "RUNTIMES"
	markerDshDriver    = "DshRuntimeDriver"
	markerI18nPrefix   = "library.config.agent.field.runtime.option."
	markerRegistry     = "runtimeDriverRegistry.register("
)

var (
	// "library.config.agent.field.runtime.option.dsh": "DeepSeek Harness",
	reRuntimeLabel = regexp.MustCompile(`"library\.config\.agent\.field\.runtime\.option\.([A-Za-z0-9_-]+)"\s*:\s*"([^"]*)"`)
	// .filter((runtime) => runtime !== "dsh")
	reFilterNe = regexp.MustCompile(`runtime\s*!==\s*"([^"]+)"`)
	reFilterEq = regexp.MustCompile(`runtime\s*===\s*"([^"]+)"`)
	// runtimeDriverRegistry.register(new DshRuntimeDriver())
	reRegister = regexp.MustCompile(`runtimeDriverRegistry\.register\(\s*new\s+([A-Za-z0-9_$]+)\s*\(`)
)

// RuntimeInfo 描述一个 Agent 运行时及其可用状态。
type RuntimeInfo struct {
	// Type 是写入 agent.type 的值，例如 "dsh"、"claude-code"、"pi"。
	Type string `json:"type"`
	// Label 是界面显示名，例如 "DeepSeek Harness"。
	Label string `json:"label"`
	// Supported 表示它出现在 AGENT_RUNTIME_CAPABILITIES 里，即运行时的确实装了。
	Supported bool `json:"supported"`
	// OfferedInCreate 表示新建 Agent 的界面里能直接选到它。
	OfferedInCreate bool `json:"offeredInCreate"`
	// Hidden 表示「已内置、但新建界面选不到」——本工具要解决的正是这个。
	Hidden bool `json:"hidden"`
}

// ScanResult 是一次 asar 扫描的全部结论。
type ScanResult struct {
	Runtimes []RuntimeInfo `json:"runtimes"`
	// Drivers 是注册进运行时驱动表的类名（如 DshRuntimeDriver）。
	Drivers []string `json:"drivers"`
	// CreateFilterStmt 是给新建界面做过滤的那条语句原文，补丁后端据此精确匹配。
	CreateFilterStmt string `json:"createFilterStmt"`
	// HiddenFilterNames 是被那条语句排除掉的运行时类型。
	HiddenFilterNames []string `json:"hiddenFilterNames"`
	FilesScanned      int      `json:"filesScanned"`
	FilesFailed       int      `json:"filesFailed"`
	// Markers 记录每个关键标记出现在哪个文件里，便于排障与定位补丁点。
	Markers map[string]string `json:"markers"`
}

// HiddenRuntimes 返回「已内置但界面选不到」的运行时。
func (s *ScanResult) HiddenRuntimes() []RuntimeInfo {
	var out []RuntimeInfo
	for _, r := range s.Runtimes {
		if r.Hidden {
			out = append(out, r)
		}
	}
	return out
}

// FindRuntime 按 type 查找。
func (s *ScanResult) FindRuntime(typ string) (RuntimeInfo, bool) {
	for _, r := range s.Runtimes {
		if r.Type == typ {
			return r, true
		}
	}
	return RuntimeInfo{}, false
}

// createFilter 描述「新建界面」的过滤规则。
type createFilter struct {
	Has   bool
	Mode  string // "exclude" 或 "include"
	Names map[string]bool
}

func (f *createFilter) offers(typ string) bool {
	if f == nil || !f.Has {
		return true
	}
	if f.Mode == "exclude" {
		return !f.Names[typ]
	}
	return f.Names[typ]
}

type scanHits struct {
	labels    map[string]string
	capsKeys  []string
	filter    *createFilter
	filterRaw string
	drivers   []string
	markers   map[string]string
	scanned   int
	failed    int
}

// ScanRuntimes 扫描 asar，列出所有已实现的运行时，并判断哪些被新建界面隐藏。
func ScanRuntimes(a *asar.Archive) (*ScanResult, error) {
	var files []string
	if err := a.Walk(func(p string, e *asar.Entry) error {
		if isJS(p) {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	hits := &scanHits{labels: map[string]string{}, markers: map[string]string{}}

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}

	var mu sync.Mutex
	jobs := make(chan string)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				raw, err := a.ReadFile(p)
				mu.Lock()
				hits.scanned++
				if err != nil {
					hits.failed++
				}
				mu.Unlock()
				if err != nil {
					continue
				}
				analyseFile(p, string(raw), hits, &mu)
			}
		}()
	}
	for _, p := range files {
		jobs <- p
	}
	close(jobs)
	wg.Wait()

	return hits.toResult(), nil
}

func analyseFile(path, content string, hits *scanHits, mu *sync.Mutex) {
	local := map[string]string{}
	var capsKeys []string
	var filter *createFilter
	var filterRaw string
	var drivers []string
	labels := map[string]string{}

	if strings.Contains(content, markerCapabilities) {
		local["AGENT_RUNTIME_CAPABILITIES"] = path
		capsKeys = parseCapabilityKeys(content)
	}

	if stmt, ok := findRuntimesStmt(content); ok {
		local["RUNTIMES"] = path
		filterRaw = stmt
		filter = parseCreateFilter(stmt)
	}

	if strings.Contains(content, markerDshDriver) {
		local["DshRuntimeDriver"] = path
	}
	if strings.Contains(content, markerRegistry) {
		local["runtimeDriverRegistry.register"] = path
		for _, m := range reRegister.FindAllStringSubmatch(content, -1) {
			drivers = append(drivers, m[1])
		}
	}
	if strings.Contains(content, markerI18nPrefix) {
		for _, m := range reRuntimeLabel.FindAllStringSubmatch(content, -1) {
			labels[m[1]] = m[2]
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for k, v := range local {
		if _, taken := hits.markers[k]; !taken {
			hits.markers[k] = v
		}
	}
	hits.capsKeys = append(hits.capsKeys, capsKeys...)
	hits.drivers = append(hits.drivers, drivers...)
	// 语句原文无论如何都要记下来：打完补丁后过滤条件会消失，
	// 但那条语句本身还在，撤销与"是否已打补丁"的判断都要靠它。
	if filterRaw != "" && hits.filterRaw == "" {
		hits.filterRaw = filterRaw
	}
	if filter != nil && hits.filter == nil {
		hits.filter = filter
	}
	for k, v := range labels {
		if _, exists := hits.labels[k]; !exists {
			hits.labels[k] = v
		}
	}
}

// findRuntimesStmt 定位给新建界面赋值的 `RUNTIMES = <...>;` 语句。
func findRuntimesStmt(content string) (string, bool) {
	for _, prefix := range []string{"var " + markerRuntimesVar + " = ", markerRuntimesVar + " = "} {
		idx := strings.Index(content, prefix)
		if idx < 0 {
			continue
		}
		stmt := upTo(content[idx:], ';', 4000)
		// 必须确实是运行时列表那条语句，避免撞上同名的无关变量
		if strings.Contains(stmt, "AGENT_RUNTIME_CAPABILITIES") || strings.Contains(stmt, ".filter(") {
			return strings.TrimSpace(stmt), true
		}
	}
	return "", false
}

// parseCapabilityKeys 从 `AGENT_RUNTIME_CAPABILITIES = { ... }` 中取出第一层的键名。
func parseCapabilityKeys(content string) []string {
	i := strings.Index(content, markerCapabilities)
	if i < 0 {
		return nil
	}
	body := content[i+len(markerCapabilities):]
	depth := 1
	lineStart := 0
	var keys []string

	for pos := 0; pos < len(body); pos++ {
		switch body[pos] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return keys
			}
		case '\n':
			lineStart = pos + 1
		case ':':
			if depth == 1 {
				if k, ok := normaliseCapabilityKey(body[lineStart:pos]); ok {
					keys = append(keys, k)
				}
			}
		}
	}
	return keys
}

func normaliseCapabilityKey(s string) (string, bool) {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
	if s == "" || strings.ContainsAny(s, " \t{}[](),.") {
		return "", false
	}
	return s, true
}

// parseCreateFilter 解析过滤语句。返回 nil 表示没有过滤（界面上全部可选）。
func parseCreateFilter(stmt string) *createFilter {
	if !strings.Contains(stmt, ".filter(") {
		return nil
	}
	if names := matchSet(reFilterNe, stmt); len(names) > 0 {
		return &createFilter{Has: true, Mode: "exclude", Names: names}
	}
	if names := matchSet(reFilterEq, stmt); len(names) > 0 {
		return &createFilter{Has: true, Mode: "include", Names: names}
	}
	return nil
}

func matchSet(re *regexp.Regexp, s string) map[string]bool {
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out[m[1]] = true
	}
	return out
}

func (h *scanHits) toResult() *ScanResult {
	res := &ScanResult{
		Markers:      h.markers,
		FilesScanned: h.scanned,
		FilesFailed:  h.failed,
	}

	if h.filterRaw != "" {
		res.CreateFilterStmt = h.filterRaw
		if h.filter != nil {
			res.HiddenFilterNames = sortedKeys(h.filter.Names)
		}
	}

	seen := map[string]bool{}
	for _, k := range h.capsKeys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		res.Runtimes = append(res.Runtimes, RuntimeInfo{Type: k})
	}
	sort.Slice(res.Runtimes, func(i, j int) bool { return res.Runtimes[i].Type < res.Runtimes[j].Type })

	for i := range res.Runtimes {
		r := &res.Runtimes[i]
		r.Supported = true
		r.Label = h.labels[r.Type]
		if r.Label == "" {
			r.Label = r.Type
		}
		r.OfferedInCreate = h.filter.offers(r.Type)
		r.Hidden = !r.OfferedInCreate
	}

	sort.Strings(h.drivers)
	res.Drivers = h.drivers
	return res
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isJS(p string) bool {
	for _, ext := range []string{".js", ".cjs", ".mjs"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// upTo 从 s 起截取到 stop 字符为止（最多 max 字节）。
func upTo(s string, stop byte, max int) string {
	if end := strings.IndexByte(s, stop); end >= 0 && end <= max {
		return s[:end]
	}
	if max < len(s) {
		return s[:max]
	}
	return s
}
