# AGENTS.md

## 项目背景

SyncDockerHub 是一个将公共容器镜像仓库（Docker Hub、registry.k8s.io、quay.io 等）的镜像自动同步到私有 Harbor 仓库的命令行工具。

核心使用场景：在内网或受限网络环境中，通过代理从公共仓库拉取镜像，推送到内部 Harbor 仓库供内网使用。支持增量同步（仅同步目标不存在的标签）、失败重试、多架构镜像复制、镜像清理（删除不匹配的镜像）。

## 技术栈

- **语言**: Go 1.25+
- **CLI 框架**: [cobra](https://github.com/spf13/cobra)
- **镜像操作**: [containers/image v5](https://github.com/containers/image) — skopeo 的底层库
  - `copy.Image()` — 镜像复制，无需安装 skopeo 二进制
  - `docker.GetRepositoryTags()` — 源标签列表获取，自动处理 OCI v2 Auth 认证流程
- **配置解析**: [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)
- **构建约束**: 必须使用 `-tags "containers_image_openpgp"` 编译，以使用纯 Go OpenPGP 实现替代 CGO 依赖的 GPGME

## 项目结构

```
├── main.go                              # 入口，调用 cmd.Execute()
├── cmd/
│   ├── root.go                          # 根命令，定义 -c/--config 全局参数
│   ├── sync.go                          # sync 子命令 - 执行镜像同步（三阶段卡片输出）
│   ├── check.go                         # check 子命令 - 预览匹配结果（dry run）
│   ├── list.go                          # list 子命令 - 列出目标仓库已有标签
│   └── delete.go                        # delete 子命令 - 删除不匹配镜像
├── internal/
│   ├── config/
│   │   ├── config.go                    # 配置结构体定义（Config, Rule, SyncConfig）
│   │   └── loader.go                    # 配置加载、校验、规则过滤、镜像引用解析
│   ├── registry/
│   │   ├── types.go                     # SourceClient 接口、SourceTag、Harbor 类型
│   │   ├── client.go                    # 源客户端工厂（NewSourceClient）
│   │   ├── containers_image.go          # 基于 containers/image 的统一源客户端
│   │   └── harbor.go                    # Harbor API v2 客户端（含认证）
│   ├── syncer/
│   │   ├── types.go                     # SyncStats, DeleteStats, CheckResult, DeleteResult, SyncResult 类型
│   │   ├── syncer.go                    # 核心引擎（准备/执行同步、检查、删除分析、标签解析）
│   │   └── copy.go                      # 镜像复制（containers/image 封装、重试、代理、schema1 适配）
│   └── logger/
│       ├── logger.go                    # 彩色日志工具、标签分组显示、颜色常量
│       └── message_card.go              # 卡片格式输出（PrintInfoCard）
├── config.yaml.example                  # 配置文件示例
├── Dockerfile                           # 多阶段 Docker 构建
└── go.mod / go.sum                      # Go 模块定义
```

## 架构与数据流

```
用户配置 (config.yaml)
    │
    ▼
config.Load() ── 校验 ── FilterRules() ── 按 name 过滤规则
    │
    ▼
cmd (sync/check/list/delete)
    │
    ├── registry.ContainersImageClient ── containers/image docker.GetRepositoryTags() ── 获取源标签列表
    │   └── 自动处理 OCI v2 Auth（Bearer token/Basic/匿名回退）
    │   └── 从 ~/.docker/config.json 读取认证信息
    │   └── 通过 SetProxy() / DockerProxyURL 控制代理（不依赖环境变量）
    │   └── GetManifestMediaType() ── 获取源镜像 manifest MIME 类型（用于 schema1 检测）
    │
    ├── registry.HarborClient ── Harbor API v2 ── 获取目标标签列表 / 删除镜像
    │   └── 从 ~/.docker/config.json 读取 Harbor 认证信息
    │
    ▼
syncer.Syncer
    │
    ├── PrepareSync()          ── 准备阶段（获取标签、解析待同步列表）
    │   ├── fetchSourceTags()  ── 获取源标签列表
    │   ├── resolveTags()      ── 对比源标签与目标标签，确定待同步列表
    │   │   ├── tags 模式      ── 精确匹配，跳过已存在
    │   │   └── tag_regex 模式 ── 正则过滤 + 已存在跳过
    │   └── 返回 SyncResult    ── 包含待同步/已存在标签信息和内部执行上下文
    │
    ├── ExecuteSync()          ── 执行阶段（逐个复制镜像）
    │   ├── 打印 Skip/Update 日志（完整源=>目标引用格式）
    │   └── copyImage()        ── 失败重试（可配置次数和间隔）
    │       └── doCopy()       ── 调用 containers/image copy.Image()
    │           ├── SourceCtx.DockerProxyURL（规则级代理，仅源端）
    │           ├── DestinationCtx 不设置代理（推送直连）
    │           ├── ImageListSelection: CopyAllImages（多架构）
    │           ├── schema1 镜像：PreserveDigests=false（允许 manifest 修改以更新嵌入引用）
    │           └── 非 schema1 镜像：PreserveDigests=true（保留摘要）
    │
    ├── AnalyzeDeleteRule()    ── 分析目标仓库中应删除的镜像
    │   ├── tags 模式          ── 保留列表中的标签，标记其余为 Unmatched
    │   └── tag_regex 模式     ── 保留匹配正则的标签，标记其余为 Unmatched
    │
    └── 返回统计结果           ── SyncStats / DeleteStats / CheckResult / DeleteResult
```

## 配置模型

### 全局配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `proxy` | string | 全局 HTTP 代理地址，兜底 `HTTPS_PROXY`/`HTTP_PROXY` 环境变量 |
| `no_proxy` | string | 代理排除列表（保留字段，当前实现中代理仅作用于源端，目标端直连无需排除） |
| `sync.retry` | int | 同步失败重试次数，默认 3 |
| `sync.interval` | duration | 重试间隔，默认 5s |

### 规则配置（Rule）

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 否 | 规则名称，用于 `--rule` 参数筛选，必须唯一 |
| `source` | 是 | 源镜像路径，如 `docker.io/library/alpine` |
| `destination` | 是 | 目标镜像路径，如 `hub.wiolfi.net/mirror/library/alpine` |
| `proxy` | 是 | 是否使用全局代理拉取镜像 |
| `tags` | 否* | 精确标签列表，与 `tag_regex` 二选一 |
| `tag_regex` | 否* | 正则表达式匹配标签，与 `tags` 二选一 |

### 镜像引用解析（ParseRef）

使用 `/` 分隔，`SplitN(ref, "/", 3)` 解析为 `Registry/Project/Name`：

| 输入 | Registry | Project | Name |
|------|----------|---------|------|
| `nginx` | | | nginx |
| `library/nginx` | | library | nginx |
| `docker.io/library/nginx` | docker.io | library | nginx |

### 代理机制

- 代理通过 `SystemContext.DockerProxyURL` 显式传递给 `containers/image` 库，不依赖环境变量
- `rule.Proxy = true` 时：将全局 `proxy` URL 设置到 `SourceCtx.DockerProxyURL`，源仓库请求走代理
- `rule.Proxy = false` 时：不设置 `DockerProxyURL`，源仓库请求直连
- `DestinationCtx` 始终不设置代理，推送到 Harbor 直连
- **不使用环境变量控制代理的原因**：Go 标准库 `http.ProxyFromEnvironment` 内部使用 `sync.Once` 缓存代理配置，仅在进程首次调用时读取环境变量，后续修改环境变量不会生效，导致多规则场景下代理状态不可切换

### 认证机制

- **源仓库认证**：`containers/image` 库自动从 `~/.docker/config.json` 读取认证信息，支持 Bearer token 自动获取/刷新、Basic auth、匿名回退
- **Harbor 认证**：`HarborClient` 从 `~/.docker/config.json` 读取对应 registry 的 Basic Auth 凭证，所有 API 请求（GET/DELETE）携带 `Authorization` 头

## CLI 命令

所有命令共享 `-c/--config` 全局参数（默认 `config.yaml`）。

### sync

执行镜像同步。对每条规则，从 destination 解析 Harbor 地址创建客户端，查询已有标签，仅同步不存在的标签。schema1 格式镜像通过禁用 `PreserveDigests` 适配复制。

输出采用三阶段卡片式展示（通过 `logger.PrintInfoCard` 实现）：
1. **基础信息卡片**：同步开始前打印，包含 Name、Source、Destination、Mode、Pattern
2. **任务统计卡片**：获取标签信息后打印，包含 Total tags、Synced tags、Existed tags
3. **同步结果卡片**：镜像复制完成后打印，包含 New、Updated、Failed

标签分组通过 `logger.PrintTagGroup` 输出，超过 30 个标签时自动截断。

```
image-syncer sync [-c config.yaml] [-r alpine,nginx]
```

### check

预览规则匹配结果（dry run）。通过 `logger.PrintInfoCard` 展示卡片格式的规则信息，通过 `logger.PrintTagGroup` 展示标签状态（待同步/已存在/需更新），标签超过 30 个时截断显示。

```
image-syncer check [-c config.yaml] [-r alpine]
```

### list

列出规则对应的目标仓库已有标签。从规则的 destination 中解析 Harbor 地址和项目。通过 `logger.PrintInfoCard` 输出规则信息卡片。

```
image-syncer list [-c config.yaml] [-r alpine]
```

### delete

删除目标仓库中不匹配规则的镜像。支持 dry-run 模式预览。通过 `logger.PrintInfoCard` 展示卡片格式的规则信息和删除计划。

```
image-syncer delete [-c config.yaml] [-r alpine] [--dry-run]
```

删除逻辑：
- **tags 模式**：保留列表中声明的标签，删除不在列表中的标签
- **tag_regex 模式**：保留匹配正则的标签，删除不匹配的标签
- **无 tags/tag_regex**：不删除任何镜像

## logger 包

`internal/logger` 提供统一的终端输出工具，所有子命令通过该包输出卡片和标签信息。

### 日志函数

| 函数 | 说明 |
|------|------|
| `Info(format, args...)` | 白色 [INFO] 日志 |
| `Done(format, args...)` | 绿色 [DONE] 日志 |
| `Warn(format, args...)` | 黄色 [WARN] 日志 |
| `Error(format, args...)` | 红色 [ERROR] 日志（输出到 stderr） |
| `Fatal(format, args...)` | 红色 [FATAL] 日志后退出进程 |
| `Debug(format, args...)` | 蓝色 [DEBUG] 日志 |

### 卡片输出

```go
logger.PrintInfoCard(title string, kvs map[string]string)
```

输出格式化的卡片信息，自动对齐键值对。各子命令直接调用此函数输出规则信息卡片：

```go
kvs := map[string]string{
    "Name":        rule.Name,
    "Source":      rule.Source,
    "Destination": rule.Dest,
    "Mode":        "tags (exact match)",
}
logger.PrintInfoCard(fmt.Sprintf("RULE %d/%d", idx, total), kvs)
```

### 标签分组显示

```go
logger.PrintTagGroup(label string, tags []string)
```

格式化输出标签组，超过 30 个标签时自动截断显示。标签不带颜色参数，由调用方在 label 中嵌入颜色。

```go
logger.PrintTagGroup(logger.ColorGreen+"[+] Synced"+logger.ColorReset, result.ToSync)
```

### 标签列表格式化

```go
logger.FormatTagList(tags []string) string
```

将标签列表格式化为逗号分隔字符串，超过 10 个标签时截断并显示剩余数量。

### 颜色常量

| 常量 | ANSI 代码 | 用途 |
|------|-----------|------|
| `ColorReset` | `\033[0m` | 重置颜色 |
| `ColorRed` | `\033[0;31m` | 错误/删除 |
| `ColorGreen` | `\033[0;32m` | 成功/新增 |
| `ColorYellow` | `\033[0;33m` | 警告/已存在 |
| `ColorBlue` | `\033[0;34m` | 调试信息 |
| `ColorCyan` | `\033[0;36m` | 信息/列表 |
| `ColorMagenta` | `\033[0;35m` | 更新 |
| `ColorBold` | `\033[1m` | 强调 |
| `ColorDim` | `\033[2m` | 暗淡/次要 |

## 关键实现细节

### Schema1 镜像处理策略

schema1 镜像的 manifest 中嵌入了源仓库的 Docker 引用，复制到目标仓库时引用会改变，导致摘要变化。`containers/image` 库在 `PreserveDigests: true` 时会拒绝此操作。

**处理方式**：
- **sync 命令**：不屏蔽 schema1 镜像，而是在复制前通过 `GetManifestMediaType()` 检测 manifest 类型，如果是 schema1 则设置 `PreserveDigests: false`，允许 manifest 被修改以更新嵌入的 Docker 引用
- **delete 命令**：不删除 schema1 镜像，schema1 镜像与普通镜像一样参与 tags/tag_regex 匹配

**检测方法**：
```go
func isSchema1MediaType(mediaType string) bool {
    return mediaType == "application/vnd.docker.distribution.manifest.v1+json" ||
        mediaType == "application/vnd.docker.distribution.manifest.v1+prettyjws"
}
```

### 同步流程（PrepareSync + ExecuteSync）

sync 命令将同步流程拆分为准备和执行两个阶段，以支持三阶段卡片式输出：

1. **PrepareSync()**：获取源/目标标签列表，解析待同步标签，返回 `SyncResult`（包含待同步/已存在标签信息和内部执行上下文 `tagsToSync`/`destPr`/`rule`）
2. **ExecuteSync()**：遍历 `SyncResult.tagsToSync` 逐个复制镜像，打印 Skip/Update/Sync 日志，更新统计信息

### 源标签获取（ContainersImageClient）

使用 `containers/image` 库的 `docker.GetRepositoryTags()` 统一获取所有源仓库的标签列表：

```go
sysCtx := c.buildSysCtx()  // 根据 proxyURL 设置 DockerProxyURL
ref, _ := alltransports.ParseImageName("docker://" + repository)
tagNames, _ := docker.GetRepositoryTags(ctx, sysCtx, ref)
```

- 自动处理 OCI v2 Auth 认证流程（Bearer token 获取/刷新、Basic auth、匿名回退）
- 从 `~/.docker/config.json` 读取认证信息
- 支持所有 OCI 兼容仓库（Docker Hub、registry.k8s.io、quay.io、GHCR、阿里云 ACR 等）
- 内置 `sync.Mutex` + `map` 内存缓存
- 代理通过 `SetProxy()` 方法设置，内部解析为 `*url.URL` 并写入 `SystemContext.DockerProxyURL`

### Manifest 类型获取（GetManifestMediaType）

```go
func (c *ContainersImageClient) GetManifestMediaType(repository, tag string) (string, error) {
    refStr := "docker://" + repository + ":" + tag
    ref, _ := alltransports.ParseImageName(refStr)
    sysCtx := c.buildSysCtx()
    src, _ := ref.NewImageSource(context.Background(), sysCtx)
    defer src.Close()
    _, mimeType, _ := src.GetManifest(context.Background(), nil)
    return mimeType, nil
}
```

- 打开镜像源引用，获取 manifest 的 MIME 类型
- 用于 `doCopy()` 中在复制前检测 schema1 镜像

### 镜像复制（doCopy）

```go
sourceCtx := &types.SystemContext{}
destCtx := &types.SystemContext{}

if rule.Proxy && s.proxyURL != "" {
    proxyURL, _ := url.Parse(s.proxyURL)
    sourceCtx.DockerProxyURL = proxyURL  // 仅源端走代理
}

preserveDigests := true
if idx := strings.LastIndex(srcRef, ":"); idx > 0 {
    mediaType, _ := s.sourceClient.GetManifestMediaType(srcRef[:idx], srcRef[idx+1:])
    if isSchema1MediaType(mediaType) {
        preserveDigests = false  // schema1 镜像允许 manifest 修改
    }
}

options := &copy.Options{
    SourceCtx:          sourceCtx,
    DestinationCtx:     destCtx,
    ImageListSelection: copy.CopyAllImages,
    PreserveDigests:    preserveDigests,
}
_, err = copy.Image(ctx, policyCtx, dstRef, srcRef, options)
```

- 签名策略使用 `InsecureAcceptAnything`（接受所有签名）
- `PolicyContext` 在 `NewSyncer` 时创建，`Close()` 时销毁
- 代理通过 `SourceCtx.DockerProxyURL` 显式传递，仅源端走代理，目标端直连
- schema1 镜像设置 `PreserveDigests: false`，允许 manifest 中嵌入的 Docker 引用被更新
- 非 schema1 镜像设置 `PreserveDigests: true`，保留摘要不变
- 所有复制错误统一走重试逻辑，不针对 schema1 特殊处理

### Harbor API 客户端

- 调用 Harbor v2 API：`/api/v2.0/projects/{project}/repositories/{name}/artifacts`
- 仓库名中的 `/` 使用 `%252F` 双重 URL 编码（Harbor 特殊要求）
- 每条规则独立创建 HarborClient（因为不同规则可能指向不同 Harbor 实例）
- 从 `~/.docker/config.json` 自动读取认证信息，所有请求携带 `Authorization` 头
- 支持 `ListArtifacts()`（含 media_type）、`DeleteArtifact()`、`ListRepositories()` 等方法

### 标签过滤逻辑（resolveTags）

1. **tags 模式**：遍历指定标签列表，跳过目标已存在的标签
2. **tag_regex 模式**：
   - 从源仓库获取全部标签
   - 正则匹配过滤
   - 跳过目标已存在的标签

`resolveTags()` 不直接打印日志，而是将信息存入 `tagAction` 结构体（含 `SrcRef`/`DstRef` 字段），由 `ExecuteSync()` 统一打印。

### 删除分析逻辑（AnalyzeDeleteRule）

1. 获取目标仓库所有 artifacts（含 media_type）
2. tags 模式：不在列表中的标签标记为 Unmatched
3. tag_regex 模式：不匹配正则的标签标记为 Unmatched
4. 返回 `DeleteResult`（Unmatched 列表、Kept 列表）

### 配置校验（Validate）

- `source` 和 `destination` 必填
- `name` 如果指定则必须唯一
- `tag_regex` 必须是合法正则表达式
- `tags` 和 `tag_regex` 不可同时使用

### 规则过滤（FilterRules）

- 传入逗号分隔的名称字符串
- 按 `rule.Name` 匹配
- 不存在的名称报错
- 传入空字符串返回全部规则

## 构建与部署

```bash
# 本地构建（必须使用 containers_image_openpgp tag）
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o bin/image-syncer .

# 直接运行（必须使用 containers_image_openpgp tag）
CGO_ENABLED=0 go run -tags "containers_image_openpgp" .

# 静态分析
CGO_ENABLED=0 go vet -tags "containers_image_openpgp" .

# Docker 构建
docker build -t image-syncer:latest .

# 运行
docker run --rm \
  -v /path/to/config.yaml:/etc/image-syncer/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  image-syncer:latest
```

Docker 运行时需要挂载 `~/.docker/config.json` 以提供源仓库和 Harbor 的认证信息。
