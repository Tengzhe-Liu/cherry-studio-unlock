// Package web 提供本地 HTTP 服务与内嵌界面。
//
// 之所以用「本地服务 + 浏览器」而不是原生 GUI：这样整个工具可以是一个
// 零依赖的单文件可执行文件，三个平台交叉编译即可发布，不需要 cgo，
// 也不需要用户额外装任何东西。
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/cherry"
	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/patch"
	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/service"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/i18n"
)

//go:embed assets
var assets embed.FS

// Server 承载界面与 API。
type Server struct {
	svc *service.Service
}

// New 构造 Server。
func New(svc *service.Service) *Server { return &Server{svc: svc} }

// Handler 组装全部路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	static, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	mux.HandleFunc("GET /api/status", s.wrap(s.status))
	mux.HandleFunc("POST /api/rescan", s.wrap(s.rescan))
	mux.HandleFunc("POST /api/config", s.wrap(s.setConfig))

	mux.HandleFunc("GET /api/agents", s.wrap(s.listAgents))
	mux.HandleFunc("POST /api/agents", s.wrap(s.createAgent))
	mux.HandleFunc("PUT /api/agents/{id}", s.wrap(s.updateAgent))
	mux.HandleFunc("DELETE /api/agents/{id}", s.wrap(s.deleteAgent))
	mux.HandleFunc("POST /api/agents/{id}/duplicate", s.wrap(s.duplicateAgent))

	mux.HandleFunc("GET /api/models", s.wrap(s.listModels))

	mux.HandleFunc("GET /api/backups", s.wrap(s.listBackups))
	mux.HandleFunc("POST /api/backups/{id}/restore", s.wrap(s.restoreBackup))

	mux.HandleFunc("GET /api/patch", s.wrap(s.patchState))
	mux.HandleFunc("POST /api/patch/apply", s.wrap(s.patchApply))
	mux.HandleFunc("POST /api/patch/revert", s.wrap(s.patchRevert))

	return mux
}

// apiError 是统一的错误响应。
type apiError struct {
	Error string `json:"error"`
	// Kind 让前端能针对特定情况给出更友好的提示。
	Kind string `json:"kind,omitempty"`
}

// handler 是返回数据或错误的处理函数。
type handler func(r *http.Request) (any, error)

func (s *Server) wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := h(r)
		if err != nil {
			var running service.ErrRunning
			kind := ""
			if errors.As(err, &running) {
				kind = "running"
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			writeJSON(w, apiError{Error: i18n.Text(s.svc.Lang(), err), Kind: kind})
			return
		}
		writeJSON(w, data)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return i18n.Wrap("err.badBody", err)
	}
	return nil
}

// ---------- 处理函数 ----------

func (s *Server) status(*http.Request) (any, error) { return s.svc.Status() }

func (s *Server) rescan(*http.Request) (any, error) {
	res, err := s.svc.Scan(true)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// setConfig 手动指定 app.asar 路径（会先试扫一遍确认有效再保存）。
func (s *Server) setConfig(r *http.Request) (any, error) {
	var body struct {
		AsarPath string `json:"asarPath"`
		Lang     string `json:"lang"`
	}
	if err := decodeBody(r, &body); err != nil {
		return nil, err
	}
	// 两个字段都是可选的：只切语言时不传路径，只改路径时不传语言
	if body.Lang != "" {
		if err := s.svc.SetLang(i18n.Parse(body.Lang)); err != nil {
			return nil, err
		}
	}
	if body.AsarPath != "" {
		if err := s.svc.SetAsarPath(body.AsarPath); err != nil {
			return nil, err
		}
	}
	return s.svc.Status()
}

func (s *Server) listAgents(*http.Request) (any, error) { return s.svc.ListAgents() }

func (s *Server) listModels(*http.Request) (any, error) { return s.svc.ListModels() }

func (s *Server) createAgent(r *http.Request) (any, error) {
	var in service.AgentInput
	if err := decodeBody(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateAgent(in)
}

func (s *Server) updateAgent(r *http.Request) (any, error) {
	var in service.AgentInput
	if err := decodeBody(r, &in); err != nil {
		return nil, err
	}
	return s.svc.UpdateAgent(r.PathValue("id"), in)
}

func (s *Server) deleteAgent(r *http.Request) (any, error) {
	return map[string]bool{"ok": true}, s.svc.DeleteAgent(r.PathValue("id"))
}

func (s *Server) duplicateAgent(r *http.Request) (any, error) {
	var body struct {
		Name string `json:"name"`
	}
	_ = decodeBody(r, &body)
	return s.svc.DuplicateAgent(r.PathValue("id"), body.Name)
}

func (s *Server) listBackups(*http.Request) (any, error) {
	return cherry.ListBackups(s.svc.Install().DataDir)
}

func (s *Server) restoreBackup(r *http.Request) (any, error) {
	return map[string]bool{"ok": true}, s.svc.RestoreBackup(r.PathValue("id"))
}

func (s *Server) patchState(*http.Request) (any, error) {
	p := s.svc.Install().AsarPath
	if p == "" {
		return patch.State{Reason: i18n.T(s.svc.Lang(), "err.asarNotFound")}, nil
	}
	return patch.Inspect(p, s.svc.Lang())
}

func (s *Server) patchApply(*http.Request) (any, error) {
	if cherry.IsRunning() {
		return nil, service.ErrRunning{}
	}
	p := s.svc.Install().AsarPath
	if p == "" {
		return nil, i18n.E("err.asarNotFound")
	}
	return patch.Apply(p, s.svc.Install().DataDir, s.svc.Lang())
}

func (s *Server) patchRevert(*http.Request) (any, error) {
	if cherry.IsRunning() {
		return nil, service.ErrRunning{}
	}
	p := s.svc.Install().AsarPath
	if p == "" {
		return nil, i18n.E("err.asarNotFound")
	}
	return map[string]bool{"ok": true}, patch.Revert(p, s.svc.Install().DataDir)
}

// Serve 监听本地回环地址并开始服务，返回实际使用的地址。
func (s *Server) Serve(port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + addr, nil
}

// StatusText 给终端输出一段人类可读的自检摘要，按当前语言渲染。
func StatusText(svc *service.Service) string {
	lang := svc.Lang()
	st, err := svc.Status()
	if err != nil {
		return i18n.T(lang, "cli.statusFailed", i18n.Text(lang, err))
	}
	var b strings.Builder
	ins := st.Install
	fmt.Fprintln(&b, i18n.T(lang, "cli.dataDir", ins.DataDir))
	if ins.Version != "" {
		fmt.Fprintln(&b, i18n.T(lang, "cli.version", ins.Version))
	}
	if ins.AsarPath != "" {
		fmt.Fprintln(&b, i18n.T(lang, "cli.asar", ins.AsarPath))
	}
	running := i18n.T(lang, "cli.runningNo")
	if st.Running {
		running = i18n.T(lang, "cli.runningYes")
	}
	fmt.Fprintln(&b, i18n.T(lang, "cli.running", running))
	fmt.Fprintln(&b, i18n.T(lang, "cli.agentCount", st.AgentCount))
	for _, c := range st.RuntimeChecks {
		mark := "✔"
		if c.Hidden {
			mark = "✖"
		}
		fmt.Fprintf(&b, "  %s %-16s %s", mark, c.Type, c.Verdict)
		fmt.Fprintln(&b)
	}
	if st.ScanError != "" {
		fmt.Fprintln(&b, i18n.T(lang, "cli.scanHint", st.ScanError))
	}
	return b.String()
}
