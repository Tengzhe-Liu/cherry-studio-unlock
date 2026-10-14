# 第三方组件

本项目自身的代码以 [MIT](LICENSE) 授权。以下是它所包含或依赖的第三方组件。

## 直接包含在本仓库中的代码

### fractional-indexing

- 位置：`tools/orderkey-vectors/fractional-indexing.js`
- 用途：生成排序键测试向量的**参考实现**，只在开发/测试时使用，不进入发布产物
- 许可证：MIT
- 版权：Copyright (c) 2021 Rocicorp LLC
- 上游：https://github.com/rocicorp/fractional-indexing

该文件保留了原始的许可证声明。它取自 `fractional-indexing@3.2.0` 的上游源码
（Cherry Studio 也打包了同一个库），不是 Cherry Studio 自己的代码。

Go 侧的移植实现见 `internal/store/orderkey.go`，是对同一算法的重写。

## 通过 Go 模块依赖（见 `go.mod` / `go.sum`）

### modernc.org/sqlite

- 用途：纯 Go 的 SQLite 实现，让本项目无需 cgo 即可交叉编译
- 许可证：BSD-3-Clause
- 上游：https://gitlab.com/cznic/sqlite

其余间接依赖（`modernc.org/libc`、`golang.org/x/sys` 等）各自适用其原有许可证，
完整清单与版本以 `go.mod` / `go.sum` 为准。

## 本仓库不包含的内容

本项目**不包含** Cherry Studio 的任何代码、二进制或其打包产物（`app.asar`）。
工具在运行时读取用户本机已安装的 `app.asar`，但不会复制、分发或重新打包它。
`internal/store/testdata/orderkey_vectors.json` 是由上述 MIT 库生成的测试数据，
不含任何来自 Cherry Studio 的代码或用户数据。
