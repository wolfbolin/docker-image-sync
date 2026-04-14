# AGENTS.md

## 项目背景

SyncDockerHub 是一个将公共容器镜像仓库（Docker Hub、registry.k8s.io、quay.io 等）的镜像自动同步到私有 Harbor 仓库的命令行工具。

核心使用场景：在内网或受限网络环境中，通过代理从公共仓库拉取镜像，推送到内部 Harbor 仓库供内网使用。支持增量同步（仅同步目标不存在的标签）、失败重试、多架构镜像复制。

## 技术栈

- **语言**: Go 1.25+
- **CLI 框架**: [cobra](https://github.com/spf13/cobra)
- **镜像复制**: [containers/image v5](https://github.com/containers/image) — skopeo 的底层库，直接调用 `copy.Image()` API，无需安装 skopeo 二进制
- **配置解析**: [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)
- **构建约束**: 必须使用 `-tags "containers_image_openpgp"` 编译，以使用纯 Go OpenPGP 实现替代 CGO 依赖的 GPGME

## 项目结构

```
├── main.go                              # 入口，调用 cmd.Execute()
├── cmd/
│   ├── root.go                          # 根命令，定义 -c/--config 全局参数
│   ├── sync.go                          # sync 子命令 - 执行镜像同步
│   ├── check.go                         # check 子命令 - 预览匹配结果（dry run）
│   └── list.go                          # list 子命令 - 列出目标仓库已有标签
├── internal/
│   ├── config/
│   │   ├── config.go                    # 配置结构体定义（Config, Rule, SyncConfig）
│   │   └── loader.go                    # 配置加载、校验、规则过滤、镜像引用解析
│   ├── registry/
│   │   ├── types.go                     # API 响应类型（DockerHubTag, HarborArtifact, HarborTagInfo 等）
│   │   ├── dockerhub.go                 # Docker Hub API v2 客户端（带内存缓存）
│   │   └── harbor.go                    # Harbor API v2 客户端
│   ├── syncer/
│   │   ├── types.go                     # SyncStats, CheckResult 类型定义
│   │   ├── syncer.go                    # 核心同步引擎（标签解析、digest 比较）
│   │   └── copy.go                      # 镜像复制（containers/image 封装、重试、代理）
│   └── logger/
│       └── logger.go                    # 彩色日志工具（INFO/WARN/ERROR/FATAL/DEBUG）
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
cmd (sync/check/list)
    │
    ├── registry.DockerHubClient ── Docker Hub API ── 获取源镜像标签列表
    ├── registry.HarborClient    ── Harbor API v2   ── 获取目标已有标签列表
    │
    ▼
syncer.Syncer
    │
    ├── resolveTags()     ── 对比源标签与目标标签，确定待同步列表
    │   ├── tags 模式     ── 精确匹配，跳过已存在
    │   └── tag_regex 模式 ── 正则过滤 + V1 manifest 跳过 + 已存在跳过
    │
    ├── copyImage()       ── 失败重试（可配置次数和间隔）
    │   └── doCopy()      ── 调用 containers/image copy.Image()
    │       ├── 设置 HTTPS_PROXY / HTTP_PROXY（规则级代理开关）
    │       ├── 设置 NO_PROXY（代理排除列表）
    │       ├── ImageListSelection: CopyAllImages（多架构）
    │       └── PreserveDigests: true（保留摘要）
    │
    └── 返回 SyncStats    ── Success / Failed / Exist / Skipped
```

## 配置模型

### 全局配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `proxy` | string | 全局 HTTP 代理地址，兜底 `HTTPS_PROXY`/`HTTP_PROXY` 环境变量 |
| `no_proxy` | string | 代理排除列表，兜底 `NO_PROXY`/`no_proxy` 环境变量 |
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

- `rule.Proxy = true` 时：设置 `HTTPS_PROXY`/`HTTP_PROXY` 环境变量 + `NO_PROXY` 排除列表
- `rule.Proxy = false` 时：清除所有代理相关环境变量
- `containers/image` 库通过 `http.ProxyFromEnvironment` 读取环境变量，`NO_PROXY` 中的地址不走代理
- 典型用法：`no_proxy` 中配置目标 Harbor 地址，实现"拉取走代理、推送不走代理"

## CLI 命令

所有命令共享 `-c/--config` 全局参数（默认 `config.yaml`）。

### sync

执行镜像同步。对每条规则，从 destination 解析 Harbor 地址创建客户端，查询已有标签，仅同步不存在的标签。

```
sync-docker sync [-c config.yaml] [-r alpine,nginx]
```

### check

预览规则匹配结果（dry run）。展示 Box 格式的规则信息和标签状态（待同步/已存在/跳过 V1），标签超过 30 个时截断显示。

```
sync-docker check [-c config.yaml] [-r alpine]
```

### list

列出规则对应的目标仓库已有标签。从规则的 destination 中解析 Harbor 地址和项目。

```
sync-docker list [-c config.yaml] [-r alpine]
```

## 关键实现细节

### 镜像复制（doCopy）

```go
// 等价于: skopeo copy --all --preserve-digests docker://src docker://dst
options := &copy.Options{
    SourceCtx:          &types.SystemContext{},
    DestinationCtx:     &types.SystemContext{},
    ImageListSelection: copy.CopyAllImages,
    PreserveDigests:    true,
}
_, err = copy.Image(ctx, policyCtx, dstRef, srcRef, options)
```

- 签名策略使用 `InsecureAcceptAnything`（接受所有签名）
- `PolicyContext` 在 `NewSyncer` 时创建，`Close()` 时销毁
- 代理通过环境变量控制，每次 `doCopy` 调用前根据 `rule.Proxy` 设置/清除

### Docker Hub API 客户端

- 调用 `https://hub.docker.com/v2/namespaces/{project}/repositories/{name}/tags`
- 分页获取（每页 100 条）
- 内置 `sync.Mutex` + `map` 内存缓存，同一镜像不重复请求

### Harbor API 客户端

- 调用 Harbor v2 API：`/api/v2.0/projects/{project}/repositories/{name}/artifacts`
- 仓库名中的 `/` 使用 `%252F` 双重 URL 编码（Harbor 特殊要求）
- 每条规则独立创建 HarborClient（因为不同规则可能指向不同 Harbor 实例）

### 标签过滤逻辑（resolveTags）

1. **tags 模式**：遍历指定标签列表，跳过目标已存在的标签
2. **tag_regex 模式**：
   - 从 Docker Hub API 获取源镜像全部标签
   - 正则匹配过滤
   - 跳过 V1 manifest（`application/vnd.docker.distribution.manifest.v1+json` 和 `v1+prettyjws`）
   - 跳过目标已存在的标签

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
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o sync-docker .

# Docker 构建
docker build -t sync-docker:latest .

# 运行
docker run --rm \
  -v /path/to/config.yaml:/etc/sync-docker/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  sync-docker:latest
```

Docker 运行时需要挂载 `~/.docker/config.json` 以提供 Harbor 写入认证。
