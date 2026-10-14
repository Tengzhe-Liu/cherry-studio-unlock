# 从这里开始

这个包是 **Cherry Agent Manager v0.1.0** 的完整发布包：源码 + 预编译程序 + 文档。

---

## 包里有什么

```
cherry-agent-manager-v0.1.0/
├── START-HERE.md          ← 你正在看的这一份
├── dist/                  ← 五个平台的预编译程序（无需安装任何东西）
│   ├── cherry-agent-manager_windows_amd64.exe   ← Windows 用这个
│   ├── cherry-agent-manager_darwin_arm64        ← macOS Apple Silicon
│   ├── cherry-agent-manager_darwin_amd64        ← macOS Intel
│   ├── cherry-agent-manager_linux_amd64
│   └── cherry-agent-manager_linux_arm64
├── README.md              ← 项目说明（问题背景、原理、安全设计、排障）
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

### 第 3 步 · 运行工具

双击 `dist/cherry-agent-manager_windows_amd64.exe`。

浏览器会自动打开界面。**不需要任何命令行参数。**
（如果被防火墙拦了，允许即可 —— 它只监听本机 `127.0.0.1`，不对外。）

### 第 4 步 · 确认环境

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

### 第 5 步 · 解锁 dsh

两种做法，二选一或都做：

**方式 A（推荐）· 打补丁** —— 点「打补丁解锁」。
改的是 `app.asar` 里的 17 个字节，**文件大小不变**，随时可在同一按钮撤销。

**方式 B · 直接建 Agent** —— 点「新建 Agent」，运行时选 `dsh`。
不动程序文件，升级后通常也能继续用。

### 第 6 步 · 验证

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

右上角可切换 **中文 / English**，选择会被记住。
切换是整体生效的：界面文案、运行时状态、以及后端返回的报错都会跟着切。

命令行同样支持：`--lang en`。

---

## 想发到 GitHub

包里带着完整的 `.git` 提交历史（4 个提交），可以直接推送：

```bash
cd cherry-agent-manager-v0.1.0

# 1) 先把提交作者改成你自己（首次提交时用的是占位值）
git log -1 --format='%an <%ae>'          # 看当前是谁
git config user.name  "你的名字"
git config user.email "你的邮箱"

# 2) 建好空的 GitHub 仓库后
git remote add origin git@github.com:你的用户名/你的仓库名.git
git push -u origin main
```

**推送前建议再确认一次 Cherry Studio 的许可证**，README 的免责声明里已说明本项目
不包含它的任何代码，但你自己心里有数更稳妥。

发布 Release 时，把 `dist/` 里那五个文件作为附件传上去即可
（`dist/` 已被 `.gitignore` 排除，不会进仓库）。
