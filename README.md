# Cherry Agent Manager

**解锁 Cherry Studio 隐藏的 Agent 运行时，并管理你已有的 Agent。**

Cherry Studio 内置了 DeepSeek Harness（`dsh`）运行时——驱动、能力表、工具、图标、多语言全都在——
但**新建 Agent 的界面里选不到它**。本工具把这道人为的遮挡打开，并顺带给你一个 Agent 管理器。

> ⚠️ 非官方工具。会修改 Cherry Studio 的本地数据（数据库或 `app.asar`）。
> 所有写操作都会**先自动备份**，且大部分操作可一键撤销。请仍然自行留意备份。

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
| Windows | `cherry-agent-manager_windows_amd64.exe` |
| macOS (Apple Silicon) | `cherry-agent-manager_darwin_arm64` |
| macOS (Intel) | `cherry-agent-manager_darwin_amd64` |
| Linux | `cherry-agent-manager_linux_amd64` |

无依赖，不需要装 Node、Python 或任何运行时。

### 命令行

```bash
cherry-agent-manager                  # 自动探测环境并打开界面
cherry-agent-manager --status         # 只打印自检结果
cherry-agent-manager --port 8765      # 指定端口
cherry-agent-manager --no-open        # 不自动开浏览器
cherry-agent-manager --data-dir DIR   # 手动指定数据目录
cherry-agent-manager --asar FILE      # 手动指定 app.asar
```

### 从源码构建

```bash
git clone <this repo>
cd cherry-agent-manager
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

1. 上次记住的路径（存在数据目录下的 `cherry-agent-manager.json`）
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
  `<数据目录>/backups-manual/cherry-agent-manager/`，并写一份 `manifest.json`。
- **撤销本身也可撤销。** 从备份恢复前，会先把当前状态再存一份，所以点错也能回头。
- **等长替换。** 打补丁只把 `runtime !== "dsh"` 这 17 个字节换成 `true /*unlocked*/`——
  长度完全相同，因此不需要重写整个 288MB 的归档，也**不会改变文件大小**。
  补丁记录只保存被替换的那几个字节，撤销成本极低。
- **撤销前校验上下文。** Cherry Studio 升级后 `app.asar` 会变，旧偏移量就失效了。
  撤销前会比对改动位置两侧各 32 字节的哈希，不一致就拒绝执行，而不是把文件改坏。
- **拒绝往不认识的库里写。** 打开数据库时先校验 `agent` 表的列是否齐全；
  对不上就明确报错，绝不将错就错地塞数据。
- **拒绝创建跑不起来的 Agent。** 写入前会扫描 `app.asar`，确认目标运行时确实被实现了。
- **本地回环 + 只读 UI。** 服务只监听 `127.0.0.1`，不对外网暴露。

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

本项目与 Cherry Studio 官方无关，未经其授权或认可。
它修改的是**你自己机器上、你自己的数据**，目的是让你用上 App 已经内置的功能。
请自行评估风险并保留备份。如果 Cherry Studio 官方后续开放了这个运行时的创建入口，
这个工具就可以功成身退了。

## License

MIT
