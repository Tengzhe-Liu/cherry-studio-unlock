# START HERE · 从这里开始

**Cherry Agent Manager v0.1.0** — source code + prebuilt binaries + docs.
完整发布包：源码 + 预编译程序 + 文档。

**中文** ｜ [English](#english)

---

# 中文

## 包里有什么

```
cherry-agent-manager-v0.1.0/
├── START-HERE.md          ← 你正在看的这一份
├── dist/                  ← 五个平台的预编译程序（无需安装任何东西）
├── README.md              ← 项目说明：问题背景、原理、安全设计、排障
├── docs/
│   └── runtime-injection.md   ← 「注入全新运行时」的可行性报告
├── THIRD-PARTY-NOTICES.md ← 第三方组件许可
├── LICENSE                ← MIT
├── main.go / internal/ / tools/ / Makefile / go.mod   ← 完整源码
└── .git/                  ← 完整提交历史，可直接推到 GitHub
```

---

## 跑一遍安装流程

### 第 1 步 · 解压

把 zip 解压到任意目录。**路径里最好不要有中文和空格**，省得命令行出岔子。

### 第 2 步 · 完全退出 Cherry Studio

⚠️ **这一步不能省。**

从右下角**托盘图标**右键 → 退出。只关窗口不算，后台进程还在。

原因：Cherry Studio 会把 Agent 列表缓存在内存里，运行中写入会被它覆盖。
工具检测到它在运行时会直接拒绝写操作。

### 第 3 步 · 打开 `dist` 文件夹，按你的系统选启动文件

进入 `dist` 目录，**只挑你系统对应的那一个**：

| 你的系统 | 选这个文件 |
|---|---|
| **Windows**（绝大多数电脑） | `cherry-agent-manager_windows_amd64.exe` |
| **macOS** · Apple 芯片（M1/M2/M3/M4） | `cherry-agent-manager_darwin_arm64` |
| **macOS** · Intel 芯片 | `cherry-agent-manager_darwin_amd64` |
| **Linux** · x86_64 | `cherry-agent-manager_linux_amd64` |
| **Linux** · ARM64 | `cherry-agent-manager_linux_arm64` |

不确定自己的 Mac 是哪种芯片？屏幕左上角  → 「关于本机」，看「芯片」一栏。

**macOS / Linux 用户注意**：命令行工具默认没有执行权限，第一次要先给它：

```bash
cd 解压后的目录/dist
chmod +x cherry-agent-manager_darwin_arm64     # 换成你选的那个文件名

# macOS 上如果提示「无法验证开发者」，再补一句：
xattr -d com.apple.quarantine cherry-agent-manager_darwin_arm64
```

Windows 用户不需要这一步，直接双击即可。

### 第 4 步 · 运行它

双击（macOS / Linux 则在终端里执行 `./文件名`）。

浏览器会自动打开界面。**不需要任何命令行参数。**
如果被防火墙拦了，允许即可 —— 它只监听本机 `127.0.0.1`，不对外。

### 第 5 步 · 确认环境

看界面上的「环境」卡片，应该显示：

| 项 | 期望值 |
|---|---|
| 数据目录 | `...\AppData\Roaming\CherryStudio\Data` |
| 程序版本 | `2.1.6` 或你实际装的版本 |
| app.asar | 能自动认出路径 |
| Cherry Studio | 未运行 |

再往下看「运行时」卡片，应该看到三行：

```
✔ claude-code   已内置且可直接新建
✖ dsh           已内置，但新建界面里选不到——本工具可以解锁
✔ pi            已内置且可直接新建
```

**如果 app.asar 显示「未找到」**：把它填进「环境」一栏的输入框再点「保存并扫描」。
填一次会记住，下次不用再填。

### 第 6 步 · 解锁 dsh

两种做法，二选一或都做：

**方式 A（推荐）· 打补丁** —— 点「打补丁解锁」。
改的是 `app.asar` 里的 17 个字节，**文件大小不变**，随时可在同一按钮撤销。

**方式 B · 直接建 Agent** —— 点「新建 Agent」，运行时选 `dsh`。
不动程序文件，升级后通常也能继续用。

### 第 7 步 · 验证

重新打开 Cherry Studio → 新建 Agent。

- 走了**方式 A**：新建界面里应能看到第三个选项 **DeepSeek Harness**。
- 走了**方式 B**：直接去 Agent 列表看那个新建的 Agent 在不在、能不能开会话。

---

## 出问题时

| 现象 | 处理 |
|---|---|
| 「运行时」是空的 | app.asar 没找到 —— 在「环境」里手动填路径 |
| 保存按钮是灰的 | Cherry Studio 还在运行，或没选运行时 |
| 打了补丁但界面里还是没有 dsh | 补丁改的是启动时加载的文件，**必须重启 Cherry Studio** |
| 想全部还原 | 「备份与撤销」里任何一条都能一键恢复；补丁也能单独撤销 |

更详细的原因分析见 `README.md` 的「遇到问题」一节。

---

## 界面语言

界面右上角可切换 **中文 / English**，选择会被记住。
切换是整体生效的：界面文案、运行时状态、以及后端返回的报错都会跟着切。

命令行同样支持：`--lang en`。

---
---

# English

## What's in this package

```
cherry-agent-manager-v0.1.0/
├── START-HERE.md          ← the file you are reading
├── dist/                  ← prebuilt binaries for five platforms (nothing to install)
├── README.md              ← project overview: the problem, how it works, safety, troubleshooting
├── docs/
│   └── runtime-injection.md   ← feasibility report on injecting brand-new runtimes
├── THIRD-PARTY-NOTICES.md ← third-party licenses
├── LICENSE                ← MIT
├── main.go / internal/ / tools/ / Makefile / go.mod   ← full source
└── .git/                  ← complete commit history, ready to push to GitHub
```

---

## Walk through the install flow

### Step 1 · Unzip

Extract the archive anywhere. **Avoid non-ASCII characters and spaces in the path** —
they tend to break command-line tooling.

### Step 2 · Quit Cherry Studio completely

⚠️ **Do not skip this.**

Right-click the **tray icon** → Quit. Closing the window is not enough; the process keeps running.

Why: Cherry Studio caches the agent list in memory and would overwrite your changes.
This tool detects that it is running and refuses all writes.

### Step 3 · Open the `dist` folder and pick the file for your system

Go into `dist` and take **only** the one that matches your OS:

| Your system | Pick this file |
|---|---|
| **Windows** (most PCs) | `cherry-agent-manager_windows_amd64.exe` |
| **macOS** · Apple silicon (M1/M2/M3/M4) | `cherry-agent-manager_darwin_arm64` |
| **macOS** · Intel | `cherry-agent-manager_darwin_amd64` |
| **Linux** · x86_64 | `cherry-agent-manager_linux_amd64` |
| **Linux** · ARM64 | `cherry-agent-manager_linux_arm64` |

Not sure which chip your Mac has? Apple menu  → "About This Mac" → look at "Chip".

**macOS / Linux users**: command-line tools are not executable by default. Make it so first:

```bash
cd path-to-unzipped/dist
chmod +x cherry-agent-manager_darwin_arm64     # substitute your chosen file name

# On macOS, if you get "cannot verify the developer", also run:
xattr -d com.apple.quarantine cherry-agent-manager_darwin_arm64
```

Windows users can skip all of that — just double-click.

### Step 4 · Run it

Double-click it (on macOS/Linux, run `./filename` in a terminal).

Your browser opens automatically. **No command-line arguments are needed.**
If a firewall prompt appears, allow it — the tool only listens on `127.0.0.1`, never externally.

### Step 5 · Check the environment

Look at the **Environment** card:

| Field | Expected |
|---|---|
| Data directory | `...\AppData\Roaming\CherryStudio\Data` |
| App version | `2.1.6`, or whatever you actually have |
| app.asar | detected automatically |
| Cherry Studio | not running |

Below that, the **Runtimes** card should list three rows:

```
✔ claude-code   Built in and selectable
✖ dsh           Built in but hidden from the create dialog — this tool can unlock it
✔ pi            Built in and selectable
```

**If app.asar shows "not found"**: paste its path into the input in the Environment card
and click "Save & scan". It is remembered — you only do this once.

### Step 6 · Unlock dsh

Two ways; do either or both:

**Option A (recommended) · Patch** — click "Unlock with patch".
This rewrites 17 bytes inside `app.asar`. **The file size does not change**, and the same
button reverts it at any time.

**Option B · Create the agent directly** — click "New agent" and choose `dsh` as the runtime.
Nothing outside your database is touched, so it usually survives app updates.

### Step 7 · Verify

Reopen Cherry Studio → create a new agent.

- After **Option A**: a third tile, **DeepSeek Harness**, appears in the create dialog.
- After **Option B**: check the agent list — the new agent should be there and able to start a session.

---

## When something goes wrong

| Symptom | What to do |
|---|---|
| The Runtimes list is empty | app.asar wasn't found — set its path in the Environment card |
| The Save button is greyed out | Cherry Studio is running, or no runtime is selected |
| Patched, but dsh still isn't in the dialog | The patch changes a file loaded at startup — **restart Cherry Studio** |
| Want to undo everything | Any entry under Backups & undo restores with one click; the patch reverts separately |

See the troubleshooting section of `README.md` for deeper explanations.

---

## Interface language

The top-right corner switches between **中文 / English** and remembers your choice.
The switch is global: UI text, runtime status, and backend error messages all follow it —
you will never see an English UI with a Chinese error.

The CLI supports it too: `--lang en`.
