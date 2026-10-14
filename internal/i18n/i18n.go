// Package i18n 提供中英双语文案。
//
// 设计要点：错误在产生时**不决定语言**，只携带一个 key；
// 由展示层（网页 / 命令行）在输出前按当前语言渲染。
// 否则深层包里的报错会写死成某一种语言，切换语言时就会中英夹杂。
package i18n

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Lang 是界面语言。
type Lang string

const (
	Zh Lang = "zh"
	En Lang = "en"
)

// Parse 把 "en"、"en-US"、"English" 之类都归一到支持的语言；认不出的一律当中文。
func Parse(s string) Lang {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "en") {
		return En
	}
	return Zh
}

// Detect 从环境变量猜一个默认语言（LANG / LC_ALL / LANGUAGE）。
func Detect() Lang {
	for _, k := range []string{"CHERRY_AGENT_MANAGER_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return Parse(v)
		}
	}
	return Zh
}

// messages 是全部文案。key 用点号分层，缺失时回退到中文，再缺失则直接显示 key
// ——这样漏翻会当场暴露，而不是静默显示空白。
var messages = map[string]map[Lang]string{
	// ---------- 命令行 / 自检 ----------
	"cli.title":         {Zh: "Cherry Agent Manager", En: "Cherry Agent Manager"},
	"cli.dataDir":       {Zh: "Cherry Studio 数据目录 : %s", En: "Cherry Studio data dir  : %s"},
	"cli.version":       {Zh: "程序版本               : %s", En: "App version            : %s"},
	"cli.asar":          {Zh: "app.asar               : %s", En: "app.asar               : %s"},
	"cli.running":       {Zh: "正在运行               : %v", En: "Running                : %v"},
	"cli.runningYes":    {Zh: "是（写操作会被拒绝）", En: "yes (writes are refused)"},
	"cli.runningNo":     {Zh: "否", En: "no"},
	"cli.agentCount":    {Zh: "已有 Agent             : %d", En: "Existing agents        : %d"},
	"cli.scanHint":      {Zh: "扫描提示               : %s", En: "Scan note              : %s"},
	"cli.uiAt":          {Zh: "界面地址: %s", En: "Web UI: %s"},
	"cli.quit":          {Zh: "按 Ctrl+C 退出", En: "Press Ctrl+C to quit"},
	"cli.browserFailed": {Zh: "（没能自动打开浏览器，请手动访问上面的地址）", En: "(could not open a browser — please visit the URL above)"},
	"cli.statusFailed":  {Zh: "读取状态失败: %v", En: "Failed to read status: %v"},
	"cli.errPrefix":     {Zh: "出错了：%s", En: "Error: %s"},
	"cli.serveFailed":   {Zh: "启动本地服务失败: %v", En: "Failed to start the local server: %v"},
	"cli.emptyDataDir":  {Zh: "在 %s 下没有找到 cherrystudio.sqlite", En: "No cherrystudio.sqlite found under %s"},
	"cli.noDataDir":     {Zh: "没有找到数据库文件：%s", En: "Database file not found: %s"},
	"cli.notInstalled":  {Zh: "没有找到 Cherry Studio 的数据目录", En: "Could not locate the Cherry Studio data directory"},
	"cli.notInstalledHint": {
		Zh: "没有找到 Cherry Studio 的数据目录。可以用 --data-dir 手动指定，例如 --data-dir \"%APPDATA%/CherryStudio/Data\"",
		En: "Cherry Studio data directory not found. Point at it with --data-dir, e.g. --data-dir \"%APPDATA%/CherryStudio/Data\"",
	},

	// ---------- 运行时状态 ----------
	"runtime.verdict.available":   {Zh: "已内置且可直接新建", En: "Built in and selectable"},
	"runtime.verdict.hidden":      {Zh: "已内置，但新建界面里选不到——本工具可以解锁", En: "Built in but hidden from the create dialog — this tool can unlock it"},
	"runtime.verdict.unsupported": {Zh: "该版本未内置此运行时", En: "Not present in this version"},

	// ---------- 通用错误 ----------
	"err.withDetail": {Zh: "%s：%v", En: "%s: %v"},
	"err.running":    {Zh: "Cherry Studio 正在运行。请先完全退出它（包括右下角托盘图标 → 退出），再执行写操作", En: "Cherry Studio is running. Quit it completely first (including the tray icon → Quit), then retry the write"},
	"err.badBody":    {Zh: "请求体解析失败", En: "Could not parse the request body"},
	"err.unknown":    {Zh: "未知错误", En: "Unknown error"},

	// ---------- 环境探测 ----------
	"err.dataDirMissing": {Zh: "没有找到 Cherry Studio 的数据目录", En: "Cherry Studio data directory not found"},
	"err.dbMissing":      {Zh: "找到了数据目录但没有数据库文件：%s", En: "Found the data directory but no database file: %s"},

	// ---------- app.asar ----------
	"err.asarNotFound":   {Zh: "没有找到 app.asar。请先在本工具里指定它的路径（通常在 <Cherry Studio 安装目录>/resources/app.asar）", En: "app.asar not found. Set its path in this tool first (usually <Cherry Studio install dir>/resources/app.asar)"},
	"err.asarUnreadable": {Zh: "无法读取 %s", En: "Cannot read %s"},
	"err.asarEmpty":      {Zh: "路径不能为空", En: "Path must not be empty"},
	"err.asarNoSuchFile": {Zh: "找不到文件：%s", En: "File not found: %s"},
	"err.asarInvalid":    {Zh: "这个文件打不开或不是有效的 app.asar", En: "This file cannot be opened, or is not a valid app.asar"},
	"err.asarNoRuntimes": {Zh: "能打开 %s，但没从中认出任何 Agent 运行时——它可能不是 Cherry Studio 的 app.asar", En: "Opened %s but found no agent runtimes in it — it may not be Cherry Studio's app.asar"},
	"err.rememberFailed": {Zh: "找到 app.asar 但没能记住路径：%v", En: "Found app.asar but could not remember its path: %v"},
	"warn.asarNotFound":  {Zh: "没有自动找到 app.asar。数据库相关的功能仍然可用，但「运行时扫描」和「打补丁」需要你在界面上手动指定路径。", En: "Could not locate app.asar automatically. Database features still work, but runtime scanning and patching need you to set the path in the UI."},

	// ---------- 数据库 / Agent ----------
	"err.schemaMismatch": {Zh: "数据库结构不匹配（%s）：缺少列 %s。这通常意味着 Cherry Studio 换了表结构，请升级本工具或改用「补丁」后端。", En: "Database schema mismatch (%s): missing columns %s. Cherry Studio probably changed its schema — update this tool, or use the patch backend instead."},
	"err.schemaNoTable":  {Zh: "数据库结构不匹配（%s）：这个库里根本没有 agent 表。这通常意味着你选的不是 Cherry Studio 的数据目录，或者它换了存储方式。", En: "Database schema mismatch (%s): there is no agent table at all. This usually means the folder is not Cherry Studio's data directory, or it changed how it stores data."},
	"err.readAgents":     {Zh: "读取 Agent 列表失败", En: "Failed to read the agent list"},
	"err.readModels":     {Zh: "读取模型列表失败", En: "Failed to read the model list"},
	"err.agentNotFound":  {Zh: "找不到 Agent：%s", En: "Agent not found: %s"},
	"err.nameEmpty":      {Zh: "Agent 名字不能为空", En: "Agent name must not be empty"},
	"err.typeRequired":   {Zh: "必须指定运行时类型", En: "A runtime type is required"},
	"err.modelRequired":  {Zh: "必须指定模型", En: "A model is required"},
	"err.modelNotExist":  {Zh: "模型不存在：%s（必须从 user_model 里选）", En: "Model does not exist: %s (it must come from user_model)"},
	"err.createAgent":    {Zh: "写入 Agent 失败", En: "Failed to create the agent"},
	"err.updateAgent":    {Zh: "更新 Agent 失败", En: "Failed to update the agent"},
	"err.deleteAgent":    {Zh: "删除 Agent 失败", En: "Failed to delete the agent"},
	"err.duplicateAgent": {Zh: "复制 Agent 失败", En: "Failed to duplicate the agent"},
	"err.setDeleted":     {Zh: "更新删除状态失败", En: "Failed to update the deleted flag"},
	"err.orderKey":       {Zh: "计算排序键失败", En: "Failed to compute the order key"},
	"err.uuidFailed":     {Zh: "生成 UUID 失败", En: "Failed to generate a UUID"},
	"err.encodeConfig":   {Zh: "序列化 configuration 失败", En: "Failed to serialize configuration"},
	"err.permissionMode": {Zh: "运行时 %s 不支持权限模式 %q，可用：%v", En: "Runtime %s does not support permission mode %q; available: %v"},
	"err.runtimeMissing": {Zh: "当前 Cherry Studio 版本不包含运行时 %q。已识别到的运行时：%v", En: "This Cherry Studio build has no runtime %q. Runtimes found: %v"},
	"err.runtimeImpl":    {Zh: "运行时 %q 未被正确实现，拒绝创建", En: "Runtime %q is not properly implemented; refusing to create"},
	"err.verifyRuntime":  {Zh: "无法确认运行时是否真的内置：%v\n可以先在本工具的「环境」一栏指定 app.asar 的路径（通常在 <Cherry Studio 安装目录>/resources/app.asar）再重试。", En: "Could not verify that the runtime is actually bundled: %v\nSet the app.asar path in the Environment section of this tool first (usually <Cherry Studio install dir>/resources/app.asar), then retry."},
	"err.modelGone":      {Zh: "模型不存在：%s（必须从 user_model 里选）", En: "Model does not exist: %s (it must come from user_model)"},
	"err.badOffsetValue": {Zh: "非法偏移量: %q", En: "Invalid offset value: %q"},

	// ---------- 备份 ----------
	"err.backupFailed":   {Zh: "备份失败，已中止（未做任何修改）", En: "Backup failed; aborted with no changes made"},
	"err.backupDir":      {Zh: "创建备份目录失败", En: "Failed to create the backup directory"},
	"err.backupCopy":     {Zh: "备份 %s 失败", En: "Failed to back up %s"},
	"err.backupNothing":  {Zh: "没有任何文件需要备份", En: "There is nothing to back up"},
	"err.backupManifest": {Zh: "写入备份清单失败", En: "Failed to write the backup manifest"},
	"err.restoreFailed":  {Zh: "恢复 %s 失败", En: "Failed to restore %s"},
	"err.backupNotFound": {Zh: "找不到备份：%s", En: "Backup not found: %s"},
	"err.agentDir":       {Zh: "Agent 已创建，但数据目录建立失败", En: "Agent created, but its data directory could not be created"},

	// ---------- 补丁 ----------
	"patch.reason.noAsar":      {Zh: "没有指定 app.asar 路径", En: "No app.asar path given"},
	"patch.reason.openFailed":  {Zh: "无法打开 app.asar", En: "Could not open app.asar"},
	"patch.reason.scanFailed":  {Zh: "扫描失败", En: "Scan failed"},
	"patch.reason.noHidden":    {Zh: "这个版本没有隐藏任何运行时，无需打补丁", En: "This version hides no runtimes — no patch needed"},
	"patch.reason.noStatement": {Zh: "没有找到新建界面的过滤语句，可能这个版本改了实现方式", En: "Could not find the create-dialog filter; this version may implement it differently"},
	"patch.applied":            {Zh: "补丁已生效", En: "Patch is active"},
	"patch.err.notSupported":   {Zh: "无法打补丁：%s", En: "Cannot patch: %s"},
	"patch.err.stmtMissing":    {Zh: "在 %s 里找不到过滤语句（扫描与读取结果不一致）", En: "Filter statement not found in %s (scan and read disagree)"},
	"patch.err.noPredicate":    {Zh: "没有找到可替换的过滤条件", En: "No replaceable filter condition found"},
	"patch.err.lenMismatch":    {Zh: "内部错误：替换文本长度不一致（%d vs %d）", En: "Internal error: replacement length mismatch (%d vs %d)"},
	"patch.err.openRW":         {Zh: "以读写方式打开 app.asar 失败（Cherry Studio 是否还在运行？）", En: "Could not open app.asar for writing (is Cherry Studio still running?)"},
	"patch.err.writeFailed":    {Zh: "写入补丁失败", En: "Failed to write the patch"},
	"patch.err.syncFailed":     {Zh: "落盘失败", En: "Failed to flush to disk"},
	"patch.err.recordFailed":   {Zh: "补丁已应用，但撤销记录保存失败（请手动备份 app.asar）", En: "Patch applied, but saving the undo record failed (back up app.asar manually)"},
	"patch.err.verifyFailed":   {Zh: "补丁写入后复查失败，app.asar 可能未生效", En: "Post-patch verification failed; app.asar may not have taken effect"},
	"patch.err.noRecord":       {Zh: "没有找到针对该 app.asar 的补丁记录", En: "No patch record found for this app.asar"},
	"patch.err.badRecord":      {Zh: "补丁记录损坏", En: "Patch record is corrupt"},
	"patch.err.changed":        {Zh: "app.asar 在打补丁之后发生了变化（可能升级过 Cherry Studio），为避免改坏文件，已拒绝自动撤销。重装 Cherry Studio 即可回到干净状态", En: "app.asar changed after patching (Cherry Studio was probably updated). Refusing to auto-revert so the file is not damaged — reinstalling Cherry Studio restores a clean state"},

	// ---------- asar 解析（仅在文件损坏等异常时出现） ----------
	"err.asarReadHeader":     {Zh: "读取 asar 头部失败", En: "Failed to read the asar header"},
	"err.asarBadMagic":       {Zh: "不是有效的 asar 文件（头部标记为 %d，期望 4）", En: "Not a valid asar file (header marker is %d, expected 4)"},
	"err.asarBadHeaderSize":  {Zh: "asar 头部长度异常: %d", En: "Abnormal asar header length: %d"},
	"err.asarReadTree":       {Zh: "读取 asar 目录树失败", En: "Failed to read the asar file tree"},
	"err.asarHeaderTooShort": {Zh: "asar 头部长度不足以容纳 pickle 前缀", En: "The asar header is too short to hold the pickle prefix"},
	"err.asarTreeOOB":        {Zh: "asar 目录树长度越界: jsonLen=%d headerLen=%d", En: "asar file tree length out of range: jsonLen=%d headerLen=%d"},
	"err.asarParseTree":      {Zh: "解析 asar 目录树失败", En: "Failed to parse the asar file tree"},
	"err.asarNoFilesNode":    {Zh: "asar 目录树缺少 files 节点", En: "The asar file tree has no files node"},
	"err.asarEntryMissing":   {Zh: "asar 中不存在: %s", En: "Not present in the asar: %s"},
	"err.asarIsDir":          {Zh: "%s 是目录", En: "%s is a directory"},
	"err.asarBadOffset":      {Zh: "%s 的 offset 非法 (%q)", En: "Invalid offset for %s (%q)"},
	"err.asarReadFile":       {Zh: "读取 %s 失败", En: "Failed to read %s"},
	"err.asarUnpackedPatch":  {Zh: "%s 是 unpacked 文件，当前不支持对其打补丁", En: "%s is an unpacked file; patching it is not supported yet"},

	// ---------- 其他底层错误 ----------
	"err.writeConfig":       {Zh: "写入配置失败", En: "Failed to write the config"},
	"err.dataDirTriedPaths": {Zh: "没有找到 Cherry Studio 的数据目录（已尝试：%s）", En: "Cherry Studio data directory not found (tried: %s)"},
	"err.openDb":            {Zh: "打开数据库失败", En: "Failed to open the database"},
	"err.pingDb":            {Zh: "连接数据库失败", En: "Failed to connect to the database"},
	"err.readSchema":        {Zh: "读取 agent 表结构失败", En: "Failed to read the agent table schema"},
	"err.skipCurrentBackup": {Zh: "提示：撤销前备份当前状态时跳过（%v）", En: "note: skipped backing up the current state before restoring (%v)"},
	"err.verifyHint":        {Zh: "可以先在本工具的「环境」一栏指定 app.asar 的路径（通常在 <Cherry Studio 安装目录>/resources/app.asar）再重试。", En: "Set the app.asar path in the Environment section of this tool first (usually <Cherry Studio install dir>/resources/app.asar), then retry."},
	"err.unknownRuntime":    {Zh: "当前运行时不支持此操作", En: "This runtime does not support the operation"},

	// ---------- 界面其它 ----------
	"ui.langName": {Zh: "中文", En: "English"},
	"dup.suffix":  {Zh: " 副本", En: " copy"},
}

