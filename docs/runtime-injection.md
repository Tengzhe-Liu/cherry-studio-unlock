# 给 Cherry Studio 注入全新 Agent 运行时 —— 可行性评估

> 分析对象：Cherry Studio v1.13.8（内置 `app.asar` 中 `CherryStudio@2.1.6`，
> 主进程 `out/main/main.js`，渲染进程 `out/renderer/assets/`）。
> 本文所有结论都来自对打包产物的实际阅读，不是推测。

## 结论先说

| | 解锁**已内置**的运行时（如 `dsh`） | 注入**全新**的第三方运行时（如 opencode） |
|---|---|---|
| 本质 | 开一个被关掉的开关 | 从零实现一个运行时 |
| 工作量 | 半天 | 数周级，且无文档可依 |
| Cherry Studio 升级后 | 通常仍可用 | 大概率失效 |
| 是否适合放进本工具 | 适合，已实现 | 不适合，应单独立项 |

**建议：v0.1 只做「解锁」。注入留作独立研究项目。**

---

## 一、运行时是怎么注册的

主进程里，驱动注册表是个极简的 Map：

```js
var RuntimeDriverRegistry = class {
    drivers = new Map();
    register(driver) { this.drivers.set(driver.type, driver); }
    get(type) { return this.drivers.get(type); }
    getAgentSessionDriver(type) {
        const driver = this.drivers.get(type);
        if (!driver || !isAgentSessionRuntimeDriver(driver)) return void 0;
        return driver;
    }
};
```

全应用只有三处注册，且是硬编码的：

```js
runtimeDriverRegistry.register(new LazyClaudeCodeRuntimeDriver());
runtimeDriverRegistry.register(new PiRuntimeDriver());
runtimeDriverRegistry.register(new DshRuntimeDriver());
```

**没有插件口，没有自定义运行时机制。** 我们搜索过 `custom-runtime`、`acp-agent`、
`external-runtime`、`pluginRuntime` 等可能的扩展缝，一个都没有。

**好消息**是「准入判断」是鸭子类型的：

```js
function isAgentSessionRuntimeDriver(driver) {
    return driver.capabilities.includes("agent-session") && "connect" in driver;
}
```

也就是说，一个对象只要有 `type`、`capabilities: ["agent-session"]` 和 `connect()`，
就能被注册表接受。**注册本身不是难点。**

---

## 二、难点在 `connect()` 返回的那个 connection

驱动接口表面看只有几个方法：

```js
class DshRuntimeDriver {
    type = "dsh";
    capabilities = ["agent-session"];
    async fork(input) {...}
    async validateSession(session) {...}
    async listAvailableTools(mcpIds) {...}
    async connect(input) { return new DshRuntimeConnection(...); }
    async reclaimOrphanSessions(keptResumeTokens, options) {...}
}
```

但 `connect()` 返回的 `DshRuntimeConnection` 才是真正的体量所在。实测它有 **22 个方法**：

```
start  startRuntime  send  reconcile  reconcileOnce  refreshTraceContext
getContextUsage  stopTask  flushForFork  waitForForkTransition  close
finishClose  disposeRuntime  effectivePermissionMode  handlePlanModeFold
buildPolicy  pumpNotifications  runSlashCommand  pushCommandOutput
handleTurnEnd  recordProviderInvocation  maybeEmitResumeToken
markTurnActive  releaseBackgroundWorkIfIdle  emitBridgeEvent  buildSubagentSink
```

它要往上层事件队列里推 **18 类事件**：

```
chunk  text  text-start  text-delta  text-end  thinking  usage  error
turn-complete  resume-token  api-retry  autonomous-turn-state
background-work-state  background-flow-chunk  background-task-event
background-tasks  data-agent-task-event
```

此外还挂了这些子系统：

