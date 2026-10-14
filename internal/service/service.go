// Package service 把「定位环境 / 备份 / 校验 / 写库 / 打补丁」编排成用例。
//
// 所有写操作都遵守同一条纪律：**先备份，再校验，最后才写**。
// 并且 Cherry Studio 在运行时一律拒绝写入——它在内存里缓存了 Agent 列表，
// 运行中写入会被覆盖或造成状态错乱。
package service

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/asar"
	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/cherry"
	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/patch"
	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/store"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/i18n"
)

// ErrRunning 表示 Cherry Studio 正在运行，写操作被拒绝。
type ErrRunning struct{}

func (ErrRunning) Error() string { return i18n.T(i18n.Zh, "err.running") }

// Msg 让展示层能按当前语言渲染。
func (ErrRunning) Msg() i18n.Msg { return i18n.Msg{Key: "err.running"} }

// Service 是一次会话内的工具实例。
type Service struct {
	mu      sync.Mutex
	install *cherry.Install
	db      *store.DB
	// lang 是当前界面语言。后端返回的文案（状态判定、错误）都按它渲染，
	// 否则英文界面里会突然冒出中文。
	lang i18n.Lang

	scanCache    *cherry.ScanResult
	scanCachedAt time.Time
}

const scanTTL = 5 * time.Minute

// New 打开数据库并绑定安装信息。lang 为初始界面语言。
func New(install *cherry.Install, lang i18n.Lang) (*Service, error) {
	db, err := store.Open(install.DBPath)
	if err != nil {
		return nil, err
	}
	return &Service{install: install, db: db, lang: lang}, nil
}

// Lang 返回当前界面语言。
func (s *Service) Lang() i18n.Lang {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lang
}

// SetLang 切换界面语言并持久化，下次启动沿用。
func (s *Service) SetLang(l i18n.Lang) error {
	s.mu.Lock()
	s.lang = l
	dir := s.install.DataDir
	s.mu.Unlock()

	cfg := cherry.LoadConfig(dir)
	if cfg.Lang == string(l) {
		return nil
	}
	cfg.Lang = string(l)
	return cherry.SaveConfig(dir, cfg)
}

// Close 关闭数据库。
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Install 返回安装信息。
func (s *Service) Install() *cherry.Install { return s.install }

// Running 报告 Cherry Studio 是否在运行。
func (s *Service) Running() bool { return cherry.IsRunning() }

func (s *Service) requireClosed() error {
	if cherry.IsRunning() {
		return ErrRunning{}
	}
	return nil
}

// Scan 扫描 app.asar，结果缓存 scanTTL 以便反复调用而不必每次读 300MB。
func (s *Service) Scan(force bool) (*cherry.ScanResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanLocked(force)
}

func (s *Service) scanLocked(force bool) (*cherry.ScanResult, error) {
	if !force && s.scanCache != nil && time.Since(s.scanCachedAt) < scanTTL {
		return s.scanCache, nil
	}
	res, err := scanAsarPath(s.install.AsarPath)
	if err != nil {
		return nil, err
	}
	s.scanCache = res
	s.scanCachedAt = time.Now()
	return res, nil
}

// scanAsarPath 扫描任意一个 app.asar。独立成函数，方便在改路径前先试扫一遍。
func scanAsarPath(path string) (*cherry.ScanResult, error) {
	if path == "" {
		return nil, i18n.E("err.asarNotFound")
	}
	a, err := asar.Open(path)
	if err != nil {
		return nil, i18n.Wrap("err.asarUnreadable", err, path)
	}
	defer a.Close()
	return cherry.ScanRuntimes(a)
}

// SetAsarPath 手动指定 app.asar。会先试扫一遍确认有效，再保存——
// 免得保存了一个错路径，下次启动依然不可用。
func (s *Service) SetAsarPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return i18n.E("err.asarEmpty")
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return i18n.E("err.asarNoSuchFile", path)
	}

	res, err := scanAsarPath(path)
	if err != nil {
		return i18n.Wrap("err.asarInvalid", err)
	}
	if len(res.Runtimes) == 0 {
		return i18n.E("err.asarNoRuntimes", path)
	}

	s.mu.Lock()
	s.install.AsarPath = path
	s.install.AppRoot, s.install.Version = cherry.DescribeAsar(path)
	s.scanCache = res
	s.scanCachedAt = time.Now()
	// 路径一旦确认可用，就把"未找到 app.asar"的旧警告清掉
	kept := s.install.Warnings[:0]
	for _, w := range s.install.Warnings {
		if !strings.Contains(w, "app.asar") {
			kept = append(kept, w)
		}
	}
	s.install.Warnings = kept
	s.mu.Unlock()

	return cherry.RememberAsarPath(s.install.DataDir, path)
}

