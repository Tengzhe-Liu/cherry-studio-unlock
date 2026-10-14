// Package asar 只读解析 Electron 的 .asar 归档格式。
//
// asar 的结构：头部是一个 Chromium pickle，
//
//	[0:4]  uint32LE payloadSize（恒为 4）
//	[4:8]  uint32LE headerBufferSize
//	[8:8+headerBufferSize] 头部 pickle：
//	    [0:4]  uint32LE payloadSize
//	    [4:8]  uint32LE jsonLength
//	    [8:8+jsonLength] UTF-8 JSON 文件树
//
// 之后（按 4 字节对齐）是各文件的实际内容。
//
// 我们只需要"读"：定位 Cherry Studio 的运行时注册表，判断某个运行时是否真的
// 内置在当前版本里。重打包见 internal/patch。
package asar

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Tengzhe-Liu/cherry-studio-unlock/internal/i18n"
)

// headerPickleSize 是头部 pickle 之前的固定 8 字节（两个 uint32）。
const headerPickleSize = 8

// Entry 是 asar 文件树上的一个节点：要么是目录（Files 非空），要么是文件。
type Entry struct {
	Files    map[string]*Entry `json:"files,omitempty"`
	Size     int64             `json:"size,omitempty"`
	Offset   string            `json:"offset,omitempty"`
	Unpacked bool              `json:"unpacked,omitempty"`
	Link     string            `json:"link,omitempty"`
}

// IsDir 判断该节点是否为目录。
func (e *Entry) IsDir() bool { return e.Files != nil }

// Archive 是一个已打开的 asar 归档。
type Archive struct {
	path       string
	f          *os.File
	baseOffset int64
	headerSize int64
	root       *Entry
	unpacked   string
}

// Open 打开并解析 asar 的头部。文件内容按需读取，不会一次性载入。
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var sizeBuf [headerPickleSize]byte
	if _, err := f.ReadAt(sizeBuf[:], 0); err != nil {
		f.Close()
		return nil, i18n.Wrap("err.asarReadHeader", err)
	}
	if payloadSize := binary.LittleEndian.Uint32(sizeBuf[0:4]); payloadSize != 4 {
		f.Close()
		return nil, i18n.E("err.asarBadMagic", payloadSize)
	}
	headerSize := binary.LittleEndian.Uint32(sizeBuf[4:8])
	if headerSize == 0 || headerSize > 1<<30 {
		f.Close()
		return nil, i18n.E("err.asarBadHeaderSize", headerSize)
	}

	headerBuf := make([]byte, headerSize)
	if _, err := f.ReadAt(headerBuf, headerPickleSize); err != nil {
		f.Close()
		return nil, i18n.Wrap("err.asarReadTree", err)
	}
	if len(headerBuf) < headerPickleSize {
		f.Close()
		return nil, i18n.E("err.asarHeaderTooShort")
	}
	jsonLen := binary.LittleEndian.Uint32(headerBuf[4:8])
	if int64(8+jsonLen) > int64(len(headerBuf)) {
		f.Close()
		return nil, i18n.E("err.asarTreeOOB", jsonLen, len(headerBuf))
	}

	var root Entry
	if err := json.Unmarshal(headerBuf[8:8+jsonLen], &root); err != nil {
		f.Close()
		return nil, i18n.Wrap("err.asarParseTree", err)
	}
	if root.Files == nil {
		f.Close()
		return nil, i18n.E("err.asarNoFilesNode")
	}

	return &Archive{
		path:       path,
		f:          f,
		baseOffset: int64(headerPickleSize + headerSize),
		headerSize: int64(headerSize),
		root:       &root,
		unpacked:   path + ".unpacked",
	}, nil
}

// Close 关闭底层文件。
func (a *Archive) Close() error { return a.f.Close() }

// Path 返回 asar 文件路径。
func (a *Archive) Path() string { return a.path }

// BaseOffset 返回文件内容的起始偏移量。
func (a *Archive) BaseOffset() int64 { return a.baseOffset }

// HeaderSize 返回头部长度（用于计算重打包时的偏移修正）。
func (a *Archive) HeaderSize() int64 { return a.headerSize }

// Root 返回文件树根节点。
func (a *Archive) Root() *Entry { return a.root }

// Stat 按 asar 内路径查找节点，路径以 "/" 分隔且不带前导斜杠。
func (a *Archive) Stat(p string) (*Entry, bool) {
	p = strings.Trim(p, "/")
	if p == "" {
		return a.root, true
	}
	cur := a.root
	for _, seg := range strings.Split(p, "/") {
		if cur.Files == nil {
			return nil, false
		}
		next, ok := cur.Files[seg]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// ReadFile 读取一个文件的全部内容。unpacked 文件从 "<asar>.unpacked/" 下读取。
func (a *Archive) ReadFile(p string) ([]byte, error) {
	e, ok := a.Stat(p)
	if !ok {
		return nil, i18n.E("err.asarEntryMissing", p)
	}
	if e.IsDir() {
		return nil, i18n.E("err.asarIsDir", p)
	}
	if e.Unpacked {
		return os.ReadFile(filepath.Join(a.unpacked, filepath.FromSlash(strings.Trim(p, "/"))))
	}
	off, err := strconv.ParseInt(e.Offset, 10, 64)
	if err != nil {
		return nil, i18n.Wrap("err.asarBadOffset", err, p, e.Offset)
	}
	buf := make([]byte, e.Size)
	if _, err := a.f.ReadAt(buf, a.baseOffset+off); err != nil {
		return nil, i18n.Wrap("err.asarReadFile", err, p)
	}
	return buf, nil
}

// Walk 以前序遍历访问文件树中的每个文件（不含目录）。
func (a *Archive) Walk(fn func(path string, e *Entry) error) error {
	return walk(a.root, "", fn)
}

func walk(e *Entry, prefix string, fn func(string, *Entry) error) error {
	if !e.IsDir() {
		return fn(prefix, e)
	}
	names := make([]string, 0, len(e.Files))
	for name := range e.Files {
		names = append(names, name)
	}
	sort.Strings(names) // 稳定顺序，便于复现
	for _, name := range names {
		p := name
		if prefix != "" {
			p = prefix + "/" + name
		}
		if err := walk(e.Files[name], p, fn); err != nil {
			return err
		}
	}
	return nil
}

// ReadAll 读取整个归档的原始字节（重打包时用）。调用方负责内存。
func (a *Archive) ReadAll() ([]byte, error) {
	st, err := a.f.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, st.Size())
	if _, err := a.f.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}