- `DshStreamAdapter` —— 把后端原始流翻译成 Cherry 的 chunk 格式
- `subagents` —— 子 Agent 的运行状态跟踪
- `bridge` / `toolBridge` —— MCP 工具的桥接与审批往返
- `traceRecorder` —— OpenTelemetry 链路追踪
- 会话事件序号（`sessionEventSeqs`）、回合边界、上下文压缩、API 重试

换句话说，**这不是"对接一个 CLI"，而是要按 Cherry Studio 的会话模型重写一整套适配层。**

值得注意的是：Cherry Studio 做 dsh 时，是把**整个 harness 以 npm 包的形式打包进来**的
（`@cherrystudio/dsh-agent`、`dsh-agent-loop`、`dsh-llm-deepseek`、`dsh-compaction`……
共几十个包），而不是去对接一个外部 CLI。为 opencode 做同样的事，就是这个规模的工程量。

---

## 三、如果真要做，需要完成什么

1. **实现一个 driver + connection**，覆盖上面 22 个方法、18 类事件。
   没有类型定义、没有文档，只能从 dsh 的实现反推每个方法的契约。
2. **补丁主进程**：注入 driver 类、调用 `register()`、把 `type` 加进 `AGENT_RUNTIME_CAPABILITIES`
   （`out/main/rtk-*.js`）。
3. **补丁渲染进程**：`AGENT_RUNTIME_CAPABILITIES` 在渲染进程有**另一份拷贝**
   （`out/renderer/assets/EditDialogShared-*.js` 等），权限模式、工具列表、
   斜杠命令、图标都要同步，否则界面显示与运行时行为对不上。
4. **对接第三方 CLI 的协议**：以 opencode 为例，要处理它的会话格式、流式输出、
   工具调用与审批、上下文压缩、断点续传。这些 Cherry 都有既定期望的形态。
5. **持续跟随上游**：Cherry Studio 每次升级都会重打这两个 bundle，类名、字段名、
   事件类型都可能变。这等于长期维护一个 Cherry Studio 的私有分叉。

---

## 四、风险

| 风险 | 说明 |
|---|---|
| **升级即碎** | 补丁基于混淆后代码的字面匹配。任何一次升级都可能让匹配失败。 |
| **静默损坏** | connection 契约靠约定而非类型约束。实现错一个事件语义，可能表现为"能跑但状态错乱"，比直接报错更糟。 |
| **难以测试** | 没有官方测试接口，只能黑盒驱动界面来验证。 |
| **法律与观感** | 修改程序文件并分发补丁，比"改我自己数据库里的一个字段"要敏感得多。社区工具通常可以，但要说清楚。 |

---

## 五、建议的推进方式

如果确实想尝试注入，建议这样分期，每期都有明确的止损点：

**阶段 1 · 只读侦察（成本低）**
用本工具的 asar 读取器，把 `DshRuntimeConnection` 的完整源码导出并通读，
整理出一份"方法契约表"：每个方法何时被调用、必须返回什么、事件必须按什么顺序推。
**产出：一份接口规格文档。** 如果这一步就发现契约无法穷尽，就地止损。

**阶段 2 · 最小可用验证**
写一个只实现 `type` / `capabilities` / `connect` 的最小 driver，注册进去，
让它能开一个会话并推一条 `text-delta`。
**产出：一个能跑通"发消息 → 界面显示回复"的实验分支。**
这一步能真正回答"到底能不能做"。

**阶段 3 · 完整适配**
只有阶段 2 成功、且你愿意长期跟随上游版本，才进入这一步。

---

## 六、那本工具现在做什么

与其等待注入，更划算的是把**已经能做的事**做扎实：

- 自动发现**所有**被隐藏的运行时（现在已知 `dsh`，未来可能有别的）
- 两种后端：改数据库 / 打补丁
- 每次写操作前自动备份，一键撤销
- 一个能用的 Agent 管理器

这些不依赖任何未公开接口，也不怕升级——升级后重跑一次即可。
