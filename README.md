# Cherry Studio Unlock

**解锁 Cherry Studio 隐藏的 Agent 运行时，并管理你已有的 Agent。**
Unlock the agent runtimes Cherry Studio hides, and manage your agents.

> ⚠️ 非官方工具。会修改 Cherry Studio 的本地数据（数据库或 `app.asar`）。
> 所有写操作都会**先自动备份**，且大部分操作可一键撤销。请仍然自行留意备份。

**中文** ｜ [English](#english)

---

## 它解决什么问题

在 Cherry Studio v1.13.8（`app.asar` 内 `CherryStudio@2.1.6`）里可以看到：

**运行时确实被实现了，并注册进了驱动表**（`out/main/main.js`）：

```js
runtimeDriverRegistry.register(new LazyClaudeCodeRuntimeDriver());
runtimeDriverRegistry.register(new PiRuntimeDriver());
runtimeDriverRegistry.register(new DshRuntimeDriver());   // ← dsh 在
```

`AGENT_RUNTIME_CAPABILITIES` 里也有完整的一项，界面文案是「DeepSeek Harness」。

**但新建 Agent 的选择器把它过滤掉了**（`out/renderer/assets/EditDialogShared-*.js`）：

```js
var RUNTIMES = Object.keys(AGENT_RUNTIME_CAPABILITIES).filter((runtime) => runtime !== "dsh");
```

于是「运行时模式」只剩 Claude Agent 和 Pi 两个可选项。

本工具做两件事，二选一或都做：

| 方式 | 做法 | 效果 |
|---|---|---|
| **打补丁**（推荐） | 把 `runtime !== "dsh"` 换成**等长的**恒真表达式 | Cherry Studio 原生界面里直接能新建 dsh Agent |
| **写数据库** | 直接往 `agent` 表插一行 `type='dsh'` | 不用改程序文件，升级后通常仍可用 |

---

## 快速开始

### 下载即用

到 [Releases](../../releases) 下载对应平台的文件，双击运行，浏览器会自动打开界面。

| 平台 | 文件 |
|---|---|
| Windows | `cherry-studio-unlock_windows_amd64.exe` |
| macOS (Apple Silicon) | `cherry-studio-unlock_darwin_arm64` |
| macOS (Intel) | `cherry-studio-unlock_darwin_amd64` |
| Linux | `cherry-studio-unlock_linux_amd64` |

无依赖，不需要装 Node、Python 或任何运行时。

### 命令行

```bash
cherry-studio-unlock                  # 自动探测环境并打开界面
cherry-studio-unlock --status         # 只打印自检结果
cherry-studio-unlock --lang en        # 以英文界面启动
cherry-studio-unlock --port 8765      # 指定端口
cherry-studio-unlock --no-open        # 不自动开浏览器
cherry-studio-unlock --data-dir DIR   # 手动指定数据目录
cherry-studio-unlock --asar FILE      # 手动指定 app.asar
```

### 界面语言

界面右上角可以在 **中文 / English** 之间切换，选择会被记住。
切换是**整体生效**的：UI 文案、运行时状态判定、以及后端返回的错误信息都会跟着切换，
不会出现"界面是英文、报错是中文"的割裂。

命令行同样支持：`--lang zh|en`。未指定时的优先级是
**命令行参数 → 配置文件 → 环境变量（`LANG` / `LC_ALL`）→ 中文**。

### 从源码构建

```bash
git clone <this repo>
cd cherry-studio-unlock
make build        # 或：go build .
make release      # 交叉编译五个平台到 dist/
```

只需要 Go 1.22+，**不需要 cgo**（SQLite 用的是纯 Go 实现）。

---

## 使用步骤

1. **完全退出 Cherry Studio** —— 包括右下角托盘图标 → 退出。
   它在内存里缓存了 Agent 列表，运行中写入会被覆盖。工具会检测并拒绝写操作。
2. 运行本工具。
3. 在「运行时」一栏确认 `dsh` 显示为「被隐藏」。
4. 点 **打补丁解锁**（或跳过这步，直接用下面的「新建 Agent」）。
5. 重新打开 Cherry Studio —— 新建 Agent 时就能选到 **DeepSeek Harness** 了。

如果你更想走数据库那条路：直接在工具里点「新建 Agent」，运行时选 `dsh` 即可。

想改回去？点「撤销补丁」，或用任意一条备份一键恢复。

---

## 遇到问题

### 「运行时」是空的 / 选不了运行时

**原因：没找到 `app.asar`。** 运行时列表是从 `app.asar` 里扫出来的，扫不到就列不出来。

Cherry Studio 装在标准位置（如 `Program Files`）时工具能自动找到；装在自定义目录
（如 `C:\app\AI\Cherry Studio`）时，工具按这个顺序找：

1. 上次记住的路径（存在数据目录下的 `cherry-studio-unlock.json`）
2. 正在运行的进程 —— **但写数据前你必须先退出 Cherry Studio，这条路就断了**
3. Windows 注册表的卸载项
4. 常见安装位置

如果都没命中，在「环境」一栏手动填入路径即可，**填一次会记住**：

```
<Cherry Studio 安装目录>\resources\app.asar
```

不确定装在哪？在 Cherry Studio 里看「关于」，或从开始菜单快捷方式上右键 →
打开文件所在位置。

> v0.1 的初版有过这个 bug：只靠「运行中的进程 + 常见路径」两条线，
> 于是用户按提示**退出 Cherry Studio 之后**反而找不到 `app.asar`，
> 而退出恰恰是唯一允许写数据的场景。现已加入注册表反查与路径记忆，
> 并有端到端回归测试（`TestAsarLookupDoesNotDependOnRunningProcess`）
> 锁住这个行为。

### 保存按钮是灰的

- **Cherry Studio 正在运行** —— 请完全退出（含托盘图标）后重开本工具。
- **上方有红框提示列不出运行时** —— 需要先指定 `app.asar`，
  或明确勾选「跳过运行时校验」（不推荐，可能创建跑不起来的 Agent）。

### 打了补丁但界面里还是没有 dsh

1. 确认补丁状态显示「已生效」；
2. **重启 Cherry Studio** —— 补丁改的是启动时加载的文件；
3. 如果重启后仍没有，说明这个版本的过滤方式变了，
   请跑一次 `--status` 并把结果贴到 issue。

---

## 安全设计

这个工具会碰你的数据，所以设计上做了这些事：

- **写前必备份。** 每次写操作前把 `cherrystudio.sqlite`（连同 `-wal`/`-shm`）复制到
  `<数据目录>/backups-manual/cherry-studio-unlock/`，并写一份 `manifest.json`。
- **撤销本身也可撤销。** 从备份恢复前，会先把当前状态再存一份，所以点错也能回头。
- **等长替换。** 打补丁只把 `runtime !== "dsh"` 这 17 个字节换成 `true /*unlocked*/`——
  长度完全相同，因此不需要重写整个 288MB 的归档，也**不会改变文件大小**。
  补丁记录只保存被替换的那几个字节，撤销成本极低。
- **撤销前校验上下文。** Cherry Studio 升级后 `app.asar` 会变，旧偏移量就失效了。
  撤销前会比对改动位置两侧各 32 字节的哈希，不一致就拒绝执行，而不是把文件改坏。
- **拒绝往不认识的库里写。** 打开数据库时先校验 `agent` 表的列是否齐全；
  对不上就明确报错，绝不将错就错地塞数据。
- **拒绝创建跑不起来的 Agent。** 写入前会扫描 `app.asar`，确认目标运行时确实被实现了。
- **本地回环。** 服务只监听 `127.0.0.1`，同网段和外网都碰不到。
- **校验 Host 与 Origin。** 只监听回环还不够：恶意网页可以用不触发 CORS 预检的
  「简单请求」向 `127.0.0.1:端口` 发 POST，虽然读不到响应，但删除 Agent、打补丁这些
  副作用已经发生了。所以改数据的请求会校验 `Origin` 必须同源，`Host` 必须指向本机
  （后者同时挡住 DNS rebinding）。

---

## 它是怎么工作的

```
internal/
├── asar/       纯 Go 解析 Electron 的 .asar 归档（含 poke 式的原地改写支持）
├── cherry/     环境探测：数据目录、进程状态、运行时扫描、备份
├── store/      SQLite 读写 + fractional-indexing 排序键算法
├── patch/      app.asar 补丁（等长替换 + 可撤销）
├── service/    用例编排：备份 → 校验 → 写 → 建目录 → 自检
└── web/        本地 HTTP 服务 + 内嵌界面
```

两个值得一提的地方：

**排序键算法是逐行移植的。** Cherry Studio 用 `fractional-indexing@3.2.0` 生成 `order_key`。
我们自己造一套键的话，新 Agent 会和 App 后续新建的 Agent 抢位置。所以这里把上游实现完整移植到 Go，
并用从**真实 `app.asar` 里提取的那份 JS 实现**生成 94 条测试向量逐条比对——
断言的是"和 Cherry Studio 生成的键完全一致"，不是"看起来差不多"。

**运行时是扫出来的，不是写死的。** 工具不认 `dsh` 这个名字，而是从 `app.asar` 里解析出
`AGENT_RUNTIME_CAPABILITIES` 的键集合、`runtimeDriverRegistry.register(...)` 的驱动列表，
以及给新建界面做过滤的那条语句。以后 Cherry Studio 藏了别的运行时，工具会自动发现。

---

## 已知限制

- **Cherry Studio 升级后补丁会失效。** 升级会覆盖 `app.asar`，重新跑一次即可。
- **数据库表结构大改时「写数据库」那条路会失效**，届时工具会明确报错而不是写坏数据。
- **只处理 `runtime !== "X"` 形式的过滤。** 如果某版本改用 `=== ` 白名单形式，
  工具会拒绝打补丁并说明原因（不会瞎改）。
- **不做 diff 式重打包。** 需要改变文件长度时（例如过滤语句结构变化）目前不支持。

---

## Roadmap

- [ ] **注入全新的第三方运行时**（opencode 等）。
      这不是「解锁」而是「实现」——需要对着 Cherry Studio 未公开的内部接口写完整驱动，
      并补丁主进程与渲染进程。可行性评估见 [`docs/runtime-injection.md`](docs/runtime-injection.md)。
- [ ] 备份自动清理（保留最近 N 份）
- [ ] Agent 导入/导出

---

## 贡献

欢迎 issue 与 PR。特别是：**在别的 Cherry Studio 版本上跑一次 `--status`，把结果贴出来**——
不同版本的打包产物结构可能不一样，这能帮工具适配得更广。

```bash
go test ./...                                                  # 单元测试
CHERRY_ASAR=/path/to/app.asar     go test ./... -run Real -v    # 对着真实归档做集成测试
CHERRY_DB=/path/to/cherrystudio.sqlite go test ./... -run CRUD -v
```

集成测试都会先复制一份再到副本上操作，不会碰你的正式数据。

---

## 免责声明

本项目与 Cherry Studio 官方**无关**，未获其授权或认可，也不代表其立场。
项目名与文档中出现的 "Cherry Studio" 仅用于说明兼容对象，不构成任何官方关联的暗示。
本项目**不包含也不分发** Cherry Studio 的任何代码或二进制。

### 为什么官方要隐藏 dsh？——我们不知道，也不猜

工具只描述**可观察到的现象**（`app.asar` 里那行 `.filter((runtime) => runtime !== "dsh")`），
不替官方解释动机。有一个值得注意的技术细节：

```js
"claude-code": { ..., heartbeat: true,  ... },
pi:            { ..., heartbeat: true,  ... },
dsh:           { ..., heartbeat: false, ... },   // ← 只有它是 false
```

`heartbeat` 为 `false` 意味着该运行时不被支持用于定时任务（心跳）。
这**可能**说明 dsh 在那个版本里还不够完备——但这是推测，不是结论。

**本工具不会改变这一点。** 它只是把创建入口打开，运行时的成熟度仍取决于官方实现。
是否要在重要场景里使用 dsh Agent，请自行判断。

### 风险

- 修改的是**你自己机器上、你自己的数据**；
- 所有写操作前都会自动备份，且大部分可撤销——但**没有任何担保**，请仍然自行保留备份；
- 打补丁改的是程序文件，**每次 Cherry Studio 升级后需要重新打一次**；
- 若官方后续开放了这个运行时的创建入口，本工具即可功成身退。

## 致谢

这个工具是站在别人肩膀上做出来的：

- **[Cherry Studio](https://github.com/CherryHQ/cherry-studio)** —— 宿主应用。本工具所做的一切，都建立在它**已经内置了 dsh 运行时**这个事实之上。
- **[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（`dsh`，MIT）** —— 本工具存在的理由。它不是我们写的；我们只是把 Cherry Studio 里已经装好、却在界面上被藏起来的入口打开。
- **[fractional-indexing](https://github.com/rocicorp/fractional-indexing)**（Rocicorp，MIT）—— Cherry Studio 用它生成 `order_key`。本工具逐行移植了这份实现，确保两边生成的键完全一致。
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)**（BSD-3-Clause）—— 纯 Go 的 SQLite 实现，让本工具不需要 cgo 就能交叉编译。

**特别说明**：以上项目与本工具**没有隶属或认可关系**。列在这里是为了说明技术来源，不构成任何关联的宣称。

## License

本项目代码以 [MIT](LICENSE) 授权。
第三方组件的许可与出处见 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)。

---
---

# English

**Unlock the agent runtimes Cherry Studio hides, and manage your agents.**

Cherry Studio ships the DeepSeek Harness (`dsh`) runtime — driver, capability table, tools,
icons, translations, all of it — but **you cannot pick it in the new-agent dialog**.
This tool opens that artificial block, and gives you an agent manager along the way.

> ⚠️ Unofficial tool. It modifies Cherry Studio's local data (its database, or `app.asar`).
> Every write is **backed up automatically** first, and most actions undo with one click.
> Keep your own backups anyway.

[中文](#cherry-studio-unlock) ｜ **English**

---

## The problem it solves

In Cherry Studio v1.13.8 (`CherryStudio@2.1.6` inside `app.asar`):

**The runtime really is implemented and registered** (`out/main/main.js`):

```js
runtimeDriverRegistry.register(new LazyClaudeCodeRuntimeDriver());
runtimeDriverRegistry.register(new PiRuntimeDriver());
runtimeDriverRegistry.register(new DshRuntimeDriver());   // ← dsh is right here
```

`AGENT_RUNTIME_CAPABILITIES` has a complete entry for it too, labelled "DeepSeek Harness".

**But the create dialog filters it out** (`out/renderer/assets/EditDialogShared-*.js`):

```js
var RUNTIMES = Object.keys(AGENT_RUNTIME_CAPABILITIES).filter((runtime) => runtime !== "dsh");
```

So "Runtime mode" offers only Claude Agent and Pi.

This tool does two things — pick either or both:

| Approach | What it does | Result |
|---|---|---|
| **Patch** (recommended) | Replaces `runtime !== "dsh"` with an **equal-length** always-true expression | dsh becomes selectable in Cherry Studio's own dialog |
| **Database write** | Inserts a row with `type='dsh'` into the `agent` table | Touches no program files, and usually survives updates |

---

## Quick start

### Download and run

Grab the file for your platform from [Releases](../../releases), run it, and your browser
opens the UI automatically.

| Platform | File |
|---|---|
| Windows | `cherry-studio-unlock_windows_amd64.exe` |
| macOS (Apple silicon) | `cherry-studio-unlock_darwin_arm64` |
| macOS (Intel) | `cherry-studio-unlock_darwin_amd64` |
| Linux (x86_64) | `cherry-studio-unlock_linux_amd64` |
| Linux (ARM64) | `cherry-studio-unlock_linux_arm64` |

No dependencies — no Node, no Python, no runtime to install.

### Command line

```bash
cherry-studio-unlock                  # detect the environment and open the UI
cherry-studio-unlock --status         # print the self-check and exit
cherry-studio-unlock --lang en        # start with the English UI
cherry-studio-unlock --port 8765      # pick a port
cherry-studio-unlock --no-open        # do not open a browser
cherry-studio-unlock --data-dir DIR   # set the data directory manually
cherry-studio-unlock --asar FILE      # set app.asar manually
```

### Interface language

The top-right corner switches between **中文 / English**, and the choice is remembered.
The switch is **global**: UI text, runtime verdicts, and backend error messages all follow
it — you will not get an English UI that throws Chinese errors.

The CLI supports it too: `--lang zh|en`. When omitted the order is
**flag → config file → environment (`LANG` / `LC_ALL`) → Chinese**.

### Build from source

```bash
git clone <this repo>
cd cherry-studio-unlock
make build        # or: go build .
make release      # cross-compile all five platforms into dist/
```

Go 1.22+ is all you need. **No cgo** — SQLite is a pure-Go implementation.

---

## Steps

1. **Quit Cherry Studio completely** — including the tray icon → Quit.
   It caches the agent list in memory and would overwrite your changes;
   the tool detects this and refuses to write.
2. Run this tool.
3. In the Runtimes card, confirm `dsh` shows as **Hidden**.
4. Click **Unlock with patch** (or skip this and use "New agent" below).
5. Reopen Cherry Studio — **DeepSeek Harness** is now selectable when creating an agent.

Prefer the database route? Just click **New agent** in this tool and pick `dsh`.

Want to go back? Click **Revert patch**, or restore any backup with one click.

---

## Troubleshooting

### The Runtimes list is empty / the runtime cannot be selected

**Cause: `app.asar` was not found.** The runtime list is scanned out of `app.asar`.

If Cherry Studio is installed in the standard location (e.g. `Program Files`) the tool finds
it automatically. Otherwise it tries, in order:

1. The path remembered from last time (`cherry-studio-unlock.json` in the data directory)
2. A running process — **but you must quit Cherry Studio before writing, so this line goes dead**
3. The Windows registry uninstall entry
4. Common install locations

If none hit, set the path manually in the Environment card — **it is remembered after the first time**:

```
<Cherry Studio install dir>\resources\app.asar
```

Not sure where it is installed? Check "About" inside Cherry Studio, or right-click the Start
menu shortcut → Open file location.

> v0.1's first release had this bug: it only used "running process + standard paths", so a user
> who followed the instructions and **quit Cherry Studio first** could no longer find `app.asar` —
> and quitting is the one scenario where writing is allowed. Registry lookup and path memory are
> now in place, locked down by an end-to-end regression test
> (`TestAsarLookupDoesNotDependOnRunningProcess`).

### The Save button is greyed out

- **Cherry Studio is running** — quit it completely (including the tray icon) and reopen this tool.
- **A red box says runtimes cannot be listed** — set `app.asar` first, or explicitly tick
  "Skip runtime check" (not recommended; you may create an agent that cannot run).

### Patched, but dsh still is not in the dialog

1. Confirm the patch state says **active**;
2. **Restart Cherry Studio** — the patch changes a file loaded at startup;
3. If it still is not there, this version filters differently. Run `--status` and paste the
   output into an issue.

---

## Safety design

This tool touches your data, so:

- **Backup before every write.** `cherrystudio.sqlite` (with `-wal`/`-shm`) is copied to
  `<data dir>/backups-manual/cherry-studio-unlock/` along with a `manifest.json`.
- **Undo is itself undoable.** Before restoring a backup, the current state is saved too.
- **Equal-length replacement.** The patch swaps the 17 bytes of `runtime !== "dsh"` for
  `true /*unlocked*/` — exactly the same length. That means no need to rewrite the whole
  288 MB archive, and **the file size does not change**. Only the replaced bytes are backed up.
- **Context check before revert.** A Cherry Studio update changes `app.asar`, invalidating the
  old offset. Before reverting, the tool hashes 32 bytes on either side of the change and
  refuses to run if they differ — rather than corrupting the file.
- **Refuses unknown schemas.** Opening the database validates the `agent` table's columns first;
  a mismatch is a hard error, never a half-blind insert.
- **Refuses to create agents that cannot run.** It scans `app.asar` before writing to confirm
  the target runtime is actually implemented.
- **Loopback only.** The server listens on `127.0.0.1`; other machines cannot reach it.
- **Host and Origin checks.** Loopback alone is not enough: a malicious web page can send a
  POST to `127.0.0.1:port` using a CORS-*simple* request. It cannot read the response, but the
  side effect — deleting an agent, applying the patch — already happened. So mutating requests
  must have a same-origin `Origin`, and `Host` must point at this machine (which also blocks
  DNS rebinding).

---

## How it works

```
internal/
├── asar/       pure-Go parser for Electron .asar archives (with in-place poke support)
├── cherry/     environment: data dir, process state, runtime scan, backups
├── store/      SQLite access + the fractional-indexing order-key algorithm
├── patch/      app.asar patching (equal-length replacement, revertible)
├── service/    use-case orchestration: back up → validate → write → mkdir → verify
└── web/        local HTTP server + embedded UI
```

Two things worth calling out:

**The order-key algorithm is a line-by-line port.** Cherry Studio generates `order_key` with
`fractional-indexing@3.2.0`. Inventing our own keys would make new agents collide with ones the
app creates later. So the upstream implementation is ported to Go in full, and checked against
**94 test vectors generated from the JS implementation extracted out of the real `app.asar`** —
the assertion is "byte-identical to what Cherry Studio generates", not "close enough".

**Runtimes are scanned, not hard-coded.** The tool does not know the name `dsh`; it parses the
keys of `AGENT_RUNTIME_CAPABILITIES`, the driver list from `runtimeDriverRegistry.register(...)`,
and the filter statement guarding the create dialog, all out of `app.asar`. If Cherry Studio
hides another runtime later, the tool finds it by itself.

---

## Known limitations

- **A Cherry Studio update breaks the patch.** Updates overwrite `app.asar`; just re-run it.
- **A major schema change breaks the database route** — the tool errors out clearly instead of writing bad data.
- **Only the `runtime !== "X"` filter form is handled.** If a version switches to an `===` allowlist,
  the tool refuses to patch and says why (it will not guess).
- **No diff-style repacking.** Changing the file length (if the filter statement structure changes)
  is not supported yet.

---

## Roadmap

- [ ] **Inject brand-new third-party runtimes** (opencode, …).
      That is not "unlocking" but "implementing": writing a full driver against Cherry Studio's
      undocumented internals and patching both the main and renderer bundles. Feasibility report:
      [`docs/runtime-injection.md`](docs/runtime-injection.md).
- [ ] Automatic backup pruning (keep the last N)
- [ ] Agent import/export

---

## Contributing

Issues and PRs welcome. Especially: **run `--status` against a different Cherry Studio version
and paste the output** — bundle layouts differ between versions, and that is what makes the tool
adaptable.

```bash
go test ./...                                                    # unit tests
CHERRY_ASAR=/path/to/app.asar     go test ./... -run Real -v      # integration vs. a real archive
CHERRY_DB=/path/to/cherrystudio.sqlite go test ./... -run CRUD -v
```

Every integration test copies the archive/database first and works on the copy — your real data
is never touched.

---

## Disclaimer

This project is **not affiliated with, authorized by, or endorsed by** Cherry Studio, and does not
represent its positions. "Cherry Studio" appears here only to name what the tool is compatible
with. This project **contains and distributes none** of Cherry Studio's code or binaries.

### Why does the vendor hide dsh? — We don't know, and we don't guess

The tool describes only **observable facts** (that `.filter((runtime) => runtime !== "dsh")` line
in `app.asar`) and does not attribute motives. One detail is worth noting though:

```js
"claude-code": { ..., heartbeat: true,  ... },
pi:            { ..., heartbeat: true,  ... },
dsh:           { ..., heartbeat: false, ... },   // ← the only one that is false
```

`heartbeat: false` means the runtime is not supported for scheduled tasks (heartbeats).
That *may* indicate dsh was not fully finished in that version — but that is speculation, not a
conclusion.

**This tool does not change any of that.** It only opens the creation entry point; the runtime's
maturity is still up to the vendor's implementation. Judge for yourself whether to rely on dsh
agents for anything important.

### Risks

- It modifies **your own data on your own machine**;
- Every write is backed up first and most actions are revertible — but there is **no warranty**.
  Keep your own backups;
- Patching modifies program files and **must be re-applied after every Cherry Studio update**;
- If the vendor re-enables this runtime, this tool has served its purpose and can retire.

## Acknowledgements

This tool stands on other people's shoulders:

- **[Cherry Studio](https://github.com/CherryHQ/cherry-studio)** — the host application. Everything this tool does rests on the fact that it **already ships the dsh runtime**.
- **[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (`dsh`, MIT)** — the reason this tool exists. We did not write it; we only opened an entry point that was already installed inside Cherry Studio but hidden in the UI.
- **[fractional-indexing](https://github.com/rocicorp/fractional-indexing)** (Rocicorp, MIT) — Cherry Studio uses it to generate `order_key`. We ported the implementation line by line so both sides produce identical keys.
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** (BSD-3-Clause) — the pure-Go SQLite implementation that lets this tool cross-compile without cgo.

**Note**: none of the projects above are affiliated with or endorse this tool. They are listed to credit where the technology comes from, not to claim any association.

## License

MIT — see [LICENSE](LICENSE).
Third-party components and their licenses: [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).
