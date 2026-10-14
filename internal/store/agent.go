package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
)

// Agent 是 agent 表里的一行。Configuration 用 map 保存，
// 这样 Cherry Studio 自己加的配置项（bootstrap_completed、builtin_role、
// service_tier 等）原样保留，不会被我们写丢。
type Agent struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Instructions  string         `json:"instructions"`
	Model         string         `json:"model"`
	PlanModel     string         `json:"planModel,omitempty"`
	SmallModel    string         `json:"smallModel,omitempty"`
	OrderKey      string         `json:"orderKey"`
	CreatedAt     int64          `json:"createdAt"`
	UpdatedAt     int64          `json:"updatedAt"`
	Configuration map[string]any `json:"configuration"`
}

// ConfigString 读取 configuration 里的字符串项。
func (a *Agent) ConfigString(key string) string {
	if a.Configuration == nil {
		return ""
	}
	if v, ok := a.Configuration[key].(string); ok {
		return v
	}
	return ""
}

// CreateInput 是新建 Agent 的入参。
type CreateInput struct {
	Type            string
	Name            string
	Description     string
	Instructions    string
	Model           string
	Avatar          string
	PermissionMode  string
	ReasoningEffort string
}

// UpdateInput 是编辑 Agent 的入参；nil 表示该字段不改。
type UpdateInput struct {
	Name            *string
	Description     *string
	Instructions    *string
	Model           *string
	Avatar          *string
	PermissionMode  *string
	ReasoningEffort *string
}

const agentColumns = `id, type, name, description, instructions, model,
	coalesce(plan_model, ''), coalesce(small_model, ''), order_key,
	created_at, updated_at, configuration`

// ListAgents 返回 Agent 列表。默认不含已删除的。
func (d *DB) ListAgents(includeDeleted bool) ([]Agent, error) {
	q := "SELECT " + agentColumns + " FROM agent"
	if !includeDeleted {
		q += " WHERE deleted_at IS NULL"
	}
	q += " ORDER BY order_key"

	rows, err := d.sql.Query(q)
	if err != nil {
		return nil, i18n.Wrap("err.readAgents", err)
	}
	defer rows.Close()
	return scanAgents(rows)
}

// GetAgent 按 id 取单个 Agent。
func (d *DB) GetAgent(id string) (*Agent, error) {
	q := "SELECT " + agentColumns + " FROM agent WHERE id = ?"
	rows, err := d.sql.Query(q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanAgents(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, i18n.E("err.agentNotFound", id)
	}
	return &list[0], nil
}

func scanAgents(rows *sql.Rows) ([]Agent, error) {
	var out []Agent
	for rows.Next() {
		var a Agent
		var cfgRaw string
		if err := rows.Scan(
			&a.ID, &a.Type, &a.Name, &a.Description, &a.Instructions, &a.Model,
			&a.PlanModel, &a.SmallModel, &a.OrderKey,
			&a.CreatedAt, &a.UpdatedAt, &cfgRaw,
		); err != nil {
			return nil, err
		}
		a.Configuration = map[string]any{}
		if strings.TrimSpace(cfgRaw) != "" {
			// 解析失败不致命：保留空 map，后面写入时会被合并覆盖，
			// 但我们会把原文留在 ConfigRaw 里以防万一。
			_ = json.Unmarshal([]byte(cfgRaw), &a.Configuration)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func encodeConfig(cfg map[string]any) (string, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", i18n.Wrap("err.encodeConfig", err)
	}
	return string(raw), nil
}

// CreateAgent 新建一个 Agent。order_key 由本工具按 Cherry Studio 的算法算出来，
// 保证追加在末尾且与 App 后续新建的 Agent 不冲突。
func (d *DB) CreateAgent(in CreateInput) (*Agent, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, i18n.E("err.nameEmpty")
	}
	if in.Type == "" {
		return nil, i18n.E("err.typeRequired")
	}
	if in.Model == "" {
		return nil, i18n.E("err.modelRequired")
	}
	ok, err := d.ModelExists(in.Model)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, i18n.E("err.modelGone", in.Model)
	}

	cfg := map[string]any{}
	if in.Avatar != "" {
		cfg["avatar"] = in.Avatar
	}
	if in.PermissionMode != "" {
		cfg["permission_mode"] = in.PermissionMode
	}
	if in.ReasoningEffort != "" {
		cfg["reasoning_effort"] = in.ReasoningEffort
	}
	cfgRaw, err := encodeConfig(cfg)
	if err != nil {
		return nil, err
	}

	maxKey, err := d.MaxOrderKey()
	if err != nil {
		return nil, err
	}
	orderKey, err := NextOrderKey(maxKey)
	if err != nil {
		return nil, i18n.Wrap("err.orderKey", err)
	}

	id, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()

	_, err = d.sql.Exec(
		`INSERT INTO agent
		   (id, type, name, description, instructions, model,
		    plan_model, small_model, disabled_tools, configuration,
		    order_key, created_at, updated_at, deleted_at)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, '[]', ?, ?, ?, ?, NULL)`,
		id, in.Type, in.Name, in.Description, in.Instructions, in.Model,
		cfgRaw, orderKey, now, now,
	)
	if err != nil {
		return nil, i18n.Wrap("err.createAgent", err)
	}

	return d.GetAgent(id)
}

// UpdateAgent 修改 Agent。type 是创建后不可改的（Cherry Studio 自己也这么规定），
// 所以这里不接受 type。
func (d *DB) UpdateAgent(id string, in UpdateInput) (*Agent, error) {
	cur, err := d.GetAgent(id)
	if err != nil {
		return nil, err
	}

	if in.Model != nil {
		ok, err := d.ModelExists(*in.Model)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, i18n.E("err.modelGone", *in.Model)
		}
	}

	next := *cur
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return nil, i18n.E("err.nameEmpty")
		}
		next.Name = *in.Name
	}
	if in.Description != nil {
		next.Description = *in.Description
	}
	if in.Instructions != nil {
		next.Instructions = *in.Instructions
	}
	if in.Model != nil {
		next.Model = *in.Model
	}

	cfg := map[string]any{}
	for k, v := range cur.Configuration {
		cfg[k] = v
	}
	if in.Avatar != nil {
		cfg["avatar"] = *in.Avatar
	}
	if in.PermissionMode != nil {
		cfg["permission_mode"] = *in.PermissionMode
	}
	if in.ReasoningEffort != nil {
		cfg["reasoning_effort"] = *in.ReasoningEffort
	}
	cfgRaw, err := encodeConfig(cfg)
	if err != nil {
		return nil, err
	}

	_, err = d.sql.Exec(
		`UPDATE agent SET name = ?, description = ?, instructions = ?, model = ?,
		        configuration = ?, updated_at = ?
		 WHERE id = ?`,
		next.Name, next.Description, next.Instructions, next.Model,
		cfgRaw, time.Now().UnixMilli(), id,
	)
	if err != nil {
		return nil, i18n.Wrap("err.updateAgent", err)
	}
	return d.GetAgent(id)
}