// Status 是界面首页需要的全部信息。
type Status struct {
	Install       *cherry.Install    `json:"install"`
	Running       bool               `json:"running"`
	Scan          *cherry.ScanResult `json:"scan,omitempty"`
	ScanError     string             `json:"scanError,omitempty"`
	Backups       []cherry.Backup    `json:"backups"`
	AgentCount    int                `json:"agentCount"`
	PatchState    patch.State        `json:"patchState"`
	RuntimeChecks []RuntimeCheck     `json:"runtimeChecks"`
}

// RuntimeCheck 是某个运行时在本机的可用性结论。
type RuntimeCheck struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Hidden  bool   `json:"hidden"`
	Verdict string `json:"verdict"`
}

// Status 汇总当前环境状态。
func (s *Service) Status() (*Status, error) {
	st := &Status{
		Install: s.install,
		Running: cherry.IsRunning(),
	}

	if res, err := s.Scan(false); err != nil {
		st.ScanError = i18n.Text(s.lang, err)
	} else {
		st.Scan = res
		for _, r := range res.Runtimes {
			check := RuntimeCheck{Type: r.Type, Label: r.Label, Hidden: r.Hidden}
			switch {
			case !r.Supported:
				check.Verdict = i18n.T(s.lang, "runtime.verdict.unsupported")
			case r.Hidden:
				check.Verdict = i18n.T(s.lang, "runtime.verdict.hidden")
			default:
				check.Verdict = i18n.T(s.lang, "runtime.verdict.available")
			}
			st.RuntimeChecks = append(st.RuntimeChecks, check)
		}
	}

	agents, err := s.db.ListAgents(false)
	if err != nil {
		return nil, err
	}
	st.AgentCount = len(agents)

	backups, err := cherry.ListBackups(s.install.DataDir)
	if err != nil {
		return nil, err
	}
	st.Backups = backups

	if s.install.AsarPath != "" {
		ps, err := patch.Inspect(s.install.AsarPath, s.lang)
		if err == nil {
			st.PatchState = ps
		}
	}

	// 提示按当前语言渲染（存的是 key，所以切语言后不需要重新探测）
	st.Install.RenderWarnings(s.lang)
	return st, nil
}

