// Package store 直接读写 Cherry Studio 的 cherrystudio.sqlite。
//
// 这是「直接写数据库」那条后端。它只碰 agent / user_model 两张表，
// 并且在打开时先校验表结构，结构对不上就拒绝写入——宁可让用户看到明确的报错，
// 也不要往一个我们不认识的库里塞半懂不懂的数据。
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // 纯 Go 实现，不需要 cgo

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
)

// DB 是一个已打开的 Cherry Studio 数据库。
type DB struct {
	sql  *sql.DB
	path string
}

// requiredColumns 是我们在写入前必须确认存在的列。
// 少任何一列都说明这个版本的表结构变了，此时应该停下来。
var requiredColumns = []string{
	"id", "type", "name", "description", "instructions", "model",
	"plan_model", "small_model", "disabled_tools", "configuration",
	"order_key", "created_at", "updated_at", "deleted_at",
}

// SchemaError 表示数据库结构与预期不符。
type SchemaError struct {
	Path    string
	Missing []string
	// NoTable 表示连 agent 表本身都没有（全新空库）。
	NoTable bool
}

func (e *SchemaError) key() string {
	if e.NoTable {
		return "err.schemaNoTable"
	}
	return "err.schemaMismatch"
}

func (e *SchemaError) args() []any {
	if e.NoTable {
		return []any{e.Path}
	}
	return []any{e.Path, strings.Join(e.Missing, ", ")}
}

func (e *SchemaError) Error() string {
	return i18n.T(i18n.Zh, e.key(), e.args()...)
}

// Msg 让展示层能按当前语言重新渲染。
func (e *SchemaError) Msg() i18n.Msg {
	return i18n.Msg{Key: e.key(), Args: e.args()}
}

// Open 打开数据库并校验表结构。
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, i18n.Wrap("err.openDb", err)
	}
	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, i18n.Wrap("err.pingDb", err)
	}
	db := &DB{sql: sqldb, path: path}
	if err := db.verifySchema(); err != nil {
		sqldb.Close()
		return nil, err
	}
	return db, nil
}

// Close 关闭数据库。
func (d *DB) Close() error { return d.sql.Close() }

// Path 返回数据库路径。
func (d *DB) Path() string { return d.path }

func (d *DB) verifySchema() error {
	rows, err := d.sql.Query("PRAGMA table_info(agent)")
	if err != nil {
		return i18n.Wrap("err.readSchema", err)
	}
	defer rows.Close()

	have := map[string]bool{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notNull int
			dflt    any
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		have[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(have) == 0 {
		return &SchemaError{Path: d.path, NoTable: true}
	}

	var missing []string
	for _, c := range requiredColumns {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return &SchemaError{Path: d.path, Missing: missing}
	}
	return nil
}

// TableCount 统计某张表的行数（用于自检展示）。
func (d *DB) TableCount(table string) (int, error) {
	// 表名来自内部常量，不存在注入风险
	var n int
	err := d.sql.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
	return n, err
}

// Model 是 user_model 里可供 Agent 选择的一个模型。
type Model struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerId"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
}

// Models 返回可用的模型列表，按 provider 分组排序。
// Agent 的 model 列是外键指向这里，所以必须从这里挑。
func (d *DB) Models() ([]Model, error) {
	rows, err := d.sql.Query(
		`SELECT id, provider_id, coalesce(name, ''), is_enabled FROM user_model ORDER BY provider_id, id`)
	if err != nil {
		return nil, i18n.Wrap("err.readModels", err)
	}
	defer rows.Close()

	var out []Model
	for rows.Next() {
		var m Model
		var enabled int
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &enabled); err != nil {
			return nil, err
		}
		m.Enabled = enabled != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// ModelExists 校验模型 ID 是否存在（写入前的外键预检）。
func (d *DB) ModelExists(id string) (bool, error) {
	var n int
	err := d.sql.QueryRow("SELECT count(*) FROM user_model WHERE id = ?", id).Scan(&n)
	return n > 0, err
}

// MaxOrderKey 返回当前所有未删除 Agent 里最大的 order_key。
func (d *DB) MaxOrderKey() (*string, error) {
	var k sql.NullString
	err := d.sql.QueryRow(
		`SELECT max(order_key) FROM agent WHERE deleted_at IS NULL`).Scan(&k)
	if err != nil {
		return nil, err
	}
	if !k.Valid || k.String == "" {
		return nil, nil
	}
	return &k.String, nil
}
