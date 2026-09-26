# 版本

[English](versioning.en.md)

dbx 有两个版本号，各自变更的原因不同。

| | 位置 | 含义 |
|---|---|---|
| 模块版本 | Git 标签 `vX.Y.Z` | `go get github.com/fluxsce/dbx@vX.Y.Z` 取到的版本 |
| 语言版本 | `go.mod` 中的 `go 1.25.0` | 能编译本模块的最低 Go 工具链 |

源码里没有版本常量。`go` 按标签解析。

## 模块版本

| 升位 | 时机 |
|---|---|
| `1.0.x` | 修正绑定、扫描或引擎 SQL。函数签名和[约定](compatibility.zh-CN.md)不变。生成 SQL 的文本和分批大小可以变。 |
| `1.x.0` | 新增方法、可选方言接口或驱动。已有调用点仍可编译。 |
| `2.0.0` | 打破 1.x 约定。导入路径改为 `github.com/fluxsce/dbx/v2`。 |

安装最新标签：

```bash
go get github.com/fluxsce/dbx@latest
```

构建不能跟着最新标签走时，钉住一个标签：

```bash
go get github.com/fluxsce/dbx@v1.0.1
```

## 语言版本

`go 1.25.0` 表示使用方用 Go 1.25 或更新的工具链编译 dbx。升高这一行会丢掉所有更旧的工具链。只有源码用到了旧工具链没有的语言或标准库能力时才升高。模块标签可以在不改这一行的情况下发布。

## 如何发布

版本写在 [CHANGELOG.md](../CHANGELOG.md) 的标题里，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)。

推送到 `main` 并通过测试后，`.github/workflows/release.yml` 做这些事：

1. 读取第一个匹配 `## [x.y.z]` 的标题。`## [Unreleased]` 不匹配。
2. `vX.Y.Z` 已存在则停止。
3. 否则把该提交打成标签 `vX.Y.Z`，推送标签，并用该标题下的正文创建 GitHub Release。

发布 `v1.0.2` 时，把 `## [1.0.2] - YYYY-MM-DD` 插在 `## [Unreleased]` 正下方、所有旧版本上方，写好变更，再推送到 `main`。新标题若追加在当前版本下面，工作流仍读取第一个数字标题，因此不会发布。

标签指向引入该标题的提交。`go get` 从公开仓库拉取该标签。模块代理在第一次请求时缓存。