// AgentInput 是新建/编辑 Agent 的入参（来自界面）。
type AgentInput struct {
	Type            string `json:"type"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Instructions    string `json:"instructions"`
	Model           string `json:"model"`
	Avatar          string `json:"avatar"`
	PermissionMode  string `json:"permissionMode"`
	ReasoningEffort string `json:"reasoningEffort"`
	// SkipRuntimeCheck 允许在扫不到 app.asar 时跳过"运行时是否真的内置"的校验。
	// 必须由用户在界面上明确勾选，默认关闭——这道校验是防止创建跑不起来的 Agent 的。
	SkipRuntimeCheck bool `json:"skipRuntimeCheck"`
}

// permissionModes 与 Cherry Studio 的权限模式一致。
// dsh 不支持 auto（它自己的 capabilities 就把 auto 排除掉了）。
var permissionModes = map[string][]string{
	"default":     {"default", "plan", "acceptEdits", "bypassPermissions"},
	"dsh":         {"default", "plan", "acceptEdits", "bypassPermissions"},
	"pi":          {"default", "acceptEdits", "bypassPermissions"},
	"claude-code": {"default", "plan", "acceptEdits", "auto", "bypassPermissions"},
}

func validatePermissionMode(runtimeType, mode string) error {
	if mode == "" {
		return nil
	}
	allowed, known := permissionModes[runtimeType]
	if !known {
		return nil // 未知运行时不做限制
	}
	for _, m := range allowed {
		if m == mode {
			return nil
		}
	}
	return i18n.E("err.permissionMode", runtimeType, mode, allowed)
}

// CreateAgent 新建 Agent。顺序：拒绝运行中 → 校验运行时 → 校验权限模式 → 备份 → 写库 → 建数据目录。
func (s *Service) CreateAgent(in AgentInput) (*store.Agent, error) {
	if err := s.requireClosed(); err != nil {
		return nil, err
	}
	if err := s.assertRuntimeUsable(in.Type, in.SkipRuntimeCheck); err != nil {
		return nil, err
	}
	if err := validatePermissionMode(in.Type, in.PermissionMode); err != nil {
		return nil, err
	}

	if _, err := cherry.CreateBackup(s.install.DataDir, "before-create-agent",
		s.dbPathWithSidecars()...); err != nil {
		return nil, i18n.Wrap("err.backupFailed", err)
	}

	created, err := s.db.CreateAgent(store.CreateInput{
		Type:            in.Type,
		Name:            in.Name,
		Description:     in.Description,
		Instructions:    in.Instructions,
		Model:           in.Model,
		Avatar:          in.Avatar,
		PermissionMode:  in.PermissionMode,
		ReasoningEffort: in.ReasoningEffort,
	})
	if err != nil {
		return nil, err
	}

	// 与 Cherry Studio 新建 Agent 时的目录结构保持一致。
	// 失败不算致命（App 会在需要时自己建），但要让用户知道。
	if err := s.ensureAgentDir(created.ID); err != nil {
		return created, i18n.Wrap("err.agentDir", err)
	}
	return created, nil
}

// assertRuntimeUsable 确认目标运行时确实内置在当前版本里。
// 这是本工具最重要的一道校验：宁可拒绝，也不要创建一个永远跑不起来的 Agent。
func (s *Service) assertRuntimeUsable(runtimeType string, allowUnverified bool) error {
	if runtimeType == "" {
		return i18n.E("err.typeRequired")
	}
	res, err := s.Scan(false)
	if err != nil {
		if allowUnverified {
			return nil // 用户已明确选择跳过校验
		}
		return i18n.E("err.verifyRuntime", err)
	}
	info, ok := res.FindRuntime(runtimeType)
	if !ok {
		var known []string
		for _, r := range res.Runtimes {
			known = append(known, r.Type)
		}
		return i18n.E("err.runtimeMissing", runtimeType, known)
	}
	if !info.Supported {
		return i18n.E("err.runtimeImpl", runtimeType)
	}
	return nil
}

func (s *Service) dbPathWithSidecars() []string {
	return []string{s.install.DBPath, s.install.DBPath + "-wal", s.install.DBPath + "-shm"}
}

func (s *Service) ensureAgentDir(id string) error {
	dir := filepath.Join(s.install.AgentsDir, id)
	if err := os.MkdirAll(filepath.Join(dir, "memory"), 0o755); err != nil {
		return err
	}
	for _, name := range []string{"SOUL.md", "USER.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListAgents 返回未删除的 Agent 列表。
func (s *Service) ListAgents() ([]store.Agent, error) { return s.db.ListAgents(false) }

// ListModels 返回可选模型。
func (s *Service) ListModels() ([]store.Model, error) { return s.db.Models() }

// UpdateAgent 编辑 Agent（同样需要先关闭 Cherry Studio）。
func (s *Service) UpdateAgent(id string, in AgentInput) (*store.Agent, error) {
	if err := s.requireClosed(); err != nil {
		return nil, err
	}
	cur, err := s.db.GetAgent(id)
	if err != nil {
		return nil, err
	}
	if err := validatePermissionMode(cur.Type, in.PermissionMode); err != nil {
		return nil, err
	}
	if _, err := cherry.CreateBackup(s.install.DataDir, "before-update-agent",
		s.dbPathWithSidecars()...); err != nil {
		return nil, i18n.Wrap("err.backupFailed", err)
	}

	up := store.UpdateInput{
		Name:            &in.Name,
		Description:     &in.Description,
		Instructions:    &in.Instructions,
		Model:           &in.Model,
		Avatar:          &in.Avatar,
		PermissionMode:  &in.PermissionMode,
		ReasoningEffort: &in.ReasoningEffort,
	}
	return s.db.UpdateAgent(id, up)
}

// DuplicateAgent 复制 Agent。
func (s *Service) DuplicateAgent(id, newName string) (*store.Agent, error) {
	if err := s.requireClosed(); err != nil {
		return nil, err
	}
	if _, err := cherry.CreateBackup(s.install.DataDir, "before-duplicate-agent",
		s.dbPathWithSidecars()...); err != nil {
		return nil, i18n.Wrap("err.backupFailed", err)
	}
	dup, err := s.db.DuplicateAgent(id, newName)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAgentDir(dup.ID); err != nil {
		return dup, i18n.Wrap("err.agentDir", err)
	}
	return dup, nil
}

// DeleteAgent 永久删除 Agent（会先备份，但仍不可轻易恢复）。
func (s *Service) DeleteAgent(id string) error {
	if err := s.requireClosed(); err != nil {
		return err
	}
	if _, err := cherry.CreateBackup(s.install.DataDir, "before-delete-agent",
		s.dbPathWithSidecars()...); err != nil {
		return i18n.Wrap("err.backupFailed", err)
	}
	return s.db.DeleteAgent(id)
}

// RestoreBackup 从备份恢复，恢复前会再存一份当前状态（所以撤销本身也可撤销）。
func (s *Service) RestoreBackup(backupID string) error {
	if err := s.requireClosed(); err != nil {
		return err
	}
	backups, err := cherry.ListBackups(s.install.DataDir)
	if err != nil {
		return err
	}
	for _, b := range backups {
		if b.ID == backupID {
			// 恢复要写文件，先关掉数据库连接避免文件占用
			s.mu.Lock()
			if s.db != nil {
				s.db.Close()
				s.db = nil
			}
			s.mu.Unlock()

			if err := cherry.Restore(s.install.DataDir, b); err != nil {
				return err
			}
			return s.reopen()
		}
	}
	return i18n.E("err.backupNotFound", backupID)
}

func (s *Service) reopen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return nil
	}
	db, err := store.Open(s.install.DBPath)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}