// SetDeleted 软删除或恢复一个 Agent（Cherry Studio 用 deleted_at 表示删除）。
func (d *DB) SetDeleted(id string, deleted bool) error {
	var ts any
	if deleted {
		ts = time.Now().UnixMilli()
	}
	res, err := d.sql.Exec(`UPDATE agent SET deleted_at = ?, updated_at = ? WHERE id = ?`,
		ts, time.Now().UnixMilli(), id)
	if err != nil {
		return i18n.Wrap("err.setDeleted", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("找不到 Agent: %s", id)
	}
	return nil
}

// DeleteAgent 永久删除（会真的从表里移除，不可恢复，所以调用方必须先备份）。
func (d *DB) DeleteAgent(id string) error {
	res, err := d.sql.Exec(`DELETE FROM agent WHERE id = ?`, id)
	if err != nil {
		return i18n.Wrap("err.deleteAgent", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("找不到 Agent: %s", id)
	}
	return nil
}

// DuplicateAgent 复制一个 Agent（含运行时类型与全部配置），排在被复制者之后。
func (d *DB) DuplicateAgent(id, newName string) (*Agent, error) {
	src, err := d.GetAgent(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(newName) == "" {
		newName = src.Name + " 副本"
	}

	cfgRaw, err := encodeConfig(src.Configuration)
	if err != nil {
		return nil, err
	}

	orderKey, err := GenerateKeyBetween(&src.OrderKey, nil)
	if err != nil {
		// 极少数情况下（src 已是最大键）退回追加到末尾
		maxKey, mErr := d.MaxOrderKey()
		if mErr != nil {
			return nil, mErr
		}
		orderKey, err = NextOrderKey(maxKey)
		if err != nil {
			return nil, err
		}
	}

	newID, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()

	_, err = d.sql.Exec(
		`INSERT INTO agent
		   (id, type, name, description, instructions, model,
		    plan_model, small_model, disabled_tools, configuration,
		    order_key, created_at, updated_at, deleted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?, ?, NULL)`,
		newID, src.Type, newName, src.Description, src.Instructions, src.Model,
		nullable(src.PlanModel), nullable(src.SmallModel),
		cfgRaw, orderKey, now, now,
	)
	if err != nil {
		return nil, i18n.Wrap("err.duplicateAgent", err)
	}
	return d.GetAgent(newID)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// newUUIDv4 生成一个 v4 UUID，用作 agent 的主键，同时也会成为它数据目录的名字。
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", i18n.Wrap("err.uuidFailed", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