// T 渲染一条文案。缺失的 key 会原样返回，方便一眼看出漏翻。
func T(l Lang, key string, args ...any) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	s, ok := m[l]
	if !ok {
		s, ok = m[Zh]
		if !ok {
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Msg 是一条待渲染的文案：只带 key 与参数，不带语言。
type Msg struct {
	Key  string
	Args []any
}

// Text 按指定语言渲染。
func (m Msg) Text(l Lang) string { return T(l, m.Key, m.Args...) }

// Error 是一个携带 i18n key 的错误，可以在不同语言下渲染成不同文本。
type Error struct {
	M   Msg
	Err error
}

func (e *Error) Error() string { return e.M.Text(Zh) } // 兜底：无上下文时用中文
func (e *Error) Unwrap() error { return e.Err }
func (e *Error) Msg() Msg      { return e.M }

// E 构造一个 i18n 错误。
func E(key string, args ...any) *Error {
	return &Error{M: Msg{Key: key, Args: args}}
}

// Wrap 在保留底层错误的同时附带一条 i18n 文案。
func Wrap(key string, err error, args ...any) *Error {
	return &Error{M: Msg{Key: key, Args: args}, Err: err}
}

// keyed 让外层能取出错误里的文案 key。
type keyed interface{ Msg() Msg }

// Text 把任意 error 渲染成指定语言。
// 带 key 的错误按其文案渲染并附上底层原因；其余错误原样返回。
func Text(l Lang, err error) string {
	if err == nil {
		return ""
	}
	var k keyed
	if !errors.As(err, &k) {
		return err.Error()
	}
	base := k.Msg().Text(l)
	var e *Error
	if errors.As(err, &e) && e.Err != nil {
		return T(l, "err.withDetail", base, e.Err.Error())
	}
	return base
}
